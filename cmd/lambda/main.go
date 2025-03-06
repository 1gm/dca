package main

import (
	"context"
	"fmt"
	"os"

	"github.com/1gm/dca"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var configFileName = os.Getenv("CONFIG_FILE")

func handleRequest(ctx context.Context, event events.EventBridgeEvent) (string, error) {
	// Load the default configuration
	if configFileName == "" {
		return "", fmt.Errorf("no configuration file provided")
	}

	app := dca.NewApp()

	app.Logger.InfoContext(ctx, "processing event bridge message", "event", event)

	if err := app.LoadConfig(ctx, configFileName); err != nil {
		app.Logger.Error("error loading config", "error", err)
		return "", err
	} else if err = app.Run(ctx); err != nil {
		app.Logger.Error("error running main", "error", err)
		return "", err
	}

	return "Successfully processed messages", nil
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

	// Start the Lambda handler
	lambda.Start(handleRequest)
}
