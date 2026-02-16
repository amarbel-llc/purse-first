// Package server provides MCP server scaffolding for building custom servers.
package server

import (
	"context"
	"encoding/json"

	"github.com/amarbel-llc/go-lib-mcp/protocol"
)

// ToolProvider is implemented by servers that provide tools.
// Tools are functions that can be invoked by the client with JSON arguments.
type ToolProvider interface {
	// ListTools returns all available tools.
	ListTools(ctx context.Context) ([]protocol.Tool, error)

	// CallTool invokes a tool with the given arguments.
	CallTool(ctx context.Context, name string, args json.RawMessage) (*protocol.ToolCallResult, error)
}

// ResourceProvider is implemented by servers that provide resources.
// Resources are data sources (files, APIs, databases, etc.) that can be read by the client.
type ResourceProvider interface {
	// ListResources returns all available resources.
	ListResources(ctx context.Context) ([]protocol.Resource, error)

	// ReadResource reads the content of a resource by URI.
	ReadResource(ctx context.Context, uri string) (*protocol.ResourceReadResult, error)

	// ListResourceTemplates returns URI templates for parameterized resources.
	// This method is optional - if not needed, return an empty slice.
	ListResourceTemplates(ctx context.Context) ([]protocol.ResourceTemplate, error)
}

// PromptProvider is implemented by servers that provide prompt templates.
// Prompts are pre-defined message templates that can be instantiated with arguments.
type PromptProvider interface {
	// ListPrompts returns all available prompt templates.
	ListPrompts(ctx context.Context) ([]protocol.Prompt, error)

	// GetPrompt retrieves and renders a prompt with the given arguments.
	GetPrompt(ctx context.Context, name string, args map[string]string) (*protocol.PromptGetResult, error)
}
