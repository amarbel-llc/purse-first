package command

import (
	"context"
	"encoding/json"

	"github.com/amarbel-llc/purse-first/libs/dewey/golf/protocol"
	"github.com/amarbel-llc/purse-first/libs/dewey/golf/server"
)

// RegisterMCPTools registers all non-hidden commands as MCP tools
// in the given ToolRegistry, using each command's description and
// auto-generated JSON schema.
func (u *Utility) RegisterMCPTools(registry *server.ToolRegistry) {
	for name, cmd := range u.AllCommands() {
		if cmd.Hidden || cmd.Run == nil {
			continue
		}

		run := cmd.Run // capture for closure
		registry.Register(
			name,
			cmd.Description.Short,
			cmd.InputSchema(),
			func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
				result, err := run(ctx, args, StubPrompter{})
				if err != nil {
					return nil, err
				}
				return resultToMCP(result), nil
			},
		)
	}
}

// RegisterMCPToolsV1 registers all non-hidden commands as V1 MCP tools
// in the given ToolRegistryV1.
func (u *Utility) RegisterMCPToolsV1(registry *server.ToolRegistryV1) {
	for name, cmd := range u.AllCommands() {
		if cmd.Hidden || cmd.Run == nil {
			continue
		}

		run := cmd.Run // capture for closure
		registry.Register(
			protocol.ToolV1{
				Name:        name,
				Title:       cmd.Title,
				Description: cmd.Description.Short,
				InputSchema: cmd.InputSchema(),
				Annotations: cmd.Annotations,
				Execution:   cmd.Execution,
			},
			func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResultV1, error) {
				result, err := run(ctx, args, StubPrompter{})
				if err != nil {
					return nil, err
				}
				return resultToMCPV1(result), nil
			},
		)
	}
}

func resultToMCPV1(r *Result) *protocol.ToolCallResultV1 {
	var text string
	if r.JSON != nil {
		data, _ := json.Marshal(r.JSON)
		text = string(data)
	} else {
		text = r.Text
	}
	return &protocol.ToolCallResultV1{
		Content: []protocol.ContentBlockV1{protocol.TextContentV1(text)},
		IsError: r.IsErr,
	}
}

func resultToMCP(r *Result) *protocol.ToolCallResult {
	var text string
	if r.JSON != nil {
		data, _ := json.Marshal(r.JSON)
		text = string(data)
	} else {
		text = r.Text
	}
	return &protocol.ToolCallResult{
		Content: []protocol.ContentBlock{protocol.TextContent(text)},
		IsError: r.IsErr,
	}
}
