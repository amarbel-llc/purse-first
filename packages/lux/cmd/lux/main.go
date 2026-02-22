package main

import (
	"context"
	"fmt"
	"os"
)

var version = "dev"

func main() {
	app := buildApp()
	if err := app.RunCLI(context.Background(), os.Args[1:], nil); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
