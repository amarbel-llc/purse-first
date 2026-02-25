package main

import (
	"context"
	"fmt"
	"os"

	"github.com/amarbel-llc/lux/internal/logfile"
)

var version = "dev"

func main() {
	cleanup := logfile.Init()
	defer cleanup()

	app := buildApp()
	if err := app.RunCLI(context.Background(), os.Args[1:], nil); err != nil {
		fmt.Fprintf(logfile.Writer(), "Error: %v\n", err)
		os.Exit(1)
	}
}
