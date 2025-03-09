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

type ExecuteOrderRequest struct {
	AmountInCents int `json:"amountInCents"`
}

type ExecuteOrderResponse struct {
	AmountInCents   int     `json:"amountInCents"`
	TransactionID   string  `json:"transactionId"`
	AdditionalInfo  string  `json:"additionalInfo"`
	RequestedVolume float64 `json:"volumeRequested"`
	VolumePurchased float64 `json:"volumePurchased"`
	Cost            float64 `json:"cost"`
	Fee             float64 `json:"fee"`
	Price           float64 `json:"price"`
}

func (p *KrakenProvider) ExecuteOrder(ctx context.Context, order ExecuteOrderRequest) (res ExecuteOrderResponse, err error) {
	defer WrapErr(&err, "KrakenProvider.ExecuteOrder")

	var volume float64
	if volume, err = p.fetchBuyVolume(ctx, order.AmountInCents); err != nil {
		return res, err
	}

	p.Logger.InfoContext(ctx, fmt.Sprintf("fetched buy volume: %0.8f", volume))

	res.AmountInCents = order.AmountInCents
	res.RequestedVolume = volume
	if res.TransactionID, res.AdditionalInfo, err = p.placeOrder(ctx, volume); err != nil {
		return res, err
	}

	var oi orderInfo
	if oi, err = p.queryOrderInfo(ctx, res.TransactionID); err != nil {
		return res, err
	}
	res.Price = oi.Price
	res.Cost = oi.Cost
	res.Fee = oi.Fee
	res.VolumePurchased = oi.VolumePurchased

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

type orderInfo struct {
	VolumePurchased float64 `json:"volumePurchased"`
	Cost            float64 `json:"cost"`
	Fee             float64 `json:"fee"`
	Price           float64 `json:"price"`
}

func (p *KrakenProvider) queryOrderInfo(ctx context.Context, transactionID string) (oi orderInfo, err error) {
	defer WrapErr(&err, "queryOrderInfo")

	p.Logger.InfoContext(ctx, "querying order info")

	// add +1 to ensure the nonce generated is higher than the previous generated nonce.
	nonce := p.GenerateNonce() + 1

	params := url.Values{}
	params.Set("txid", transactionID)
	params.Set("nonce", strconv.FormatInt(nonce, 10))
	params.Set("trades", "true")

	p.Logger.InfoContext(ctx, "creating HTTP request", "body", params)

	var req *http.Request
	if req, err = http.NewRequest("POST", "https://api.kraken.com/0/private/QueryOrders", bytes.NewBufferString(params.Encode())); err != nil {
		return oi, fmt.Errorf("failed to make request: %w", err)
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("API-Key", p.APIKey)
	req.Header.Add("API-Sign", p.generateSignature("/0/private/QueryOrders", params, nonce))

	res, err := p.http.Do(req)
	if err != nil {
		return oi, fmt.Errorf("failed to do request: %w", err)
	}

	defer func() {
		if cerr := res.Body.Close(); cerr != nil {
			p.Logger.WarnContext(ctx, "failed to close response body", "err", cerr)
		}
	}()

	var body []byte

	if body, err = io.ReadAll(res.Body); err != nil {
		return oi, fmt.Errorf("failed to read response body: %w", err)
	}

	var response = struct {
		Error  []string `json:"error"`
		Result map[string]struct {
			Refid    string  `json:"refid"`
			Userref  int     `json:"userref"`
			Status   string  `json:"status"`
			Reason   any     `json:"reason"`
			Opentm   float64 `json:"opentm"`
			Closetm  float64 `json:"closetm"`
			Starttm  int     `json:"starttm"`
			Expiretm int     `json:"expiretm"`
			Descr    struct {
				Pair      string `json:"pair"`
				Type      string `json:"type"`
				Ordertype string `json:"ordertype"`
				Price     string `json:"price"`
				Price2    string `json:"price2"`
				Leverage  string `json:"leverage"`
				Order     string `json:"order"`
				Close     string `json:"close"`
			} `json:"descr"`
			Vol        string   `json:"vol"`
			VolExec    string   `json:"vol_exec"`
			Cost       string   `json:"cost"`
			Fee        string   `json:"fee"`
			Price      string   `json:"price"`
			Stopprice  string   `json:"stopprice"`
			Limitprice string   `json:"limitprice"`
			Misc       string   `json:"misc"`
			Oflags     string   `json:"oflags"`
			Trades     []string `json:"trades"`
		} `json:"result"`
	}{}

	if err = json.Unmarshal(body, &response); err != nil {
		return oi, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	if response.Error != nil && len(response.Error) > 0 {
		return oi, fmt.Errorf("failed to query order info: %v", p.toError(response.Error[0]))
	}

	if oi.Fee, err = strconv.ParseFloat(response.Result[transactionID].Fee, 64); err != nil {
		return oi, fmt.Errorf("failed to parse fee: %w", err)
	}
	if oi.Cost, err = strconv.ParseFloat(response.Result[transactionID].Cost, 64); err != nil {
		return oi, fmt.Errorf("failed to parse cost: %w", err)
	}
	if oi.Price, err = strconv.ParseFloat(response.Result[transactionID].Price, 64); err != nil {
		return oi, fmt.Errorf("failed to parse price: %w", err)
	}
	if oi.VolumePurchased, err = strconv.ParseFloat(response.Result[transactionID].Vol, 64); err != nil {
		return oi, fmt.Errorf("failed to parse volume: %w", err)
	}

	p.Logger.InfoContext(ctx, "response from query order info", "response", oi)
	return oi, nil
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
