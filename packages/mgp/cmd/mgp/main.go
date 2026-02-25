package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/amarbel-llc/mgp/internal/catalog"
	"github.com/amarbel-llc/mgp/internal/tools"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/server"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/transport"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "mgp — model graph protocol MCP server\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  mgp [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Starts an MCP server on stdio that exposes the purse-first tool catalog via GraphQL.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	cat := catalog.NewCatalog()
	app := tools.RegisterAll(cat)

	if flag.NArg() == 2 && flag.Arg(0) == "generate-plugin" {
		if err := app.GenerateAll(flag.Arg(1)); err != nil {
			log.Fatalf("generating plugin: %v", err)
		}
		return
	}

	if flag.NArg() >= 1 && flag.Arg(0) == "hook" {
		if err := app.HandleHook(os.Stdin, os.Stdout); err != nil {
			log.Fatalf("handling hook: %v", err)
		}
		return
	}

	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "mgp: unexpected arguments: %v\n", flag.Args())
		flag.Usage()
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	t := transport.NewStdio(os.Stdin, os.Stdout)

	registry := server.NewToolRegistryV1()
	app.RegisterMCPToolsV1(registry)

	srv, err := server.New(t, server.Options{
		ServerName:    app.Name,
		ServerVersion: app.Version,
		Instructions:  "Model graph protocol MCP server. Query and execute tools from the purse-first tool catalog via GraphQL.",
		Tools:         registry,
	})
	if err != nil {
		log.Fatalf("creating server: %v", err)
	}

	if err := srv.Run(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
