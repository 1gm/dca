package internal

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/1gm/dca"
)

type Config struct {
	KrakenAPIKey       string `json:"krakenApiKey"`
	KrakenPrivateKey   string `json:"krakenPrivateKey"`
	OrderAmountInCents int    `json:"orderAmountInCents"`
}

type Main struct {
	Config Config
	Logger *slog.Logger
}

func NewMain() *Main {
	return &Main{
		Logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

func (m *Main) Run(ctx context.Context) (err error) {
	m.Logger.InfoContext(ctx, "starting process", "version", dca.Version, "commit", dca.Commit, "date", dca.Date)

	provider := dca.NewKrakenProvider(&dca.KrakenProviderConfig{
		APIKey:    m.Config.KrakenAPIKey,
		APISecret: m.Config.KrakenPrivateKey,
		Logger:    m.Logger,
	})

	order := dca.PlaceOrderRequest{AmountInCents: m.Config.OrderAmountInCents}
	if res, err := provider.PlaceOrder(ctx, order); err != nil {
		return err
	} else {
		m.Logger.Info("order successfully placed", "result", res)
	}

	return nil
}

// ParseFlagsAndLoadConfig parses the application config file from the --config flag and loads it.
func (m *Main) ParseFlagsAndLoadConfig(ctx context.Context, args []string) error {
	var configFile string

	fs := flag.NewFlagSet("dca", flag.ContinueOnError)
	fs.StringVar(&configFile, "config", os.Getenv("CONFIG_FILE"), "path to the config file")

	if err := fs.Parse(args); err != nil {
		return err
	} else if err = m.LoadConfig(ctx, configFile); err != nil {
		return err
	}

	return nil
}

func (m *Main) LoadConfig(ctx context.Context, filename string) error {
	if filename == "" {
		return errors.New("must specify a config file path using either CONFIG_FILE environment variable or the --config flag")
	}

	var b []byte
	var err error

	// If we're loading the config file from AWS we rely on AWS credential loading
	if dca.HasAWSParamStorePrefix(filename) {
		if b, err = dca.GetAWSParamStoreValue(ctx, filename); err != nil {
			return fmt.Errorf("failed to get AWS param store value for kraken api key: %v", err)
		}
	} else if b, err = os.ReadFile(filename); err != nil {
		return err
	}

	var config Config
	if err = json.Unmarshal(b, &config); err != nil {
		return err
	}

	if config.OrderAmountInCents <= 0 {
		return fmt.Errorf("orderAmountInCents cannot be less than or equal to zero")
	}

	if config.KrakenAPIKey == "" {
		return fmt.Errorf("krakenApiKey is required")
	}

	if dca.HasAWSParamStorePrefix(config.KrakenAPIKey) {
		if data, err := dca.GetAWSParamStoreValue(ctx, config.KrakenAPIKey); err != nil {
			return fmt.Errorf("failed to get AWS param store value for kraken api key: %v", err)
		} else {
			config.KrakenAPIKey = string(data)
		}
	}

	if config.KrakenPrivateKey == "" {
		return fmt.Errorf("krakenPrivateKey is required")
	}

	if dca.HasAWSParamStorePrefix(config.KrakenPrivateKey) {
		if data, err := dca.GetAWSParamStoreValue(ctx, config.KrakenPrivateKey); err != nil {
			return fmt.Errorf("failed to get AWS param store value: %v", err)
		} else {
			config.KrakenPrivateKey = string(data)
		}
	}

	// The default value for the private key is to be base64 encoded but it shouldn't be considered an error if the
	// value is not encoded.
	if data, err := base64.StdEncoding.DecodeString(config.KrakenPrivateKey); err == nil {
		config.KrakenPrivateKey = string(data)
	}

	m.Config = config
	return nil
}
