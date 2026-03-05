package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/amarbel-llc/lux/internal/logfile"
)

var version = "dev"

func main() {
	cleanup := logfile.Init()
	defer cleanup()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app := buildApp()
	if err := app.RunCLI(ctx, os.Args[1:], nil); err != nil {
		if ctx.Err() != nil {
			// Clean shutdown via signal — not an error
			return
		}
		fmt.Fprintf(logfile.Writer(), "Error: %v\n", err)
		os.Exit(1)
	}
}
