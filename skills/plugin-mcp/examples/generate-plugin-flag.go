package main

import (
	"flag"
	"log"

	"github.com/amarbel-llc/purse-first/purse"
)

// Add this block at the top of main(), after flag.Parse():
func exampleMain() {
	flag.Parse()

	if flag.NArg() == 2 && flag.Arg(0) == "generate-plugin" {
		p := purse.NewPluginBuilder("my-mcp").
			Command("my-mcp").
			StdioTransport().
			Build()

		if err := purse.WritePlugin(flag.Arg(1), p); err != nil {
			log.Fatalf("generating plugin: %v", err)
		}

		return
	}

	// ... rest of main (MCP server setup)
}
