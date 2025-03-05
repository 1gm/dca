package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/1gm/dca"
	"github.com/1gm/dca/cmd/internal"
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

	m := internal.NewMain()
	if err := m.ParseFlagsAndLoadConfig(ctx, args[1:]); err != nil {
		m.Logger.Error("error parsing flags", "error", err)
		return 1
	} else if err = m.Run(ctx); err != nil {
		m.Logger.Error("error running main", "error", err)
		return 1
	}

	return 0
}
