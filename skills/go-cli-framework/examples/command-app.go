//go:build ignore

// Example: Building a CLI + MCP tool using command.App.
//
// This creates a "fileinfo" tool that provides file metadata.
// From a single command definition you get:
//   - MCP tool registration with auto-generated JSON schema
//   - Bash command interception (redirects "stat" to the MCP tool)
//   - Plugin manifest, mappings, manpages, and shell completions
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/output"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/server"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/transport"
)

func main() {
	app := command.NewApp("fileinfo", "File metadata MCP server")
	app.Version = "0.1.0"

	// Command with required and optional params, bash mapping, and MCP handler.
	app.AddCommand(&command.Command{
		Name:    "stat",
		Aliases: []string{"info"},
		Description: command.Description{
			Short: "Get file metadata",
			Long:  "Returns size, permissions, and modification time for a file.",
		},
		Params: []command.Param{
			{Name: "path", Type: command.String, Description: "File path to inspect", Required: true},
			{Name: "format", Type: command.String, Description: "Output format: json or text", Default: "text"},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"stat "}, UseWhen: "getting file metadata"},
		},
		RunMCP: func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
			var params struct {
				Path   string `json:"path"`
				Format string `json:"format"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
			}

			info, err := os.Stat(params.Path)
			if err != nil {
				return protocol.ErrorResult(fmt.Sprintf("stat %s: %v", params.Path, err)), nil
			}

			text := fmt.Sprintf("name: %s\nsize: %d\nmode: %s\nmodified: %s",
				info.Name(), info.Size(), info.Mode(), info.ModTime())

			return &protocol.ToolCallResult{
				Content: []protocol.ContentBlock{protocol.TextContent(text)},
			}, nil
		},
	})

	// Command using context-saving (output.LimitArray) for paginated results.
	app.AddCommand(&command.Command{
		Name:        "ls",
		Description: command.Description{Short: "List directory contents"},
		Params: []command.Param{
			{Name: "path", Type: command.String, Description: "Directory path", Required: true},
			{Name: "offset", Type: command.Int, Description: "Skip first N entries. Defaults to 0."},
			{Name: "limit", Type: command.Int, Description: "Maximum entries to return."},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"ls "}, UseWhen: "listing directory contents"},
		},
		RunMCP: func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
			var params struct {
				Path   string `json:"path"`
				Offset int    `json:"offset"`
				Limit  int    `json:"limit"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
			}

			entries, err := os.ReadDir(params.Path)
			if err != nil {
				return protocol.ErrorResult(fmt.Sprintf("readdir %s: %v", params.Path, err)), nil
			}

			// Convert to strings for pagination.
			names := make([]string, len(entries))
			for i, e := range entries {
				names[i] = e.Name()
			}

			// Apply context-saving pagination.
			defaults := output.StandardDefaults()
			limits := defaults.MergeArrayLimits(output.ArrayLimits{
				Offset: params.Offset,
				Limit:  params.Limit,
			})
			result := output.LimitArray(names, limits)

			// Marshal result with pagination metadata.
			data, _ := json.MarshalIndent(result, "", "  ")
			return &protocol.ToolCallResult{
				Content: []protocol.ContentBlock{protocol.TextContent(string(data))},
			}, nil
		},
	})

	// Hidden command: not exposed as MCP tool, used for artifact generation.
	app.AddCommand(&command.Command{
		Name:        "generate",
		Description: command.Description{Short: "Generate plugin artifacts"},
		Hidden:      true,
	})

	// Dispatch based on CLI arguments.
	if len(os.Args) > 1 && os.Args[1] == "generate" {
		if len(os.Args) < 3 {
			log.Fatal("usage: fileinfo generate <output-dir>")
		}
		if err := app.GenerateAll(os.Args[2]); err != nil {
			log.Fatalf("generate: %v", err)
		}
		return
	}

	// Default: run as MCP server.
	registry := server.NewToolRegistry()
	app.RegisterMCPTools(registry)

	t := transport.NewStdio(os.Stdin, os.Stdout)
	srv, err := server.New(t, server.Options{
		ServerName:    app.Name,
		ServerVersion: app.Version,
		Tools:         registry,
	})
	if err != nil {
		log.Fatalf("server: %v", err)
	}

	if err := srv.Run(context.Background()); err != nil {
		log.Fatalf("run: %v", err)
	}
}
