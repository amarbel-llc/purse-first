package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/server"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/transport"
	"github.com/friedenberg/get-hubbed/internal/tools"
)

func main() {
	app := tools.RegisterAll()

	if len(os.Args) >= 3 && os.Args[1] == "generate-plugin" {
		if err := app.GenerateAll(os.Args[2]); err != nil {
			log.Fatalf("generating plugin: %v", err)
		}
		return
	}

	for _, arg := range os.Args[1:] {
		if arg == "-h" || arg == "--help" {
			fmt.Println("get-hubbed - a GitHub MCP server wrapping the gh CLI")
			fmt.Println()
			fmt.Println("Usage: get-hubbed")
			fmt.Println()
			fmt.Println("Runs an MCP server over stdio that exposes GitHub operations as tools.")
			os.Exit(0)
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	t := transport.NewStdio(os.Stdin, os.Stdout)

	registry := server.NewToolRegistry()
	app.RegisterMCPTools(registry)
	tools.RegisterAPITools(registry)

	srv, err := server.New(t, server.Options{
		ServerName:    app.Name,
		ServerVersion: app.Version,
		Tools:         registry,
	})
	if err != nil {
		log.Fatalf("creating server: %v", err)
	}

	if err := srv.Run(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
