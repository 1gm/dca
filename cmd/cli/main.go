package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/1gm/dca"
	"log/slog"
	"os"
	"os/signal"
)

type Config struct {
	EnableLogging      bool   `json:"enableLogging"`
	KrakenAPIKey       string `json:"krakenApiKey"`
	KrakenPrivateKey   string `json:"krakenPrivateKey"`
	OrderAmountInCents int    `json:"orderAmountInCents"`
}

func DefaultConfig() Config {
	return Config{
		EnableLogging: true,
	}
}

var (
	version = "NA"
	commit  = "NA"
	date    = "NA"
)

func main() {
	dca.Version = version
	dca.Commit = commit
	dca.Date = date

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	os.Exit(realMain(logger, os.Args))
}

func realMain(logger *slog.Logger, args []string) int {
	// shutdown context
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, os.Kill)
	go func() { <-ch; cancel() }()

	m := NewMain()
	if err := m.ParseFlagsAndLoadConfig(ctx, args[1:]); err != nil {
		logger.Error("error parsing flags", "error", err)
		return 1
	} else if err = m.Run(ctx); err != nil {
		logger.Error("error running main", "error", err)
		return 1
	}

	return 0
}

type Main struct {
	Config Config
	Logger *slog.Logger
}

func NewMain() *Main {
	return &Main{
		Logger: slog.New(slog.DiscardHandler),
	}
}

// ParseFlagsAndLoadConfig parses the application config file from the --config flag and loads it.
func (m *Main) ParseFlagsAndLoadConfig(ctx context.Context, args []string) error {
	var configFile string

	fs := flag.NewFlagSet("dca", flag.ContinueOnError)
	fs.StringVar(&configFile, "config", os.Getenv("CONFIG_FILE"), "path to the config file")

	if err := fs.Parse(args); err != nil {
		return err
	} else if m.Config, err = LoadConfig(ctx, configFile); err != nil {
		return err
	}

	return nil
}

func (m *Main) Run(ctx context.Context) (err error) {
	if m.Config.EnableLogging {
		m.Logger = slog.New(slog.NewJSONHandler(os.Stdout, nil)).WithGroup("build").With("version", version, "commit", commit, "date", date)
	}

	m.Logger.Info("starting process")

	provider := dca.NewKrakenProvider(&dca.KrakenProviderConfig{
		APIKey:    m.Config.KrakenAPIKey,
		APISecret: m.Config.KrakenPrivateKey,
		Logger:    m.Logger,
	})

	order := dca.Order{AmountInCents: m.Config.OrderAmountInCents}
	if err = provider.PlaceOrder(ctx, order); err != nil {
		return err
	}

	m.Logger.Info("process ran to completion")
	return nil
}

// LoadConfig will load a config file from the path specified by filename.
// Note: The context parameter will be used for loading config files from remote storage.
func LoadConfig(ctx context.Context, filename string) (Config, error) {
	config := DefaultConfig()
	if filename == "" {
		return config, errors.New("must specify a config file path using either CONFIG_FILE environment variable or the --config flag")
	}

	var b []byte
	var err error

	// If we're loading the config file from AWS we rely on AWS credential loading
	if dca.HasAWSParamStorePrefix(filename) {
		if b, err = dca.GetAWSParamStoreValue(ctx, filename); err != nil {
			return config, fmt.Errorf("failed to get AWS param store value for kraken api key: %v", err)
		}
	} else if b, err = os.ReadFile(filename); err != nil {
		return config, err
	}

	if err = json.Unmarshal(b, &config); err != nil {
		return config, err
	}

	if config.OrderAmountInCents <= 0 {
		return config, fmt.Errorf("orderAmountInCents cannot be less than or equal to zero")
	}

	if config.KrakenAPIKey == "" {
		return config, fmt.Errorf("krakenApiKey is required")
	}

	if dca.HasAWSParamStorePrefix(config.KrakenAPIKey) {
		if data, err := dca.GetAWSParamStoreValue(ctx, config.KrakenAPIKey); err != nil {
			return config, fmt.Errorf("failed to get AWS param store value for kraken api key: %v", err)
		} else {
			config.KrakenAPIKey = string(data)
		}
	}

	if config.KrakenPrivateKey == "" {
		return config, fmt.Errorf("krakenPrivateKey is required")
	}

	if dca.HasAWSParamStorePrefix(config.KrakenPrivateKey) {
		if data, err := dca.GetAWSParamStoreValue(ctx, config.KrakenPrivateKey); err != nil {
			return config, fmt.Errorf("failed to get AWS param store value: %v", err)
		} else {
			config.KrakenPrivateKey = string(data)
		}
	}

	// The default value for the private key is to be base64 encoded but it shouldn't be considered an error if the
	// value is not encoded.
	if data, err := base64.StdEncoding.DecodeString(config.KrakenPrivateKey); err == nil {
		config.KrakenPrivateKey = string(data)
	}

	return config, nil
}
