package dca

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type KrakenProviderConfig struct {
	APIKey    string
	APISecret string
	Logger    *slog.Logger
}

type KrakenProvider struct {
	Logger *slog.Logger

	APIKey        string
	APISecretKey  string
	GenerateNonce func() int64

	http *http.Client
}

func NewKrakenProvider(cfg *KrakenProviderConfig) *KrakenProvider {
	return &KrakenProvider{
		Logger:        cfg.Logger.With("name", "kraken.provider"),
		APIKey:        cfg.APIKey,
		APISecretKey:  cfg.APISecret,
		GenerateNonce: time.Now().UnixNano,
		http: &http.Client{
			Timeout: time.Second * 10,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: time.Second * 5,
				}).DialContext,
				TLSHandshakeTimeout: time.Second * 5,
			},
		},
	}
}

type PlaceOrderRequest struct {
	AmountInCents int `json:"amountInCents"`
}

type PlaceOrderResponse struct {
	AmountInCents  int     `json:"amountInCents"`
	Volume         float64 `json:"volume"`
	TransactionID  string  `json:"transactionId"`
	AdditionalInfo string  `json:"additionalInfo"`
}

func (p *KrakenProvider) PlaceOrder(ctx context.Context, order PlaceOrderRequest) (res PlaceOrderResponse, err error) {
	defer WrapErr(&err, "KrakenProvider.PlaceOrder")

	var volume float64
	if volume, err = p.fetchBuyVolume(ctx, order.AmountInCents); err != nil {
		return res, err
	}

	p.Logger.InfoContext(ctx, fmt.Sprintf("fetched buy volume: %0.8f", volume))

	res.AmountInCents = order.AmountInCents
	res.Volume = volume
	if res.TransactionID, res.AdditionalInfo, err = p.placeOrder(ctx, volume); err != nil {
		return res, err
	}

	return res, nil
}

const btcUSDPair = "XBTUSD"

// FetchBuyVolume finds the amount of BTC to buy in USD
func (p *KrakenProvider) fetchBuyVolume(ctx context.Context, amountInCents int) (volume float64, err error) {
	defer WrapErr(&err, "fetchBuyVolume")

	p.Logger.InfoContext(ctx, "fetching buy volume")

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.kraken.com/0/public/Ticker?pair="+btcUSDPair, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request to fetch buy volume: %w", err)
	}
	req.Header.Add("Accept", "application/json")

	res, err := p.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch buy volume: %w", err)
	}
	defer func() {
		if cerr := res.Body.Close(); cerr != nil {
			p.Logger.WarnContext(ctx, "failed to close response body", "err", cerr)
		}
	}()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response body: %w", err)
	}

	var response = struct {
		Error  []any `json:"error"`
		Result struct {
			Xxbtzusd struct {
				// ASK is the lowest price that a seller will accept
				A []string `json:"a"`
				B []string `json:"b"`
				C []string `json:"c"`
				V []string `json:"v"`
				P []string `json:"p"`
				T []int    `json:"t"`
				L []string `json:"l"`
				H []string `json:"h"`
				O string   `json:"o"`
			} `json:"XXBTZUSD"`
		} `json:"result"`
	}{}

	if err = json.Unmarshal(body, &response); err != nil {
		return 0, fmt.Errorf("failed to unmarshal response body: %w", err)
	} else if response.Error != nil && len(response.Error) > 0 {
		return 0, fmt.Errorf("failed to fetch buy volume: %w", errors.New(response.Error[0].(string)))
	}

	// base/quote - quote is the amount of USD needed to buy the base
	var quote float64
	if quote, err = strconv.ParseFloat(response.Result.Xxbtzusd.A[0], 64); err != nil {
		return 0, fmt.Errorf("failed to parse quote: %w", err)
	}

	base := 1.0
	// convert exchange rate to cents
	dollarExchangeRate := base / quote
	centsExchangeRate := dollarExchangeRate / 100
	return centsExchangeRate * float64(amountInCents), nil
}

// placeOrder places a market order for volume BTC
func (p *KrakenProvider) placeOrder(ctx context.Context, volume float64) (transactionID string, orderDescription string, err error) {
	defer WrapErr(&err, "placeOrder")

	p.Logger.InfoContext(ctx, "placing buy order", "volume", volume)

	nonce := p.GenerateNonce()
	params := url.Values{}
	params.Set("pair", btcUSDPair)
	params.Set("type", "buy")
	params.Set("volume", strconv.FormatFloat(volume, 'f', -1, 64))
	params.Set("ordertype", "market")
	params.Set("nonce", strconv.FormatInt(nonce, 10))

	p.Logger.InfoContext(ctx, "creating HTTP request", "body", params)

	var req *http.Request
	if req, err = http.NewRequest("POST", "https://api.kraken.com/0/private/AddOrder", bytes.NewBufferString(params.Encode())); err != nil {
		return "", "", fmt.Errorf("failed to make request: %w", err)
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("API-Key", p.APIKey)
	req.Header.Add("API-Sign", p.generateSignature("/0/private/AddOrder", params, nonce))

	res, err := p.http.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to do request: %w", err)
	}

	defer func() {
		if cerr := res.Body.Close(); cerr != nil {
			p.Logger.WarnContext(ctx, "failed to close response body", "err", cerr)
		}
	}()

	var body []byte

	if body, err = io.ReadAll(res.Body); err != nil {
		return "", "", fmt.Errorf("failed to read response body: %w", err)
	}

	var response = struct {
		Error  []string `json:"error"`
		Result struct {
			TransactionID []string `json:"txid"`
			Description   struct {
				Order string `json:"order"`
			} `json:"descr"`
		} `json:"result,omitempty"`
	}{}

	if err = json.Unmarshal(body, &response); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	if response.Error != nil && len(response.Error) > 0 {
		return "", "", fmt.Errorf("failed to place order: %v", p.toError(response.Error[0]))
	}

	p.Logger.InfoContext(ctx, "response from buy order placement", "response", response)
	return response.Result.TransactionID[0], response.Result.Description.Order, nil
}

func (p *KrakenProvider) generateSignature(path string, data url.Values, nonce int64) string {
	sha := sha256.New()
	sha.Write([]byte(strconv.FormatInt(nonce, 10) + data.Encode()))
	hash := sha.Sum(nil)

	mac := hmac.New(sha512.New, []byte(p.APISecretKey))
	mac.Write(append([]byte(path), hash...))

	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (p *KrakenProvider) toError(message string) error {
	if message == "EGeneral:Invalid arguments:volume minimum not met" {
		return ErrOrderToSmall
	} else if message == "EAPI:Invalid key" {
		return ErrInvalidAuth
	}

	return errors.New(message)
}
