package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/server"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/transport"
	"github.com/friedenberg/grit/internal/tools"
	intTransport "github.com/friedenberg/grit/internal/transport"
)

func main() {
	sseMode := flag.Bool("sse", false, "Use HTTP/SSE transport instead of stdio")
	port := flag.Int("port", 8080, "Port for HTTP/SSE transport")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "grit — an MCP server exposing git operations\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  grit [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Starts an MCP server on stdio (default) or HTTP/SSE.\n")
		fmt.Fprintf(os.Stderr, "Intended to be launched by an MCP client such as Claude Code.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  grit                     # stdio transport\n")
		fmt.Fprintf(os.Stderr, "  grit --sse --port 8080   # HTTP/SSE transport\n")
	}

	flag.Parse()

	app := tools.RegisterAll()

	if flag.NArg() == 2 && flag.Arg(0) == "generate-plugin" {
		if err := app.GenerateAll(flag.Arg(1)); err != nil {
			log.Fatalf("generating plugin: %v", err)
		}
		return
	}

	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "grit: unexpected arguments: %v\n", flag.Args())
		flag.Usage()
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	var t transport.Transport

	if *sseMode {
		sse := intTransport.NewSSE(fmt.Sprintf(":%d", *port))
		if err := sse.Start(ctx); err != nil {
			log.Fatalf("starting SSE transport: %v", err)
		}
		defer sse.Close()
		log.Printf("SSE transport listening on %s", sse.Addr())
		t = sse
	} else {
		t = transport.NewStdio(os.Stdin, os.Stdout)
	}

	registry := server.NewToolRegistry()
	app.RegisterMCPTools(registry)

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
