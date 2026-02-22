package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/amarbel-llc/go-lib-mcp/server"
	"github.com/amarbel-llc/go-lib-mcp/transport"
	"github.com/friedenberg/get-hubbed/internal/tools"
	"github.com/amarbel-llc/purse-first/purse"
)

func main() {
	if len(os.Args) >= 3 && os.Args[1] == "generate-plugin" {
		p := purse.NewPluginBuilder("get-hubbed").
			Command("get-hubbed").
			StdioTransport().
			Build()

		if err := purse.WritePlugin(os.Args[2], p); err != nil {
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

	srv, err := server.New(t, server.Options{
		ServerName:    "get-hubbed",
		ServerVersion: "0.1.0",
		Tools:         tools.RegisterAll(),
	})
	if err != nil {
		log.Fatalf("creating server: %v", err)
	}

	if err := srv.Run(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
