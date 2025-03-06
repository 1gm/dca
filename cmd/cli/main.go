package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/1gm/dca"
)

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

	app := dca.NewApp()
	if err := app.ParseFlagsAndLoadConfig(ctx, args[1:]); err != nil {
		app.Logger.Error("error parsing flags", "error", err)
		return 1
	} else if err = app.Run(ctx); err != nil {
		app.Logger.Error("error running main", "error", err)
		return 1
	}

	return 0
}
