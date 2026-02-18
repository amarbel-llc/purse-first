//go:build ignore

// Example: Building an MCP server directly with registries.
//
// Use this pattern when you only need an MCP server (no CLI surface).
// This example creates a server with tools, resources, and prompts.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/output"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/server"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/transport"
)

func main() {
	tools := server.NewToolRegistry()
	resources := server.NewResourceRegistry()
	prompts := server.NewPromptRegistry()

	registerTools(tools)
	registerResources(resources)
	registerPrompts(prompts)

	t := transport.NewStdio(os.Stdin, os.Stdout)
	srv, err := server.New(t, server.Options{
		ServerName:    "example-server",
		ServerVersion: "1.0.0",
		Tools:         tools,
		Resources:     resources,
		Prompts:       prompts,
	})
	if err != nil {
		log.Fatalf("server: %v", err)
	}

	if err := srv.Run(context.Background()); err != nil {
		log.Fatalf("run: %v", err)
	}
}

func registerTools(tools *server.ToolRegistry) {
	// Tool with manual JSON schema.
	tools.Register(
		"echo",
		"Echoes back the provided message",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"message": {"type": "string", "description": "The message to echo back"}
			},
			"required": ["message"]
		}`),
		func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
			var params struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
			}

			return &protocol.ToolCallResult{
				Content: []protocol.ContentBlock{
					protocol.TextContent(params.Message),
				},
			}, nil
		},
	)

	// Tool with context-saving (text truncation).
	tools.Register(
		"read_file",
		"Read file contents with optional truncation",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "File path to read"},
				"head": {"type": "integer", "description": "Return only first N lines"},
				"tail": {"type": "integer", "description": "Return only last N lines"},
				"max_bytes": {"type": "integer", "description": "Maximum output bytes"}
			},
			"required": ["path"]
		}`),
		func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
			var params struct {
				Path     string `json:"path"`
				Head     int    `json:"head"`
				Tail     int    `json:"tail"`
				MaxBytes int    `json:"max_bytes"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
			}

			data, err := os.ReadFile(params.Path)
			if err != nil {
				return protocol.ErrorResult(fmt.Sprintf("read %s: %v", params.Path, err)), nil
			}

			// Apply context-saving truncation with defaults.
			defaults := output.StandardDefaults()
			limits := defaults.MergeTextLimits(output.TextLimits{
				Head:     params.Head,
				Tail:     params.Tail,
				MaxBytes: params.MaxBytes,
			})
			result := output.LimitText(string(data), limits)

			// Include truncation info in output when truncated.
			text := result.Content
			if result.Truncated && result.TruncationInfo != nil {
				info := result.TruncationInfo
				text += fmt.Sprintf("\n\n--- truncated (%s): showing %d/%d lines, %d/%d bytes ---",
					info.Position, info.KeptLines, info.OriginalLines,
					info.KeptBytes, info.OriginalBytes)
			}

			return &protocol.ToolCallResult{
				Content: []protocol.ContentBlock{protocol.TextContent(text)},
			}, nil
		},
	)
}

func registerResources(resources *server.ResourceRegistry) {
	// Static resource.
	resources.RegisterResource(
		protocol.Resource{
			URI:         "server://info",
			Name:        "Server Info",
			Description: "Runtime information about this server",
			MimeType:    "application/json",
		},
		func(ctx context.Context, uri string) (*protocol.ResourceReadResult, error) {
			info := map[string]string{
				"go_version": runtime.Version(),
				"os":         runtime.GOOS,
				"arch":       runtime.GOARCH,
			}
			data, _ := json.MarshalIndent(info, "", "  ")
			return &protocol.ResourceReadResult{
				Contents: []protocol.ResourceContent{
					{URI: uri, MimeType: "application/json", Text: string(data)},
				},
			}, nil
		},
	)
}

func registerPrompts(prompts *server.PromptRegistry) {
	prompts.Register(
		protocol.Prompt{
			Name:        "summarize",
			Description: "Generate a summary prompt for given content",
			Arguments: []protocol.PromptArgument{
				{Name: "content", Description: "The content to summarize", Required: true},
				{Name: "style", Description: "Summary style: brief, detailed, or bullets"},
			},
		},
		func(ctx context.Context, args map[string]string) (*protocol.PromptGetResult, error) {
			style := args["style"]
			if style == "" {
				style = "brief"
			}

			return &protocol.PromptGetResult{
				Description: "Content summary request",
				Messages: []protocol.PromptMessage{
					{
						Role: "user",
						Content: protocol.TextContent(fmt.Sprintf(
							"Please provide a %s summary of the following content:\n\n%s",
							style, args["content"],
						)),
					},
				},
			}, nil
		},
	)
}
