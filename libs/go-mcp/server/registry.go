package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/go-lib-mcp/protocol"
)

// ToolRegistry is a helper for building tool providers.
// It maintains a map of tool names to handlers and implements the ToolProvider interface.
type ToolRegistry struct {
	tools    []protocol.Tool
	handlers map[string]ToolHandler
}

// ToolHandler is a function that handles tool invocations.
type ToolHandler func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error)

// NewToolRegistry creates a new empty tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		handlers: make(map[string]ToolHandler),
	}
}

// Register adds a tool to the registry.
func (r *ToolRegistry) Register(name, description string, schema json.RawMessage, handler ToolHandler) {
	r.tools = append(r.tools, protocol.Tool{
		Name:        name,
		Description: description,
		InputSchema: schema,
	})
	r.handlers[name] = handler
}

// ListTools implements ToolProvider.
func (r *ToolRegistry) ListTools(ctx context.Context) ([]protocol.Tool, error) {
	return r.tools, nil
}

// CallTool implements ToolProvider.
func (r *ToolRegistry) CallTool(ctx context.Context, name string, args json.RawMessage) (*protocol.ToolCallResult, error) {
	handler, ok := r.handlers[name]
	if !ok {
		return protocol.ErrorResult(fmt.Sprintf("unknown tool: %s", name)), nil
	}
	return handler(ctx, args)
}

// ResourceRegistry is a helper for building resource providers.
type ResourceRegistry struct {
	resources []protocol.Resource
	templates []protocol.ResourceTemplate
	readers   map[string]ResourceReader
}

// ResourceReader is a function that reads resource content.
type ResourceReader func(ctx context.Context, uri string) (*protocol.ResourceReadResult, error)

// NewResourceRegistry creates a new empty resource registry.
func NewResourceRegistry() *ResourceRegistry {
	return &ResourceRegistry{
		readers: make(map[string]ResourceReader),
	}
}

// RegisterResource adds a static resource to the registry.
func (r *ResourceRegistry) RegisterResource(resource protocol.Resource, reader ResourceReader) {
	r.resources = append(r.resources, resource)
	r.readers[resource.URI] = reader
}

// RegisterTemplate adds a resource template to the registry.
func (r *ResourceRegistry) RegisterTemplate(template protocol.ResourceTemplate, reader ResourceReader) {
	r.templates = append(r.templates, template)
	// For templates, we can't pre-register the reader since URIs are dynamic
	// Users should handle template URIs in their reader implementation
}

// ListResources implements ResourceProvider.
func (r *ResourceRegistry) ListResources(ctx context.Context) ([]protocol.Resource, error) {
	return r.resources, nil
}

// ReadResource implements ResourceProvider.
func (r *ResourceRegistry) ReadResource(ctx context.Context, uri string) (*protocol.ResourceReadResult, error) {
	reader, ok := r.readers[uri]
	if !ok {
		return nil, fmt.Errorf("unknown resource: %s", uri)
	}
	return reader(ctx, uri)
}

// ListResourceTemplates implements ResourceProvider.
func (r *ResourceRegistry) ListResourceTemplates(ctx context.Context) ([]protocol.ResourceTemplate, error) {
	return r.templates, nil
}

// PromptRegistry is a helper for building prompt providers.
type PromptRegistry struct {
	prompts   []protocol.Prompt
	renderers map[string]PromptRenderer
}

// PromptRenderer is a function that renders a prompt with arguments.
type PromptRenderer func(ctx context.Context, args map[string]string) (*protocol.PromptGetResult, error)

// NewPromptRegistry creates a new empty prompt registry.
func NewPromptRegistry() *PromptRegistry {
	return &PromptRegistry{
		renderers: make(map[string]PromptRenderer),
	}
}

// Register adds a prompt to the registry.
func (r *PromptRegistry) Register(prompt protocol.Prompt, renderer PromptRenderer) {
	r.prompts = append(r.prompts, prompt)
	r.renderers[prompt.Name] = renderer
}

// ListPrompts implements PromptProvider.
func (r *PromptRegistry) ListPrompts(ctx context.Context) ([]protocol.Prompt, error) {
	return r.prompts, nil
}

// GetPrompt implements PromptProvider.
func (r *PromptRegistry) GetPrompt(ctx context.Context, name string, args map[string]string) (*protocol.PromptGetResult, error) {
	renderer, ok := r.renderers[name]
	if !ok {
		return nil, fmt.Errorf("unknown prompt: %s", name)
	}
	return renderer(ctx, args)
}
