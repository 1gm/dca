package dca

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
)

var (
	// Version of the build executable, set by the build process.
	Version = "dev"
	// Commit of the built executable, set by the build process.
	Commit = "dev"
	// Date of the built executable, set by the build process.
	Date = "dev"
)

// AppConfig represents the configuration for App.
type AppConfig struct {
	// Kraken credentials
	KrakenAPIKey     string `json:"krakenApiKey"`
	KrakenPrivateKey string `json:"krakenPrivateKey"`
	// The amount of volume to try to buy in cents
	OrderAmountInCents int `json:"orderAmountInCents"`
}

// App represents the core functionality of the application.
type App struct {
	Config AppConfig
	Logger *slog.Logger
}

// NewApp creates a new App with an empty config and a JSON logger.
func NewApp() *App {
	return &App{
		Logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

// Run tries to execute a market order using a Kraken provider.
func (m *App) Run(ctx context.Context) (err error) {
	m.Logger.InfoContext(ctx, "starting process", "version", Version, "commit", Commit, "date", Date)

	provider := NewKrakenProvider(&KrakenProviderConfig{
		APIKey:    m.Config.KrakenAPIKey,
		APISecret: m.Config.KrakenPrivateKey,
		Logger:    m.Logger,
	})

	order := ExecuteOrderRequest{AmountInCents: m.Config.OrderAmountInCents}
	if res, err := provider.ExecuteOrder(ctx, order); err != nil {
		return err
	} else {
		m.Logger.Info("order successfully executed", "result", res)
	}

	return nil
}

// ParseFlagsAndLoadConfig parses the application config file from the --config flag and loads it.
func (m *App) ParseFlagsAndLoadConfig(ctx context.Context, args []string) error {
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

// LoadConfig loads a config file from the specified filename. If the filename has an AWS param store prefix
// then the value is loaded from AWS Systems Manager.
func (m *App) LoadConfig(ctx context.Context, filename string) error {
	if filename == "" {
		return errors.New("must specify a config file path using either CONFIG_FILE environment variable or the --config flag")
	}

	var b []byte
	var err error

	// If we're loading the config file from AWS we rely on AWS credential loading
	if HasAWSParamStorePrefix(filename) {
		if b, err = GetAWSParamStoreValue(ctx, filename); err != nil {
			return fmt.Errorf("failed to get AWS param store value for kraken api key: %v", err)
		}
	} else if b, err = os.ReadFile(filename); err != nil {
		return err
	}

	var config AppConfig
	if err = json.Unmarshal(b, &config); err != nil {
		return err
	}

	if config.OrderAmountInCents <= 0 {
		return errors.New("orderAmountInCents cannot be less than or equal to zero")
	}

	if config.KrakenAPIKey == "" {
		return errors.New("krakenApiKey is required")
	}

	if HasAWSParamStorePrefix(config.KrakenAPIKey) {
		if data, err := GetAWSParamStoreValue(ctx, config.KrakenAPIKey); err != nil {
			return fmt.Errorf("failed to get AWS param store value for kraken api key: %v", err)
		} else {
			config.KrakenAPIKey = string(data)
		}
	}

	if config.KrakenPrivateKey == "" {
		return errors.New("krakenPrivateKey is required")
	}

	if HasAWSParamStorePrefix(config.KrakenPrivateKey) {
		if data, err := GetAWSParamStoreValue(ctx, config.KrakenPrivateKey); err != nil {
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
