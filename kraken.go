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
	Client *KrakenHTTPClient
}

func NewKrakenProvider(cfg *KrakenProviderConfig) *KrakenProvider {
	p := &KrakenProvider{
		Logger: cfg.Logger.With("name", "kraken.provider"),
		Client: NewKrakenHTTPClient(cfg.APIKey, cfg.APISecret),
	}
	p.Client.Logger = cfg.Logger.With("name", "kraken.client")
	return p
}

func (p KrakenProvider) PlaceOrder(ctx context.Context, order PlaceOrderRequest) (res PlaceOrderResponse, err error) {
	defer WrapErr(&err, "KrakenProvider.PlaceOrder")

	var volume float64
	if volume, err = p.Client.FetchBuyVolume(ctx, order.AmountInCents); err != nil {
		return res, err
	}

	p.Logger.InfoContext(ctx, fmt.Sprintf("fetched buy volume: %v", volume))

	res.AmountInCents = order.AmountInCents
	res.Volume = volume
	if res.TransactionID, res.AdditionalInfo, err = p.Client.PlaceOrder(ctx, volume); err != nil {
		return res, err
	}

	return res, nil
}

const btcUSDPair = "XBTUSD"

type KrakenHTTPClient struct {
	APIKey        string
	APISecretKey  string
	Logger        *slog.Logger
	GenerateNonce func() int64

	http *http.Client
}

func NewKrakenHTTPClient(apiKey string, apiSecretKey string) *KrakenHTTPClient {
	return &KrakenHTTPClient{
		APIKey:        apiKey,
		APISecretKey:  apiSecretKey,
		Logger:        slog.New(slog.DiscardHandler),
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

// FetchBuyVolume finds the amount of BTC to buy in USD
func (c *KrakenHTTPClient) FetchBuyVolume(ctx context.Context, amountInCents int) (volume float64, err error) {
	defer WrapErr(&err, "KrakenHTTPClient.FetchBuyVolume")

	c.Logger.InfoContext(ctx, "fetching buy volume")

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.kraken.com/0/public/Ticker?pair="+btcUSDPair, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request to fetch buy volume: %w", err)
	}
	req.Header.Add("Accept", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch buy volume: %w", err)
	}
	defer func() {
		if cerr := res.Body.Close(); cerr != nil {
			c.Logger.WarnContext(ctx, "failed to close response body", "err", cerr)
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

// PlaceOrder places a market order for volume BTC.
func (c *KrakenHTTPClient) PlaceOrder(ctx context.Context, volume float64) (transactionID string, orderDescription string, err error) {
	defer WrapErr(&err, "KrakenHTTPClient.PlaceOrder")

	c.Logger.InfoContext(ctx, "placing buy order", "volume", volume)

	nonce := c.GenerateNonce()
	params := url.Values{}
	params.Set("pair", btcUSDPair)
	params.Set("type", "buy")
	params.Set("volume", strconv.FormatFloat(volume, 'f', -1, 64))
	params.Set("ordertype", "market")
	params.Set("nonce", strconv.FormatInt(nonce, 10))

	c.Logger.InfoContext(ctx, "creating HTTP request", "body", params)

	var req *http.Request
	if req, err = http.NewRequest("POST", "https://api.kraken.com/0/private/AddOrder", bytes.NewBufferString(params.Encode())); err != nil {
		return "", "", fmt.Errorf("failed to make request: %w", err)
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("API-Key", c.APIKey)
	req.Header.Add("API-Sign", c.generateSignature("/0/private/AddOrder", params, nonce))

	res, err := c.http.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to do request: %w", err)
	}

	defer func() {
		if cerr := res.Body.Close(); cerr != nil {
			c.Logger.WarnContext(ctx, "failed to close response body", "err", cerr)
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
		return "", "", fmt.Errorf("failed to place order: %v", c.toError(response.Error[0]))
	}

	c.Logger.InfoContext(ctx, "response from buy order placement", "response", response)
	return response.Result.TransactionID[0], response.Result.Description.Order, nil
}

func (c *KrakenHTTPClient) generateSignature(path string, data url.Values, nonce int64) string {
	sha := sha256.New()
	sha.Write([]byte(strconv.FormatInt(nonce, 10) + data.Encode()))
	hash := sha.Sum(nil)

	mac := hmac.New(sha512.New, []byte(c.APISecretKey))
	mac.Write(append([]byte(path), hash...))

	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (c *KrakenHTTPClient) toError(message string) error {
	if message == "EGeneral:Invalid arguments:volume minimum not met" {
		return ErrOrderToSmall
	} else if message == "EAPI:Invalid key" {
		return ErrInvalidAuth
	}

	return errors.New(message)
}
