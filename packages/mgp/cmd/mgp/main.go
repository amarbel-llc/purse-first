package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/amarbel-llc/mgp/internal/catalog"
	"github.com/amarbel-llc/mgp/internal/tools"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/server"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/transport"
)

func main() {
	pluginsDir := flag.String("plugins-dir", "", "path to share/purse-first/ directory containing plugin.json files")
	binDir := flag.String("bin-dir", "", "path to bin/ directory containing MCP server binaries")
	graphqlServer := flag.String("graphql-server", "", "command to spawn as GraphQL server (newline-delimited JSON over stdio)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "mgp — model graph protocol MCP server\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  mgp [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Starts an MCP server on stdio that exposes the purse-first tool catalog via GraphQL.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// generate-plugin and hook subcommands work with an empty catalog
	if flag.NArg() >= 1 && flag.Arg(0) == "generate-plugin" {
		cat := catalog.NewCatalog()
		app := tools.RegisterAll(cat)
		if err := app.HandleGeneratePlugin(flag.Args()[1:], os.Stdout); err != nil {
			log.Fatalf("generating plugin: %v", err)
		}
		return
	}

	if flag.NArg() >= 1 && flag.Arg(0) == "hook" {
		cat := catalog.NewCatalog()
		app := tools.RegisterAll(cat)
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

	cat := discoverCatalog(ctx, *pluginsDir, *binDir)

	if *graphqlServer != "" {
		if err := catalog.DiscoverGraphQL(ctx, cat, *graphqlServer); err != nil {
			log.Fatalf("graphql server discovery failed: %v", err)
		}
	}

	app := tools.RegisterAll(cat)

	t := transport.NewStdio(os.Stdin, os.Stdout)

	registry := server.NewToolRegistryV1()
	app.RegisterMCPToolsV1(registry)

	resources := server.NewResourceRegistry()
	resources.RegisterResource(
		protocol.Resource{
			URI:         "mgp://catalog",
			Name:        "Tool Catalog",
			Description: "Complete catalog of tools available across all MCP servers",
			MimeType:    "application/json",
		},
		func(ctx context.Context, uri string) (*protocol.ResourceReadResult, error) {
			data, err := catalog.CatalogResourceJSON(cat)
			if err != nil {
				return nil, fmt.Errorf("serializing catalog: %w", err)
			}
			return &protocol.ResourceReadResult{
				Contents: []protocol.ResourceContent{{
					URI:      uri,
					MimeType: "application/json",
					Text:     string(data),
				}},
			}, nil
		},
	)

	srv, err := server.New(t, server.Options{
		ServerName:    app.Name,
		ServerVersion: app.Version,
		Instructions:  "Model graph protocol MCP server. Query and execute tools from the purse-first tool catalog via GraphQL.",
		Tools:         registry,
		Resources:     resources,
	})
	if err != nil {
		log.Fatalf("creating server: %v", err)
	}

	if err := srv.Run(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}

	if cat.GraphQLClient != nil {
		cat.GraphQLClient.Close()
	}
}

func discoverCatalog(ctx context.Context, pluginsDir, binDir string) *catalog.Catalog {
	if pluginsDir == "" || binDir == "" {
		exe, err := os.Executable()
		if err != nil {
			log.Printf("warning: resolving executable path: %v; using empty catalog", err)
			return catalog.NewCatalog()
		}

		root := filepath.Dir(filepath.Dir(exe))

		if pluginsDir == "" {
			pluginsDir = filepath.Join(root, "share", "purse-first")
		}

		if binDir == "" {
			binDir = filepath.Join(root, "bin")
		}
	}

	cat, err := catalog.Discover(ctx, pluginsDir, binDir, "mgp")
	if err != nil {
		log.Printf("warning: catalog discovery failed: %v; using empty catalog", err)
		return catalog.NewCatalog()
	}

	return cat
}
