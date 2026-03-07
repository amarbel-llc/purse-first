package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/amarbel-llc/lux/internal/logfile"
	"github.com/amarbel-llc/lux/internal/tools"
)

var version = "dev"

func main() {
	cleanup := logfile.Init()
	defer cleanup()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app := buildApp()

	if len(os.Args) >= 2 && os.Args[1] == "generate-plugin" {
		tools.RegisterAll(app, nil)
		if err := app.HandleGeneratePlugin(os.Args[2:], os.Stdout); err != nil {
			fmt.Fprintf(logfile.Writer(), "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := app.RunCLI(ctx, os.Args[1:], nil); err != nil {
		if ctx.Err() != nil {
			// Clean shutdown via signal — not an error
			return
		}
		fmt.Fprintf(logfile.Writer(), "Error: %v\n", err)
		os.Exit(1)
	}
}
