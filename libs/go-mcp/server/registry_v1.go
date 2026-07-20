package server

import (
	"context"
	"encoding/json"
	"fmt"

	"code.linenisgreat.com/purse-first/libs/go-mcp/protocol"
)

// ToolHandlerV1 is a function that handles V1 tool invocations.
type ToolHandlerV1 func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResultV1, error)

// ToolRegistryV1 implements both ToolProvider and ToolProviderV1.
// It stores V1 tools with annotations, icons, and output schemas.
type ToolRegistryV1 struct {
	tools    []protocol.ToolV1
	handlers map[string]ToolHandlerV1
}

// NewToolRegistryV1 creates a new empty V1 tool registry.
func NewToolRegistryV1() *ToolRegistryV1 {
	return &ToolRegistryV1{
		handlers: make(map[string]ToolHandlerV1),
	}
}

// Register adds a V1 tool to the registry.
func (r *ToolRegistryV1) Register(tool protocol.ToolV1, handler ToolHandlerV1) {
	r.tools = append(r.tools, tool)
	r.handlers[tool.Name] = handler
}

// ListTools implements ToolProvider by downgrading V1 tools to V0.
func (r *ToolRegistryV1) ListTools(ctx context.Context) ([]protocol.Tool, error) {
	v0tools := make([]protocol.Tool, len(r.tools))
	for i, t := range r.tools {
		v0tools[i] = protocol.Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}
	return v0tools, nil
}

// CallTool implements ToolProvider by downgrading V1 results to V0.
func (r *ToolRegistryV1) CallTool(ctx context.Context, name string, args json.RawMessage) (*protocol.ToolCallResult, error) {
	handler, ok := r.handlers[name]
	if !ok {
		return protocol.ErrorResult(fmt.Sprintf("unknown tool: %s", name)), nil
	}
	v1result, err := handler(ctx, args)
	if err != nil {
		return nil, err
	}
	// Downgrade V1 content blocks to V0
	v0content := make([]protocol.ContentBlock, 0, len(v1result.Content))
	for _, c := range v1result.Content {
		v0content = append(v0content, protocol.ContentBlock{
			Type:     c.Type,
			Text:     c.Text,
			MimeType: c.MimeType,
			Data:     c.Data,
		})
	}
	return &protocol.ToolCallResult{
		Content: v0content,
		IsError: v1result.IsError,
	}, nil
}

// ListToolsV1 implements ToolProviderV1.
func (r *ToolRegistryV1) ListToolsV1(ctx context.Context, cursor string) (*protocol.ToolsListResultV1, error) {
	return &protocol.ToolsListResultV1{
		Tools: r.tools,
	}, nil
}

// CallToolV1 implements ToolProviderV1.
func (r *ToolRegistryV1) CallToolV1(ctx context.Context, name string, args json.RawMessage) (*protocol.ToolCallResultV1, error) {
	handler, ok := r.handlers[name]
	if !ok {
		return protocol.ErrorResultV1(fmt.Sprintf("unknown tool: %s", name)), nil
	}
	return handler(ctx, args)
}
