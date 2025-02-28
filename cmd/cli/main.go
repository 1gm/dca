package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/1gm/dca"
	"github.com/joho/godotenv"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
)

type Config struct {
	AWS struct {
		AccessKeyID     string `env:"AWS_ACCESS_KEY_ID"`
		AccessKeySecret string `env:"AWS_SECRET_ACCESS_KEY"`
		Region          string `env:"AWS_REGION"`
	}

	EnableLogging      bool   `env:"ENABLE_LOGGING"`
	APIKey             string `env:"KRAKEN_API_KEY"`
	PrivateKey         string `env:"KRAKEN_PRIVATE_KEY"`
	OrderAmountInCents int    `env:"ORDER_AMOUNT_CENTS"`
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

	os.Exit(realMain(os.Args))
}

func realMain(args []string) int {
	// shutdown context
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, os.Kill)
	go func() { <-ch; cancel() }()

	main := NewMain()
	if err := main.Run(ctx); err != nil {
		log.Printf("%+v", err)
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
		Config: Config{
			AWS: struct {
				AccessKeyID     string `env:"AWS_ACCESS_KEY_ID"`
				AccessKeySecret string `env:"AWS_SECRET_ACCESS_KEY"`
				Region          string `env:"AWS_REGION"`
			}{AccessKeyID: "", AccessKeySecret: "", Region: ""},
		},
		Logger: slog.New(slog.DiscardHandler),
	}
}

func (m *Main) Run(ctx context.Context) (err error) {
	if err := godotenv.Load(); err != nil {
		return fmt.Errorf("loading .env file: %w", err)
	}

	if val := os.Getenv("ENABLE_LOGGING"); val != "" {
		if m.Config.EnableLogging, err = strconv.ParseBool(os.Getenv("ENABLE_LOGGING")); err != nil {
			return fmt.Errorf("failed to parse ENABLE_LOGGING environment variable: %v", err)
		}
	}

	if m.Config.EnableLogging {
		m.Logger = slog.New(slog.NewJSONHandler(os.Stdout, nil)).WithGroup("build").With("version", version, "commit", commit, "date", date)
	}

	if val := os.Getenv("ORDER_AMOUNT_CENTS"); val != "" && val != "0" {
		if m.Config.OrderAmountInCents, err = strconv.Atoi(val); err != nil {
			return fmt.Errorf("failed to parse ORDER_AMOUNT_CENTS environment variable: %v", err)
		}

		if m.Config.OrderAmountInCents <= 0 {
			return fmt.Errorf("ORDER_AMOUNT_CENTS cannot be less than or equal to zero")
		}
	}

	if m.Config.APIKey = os.Getenv("KRAKEN_API_KEY"); m.Config.APIKey == "" {
		return errors.New("KRAKEN_API_KEY environment variable not set")
	} else if dca.HasAWSParamStorePrefix(m.Config.APIKey) {
		if data, err := dca.GetAWSParamStoreValue(ctx, m.Config.APIKey, m.Config.AWS.Region); err != nil {
			return fmt.Errorf("failed to get AWS param store value: %v", err)
		} else {
			m.Config.APIKey = string(data)
		}
	}

	if m.Config.PrivateKey = os.Getenv("KRAKEN_PRIVATE_KEY"); m.Config.PrivateKey == "" {
		return errors.New("KRAKEN_API_KEY environment variable not set")
	} else if dca.HasAWSParamStorePrefix(m.Config.PrivateKey) {
		if data, err := dca.GetAWSParamStoreValue(ctx, m.Config.PrivateKey, m.Config.AWS.Region); err != nil {
			return fmt.Errorf("failed to get AWS param store value: %v", err)
		} else {
			m.Config.PrivateKey = string(data)
		}
	}

	if data, err := base64.StdEncoding.DecodeString(m.Config.PrivateKey); err != nil {
		return fmt.Errorf("failed to decode KRAKEN_PRIVATE_KEY environment variable: %v", err)
	} else {
		m.Config.PrivateKey = string(data)
	}

	m.Logger.Info("starting job")

	provider := dca.NewKrakenProvider()
	provider.Logger = m.Logger.With("name", "kraken.provider")
	provider.Client = dca.NewKrakenHTTPClient(m.Config.APIKey, m.Config.PrivateKey)
	provider.Client.Logger = m.Logger.With("name", "kraken.client")

	order := dca.Order{AmountInCents: m.Config.OrderAmountInCents}
	if err = provider.PlaceOrder(ctx, order); err != nil {
		return err
	}

	m.Logger.Info("process ran to completion")
	return nil
}
