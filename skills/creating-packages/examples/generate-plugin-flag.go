//go:build ignore

// This file is an illustrative snippet for the creating-packages skill.
// `//go:build ignore` keeps it out of `go build ./...` and `go test ./...`.

package main

import (
	"flag"
	"log"

	"github.com/amarbel-llc/purse-first/purse"
)

// Basic example: plugin manifest only (no mappings)
func exampleBasic() {
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

// Advanced example: plugin manifest with targeted per-subcommand mappings
//
// Mappings tell the purse-first hook to deny Bash commands matching configured
// prefixes and suggest specific MCP tools instead. Specific mappings must come
// before general ones because FindMatch returns the first match.
func exampleWithMappings() {
	flag.Parse()

	if flag.NArg() == 2 && flag.Arg(0) == "generate-plugin" {
		reason := "Use the my-mcp MCP tool instead of shelling out"

		b := purse.NewPluginBuilder("my-mcp").
			Command("my-mcp").
			StdioTransport().
			// Targeted: only suggest specific tool(s) for known subcommands
			Mapping("Bash").
			CommandPrefixes("mycli status").
			Tool("status", "checking status").
			Reason(reason).
			Done().
			Mapping("Bash").
			CommandPrefixes("mycli list").
			Tool("list", "listing items").
			Reason(reason).
			Done().
			// Catch-all: suggest all tools for unrecognized subcommands
			Mapping("Bash").
			CommandPrefixes("mycli ").
			Tool("status", "checking status").
			Tool("list", "listing items").
			Reason(reason).
			Done()

		p := b.Build()
		dir := flag.Arg(1)

		if err := purse.WritePlugin(dir, p); err != nil {
			log.Fatalf("generating plugin: %v", err)
		}

		if mf := b.BuildMappings(); mf != nil {
			if err := purse.WriteMappings(dir, p.Name, mf); err != nil {
				log.Fatalf("generating mappings: %v", err)
			}
		}

		return
	}

	// ... rest of main (MCP server setup)
}
