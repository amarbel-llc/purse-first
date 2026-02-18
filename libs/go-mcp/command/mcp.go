package command

import (
	"context"
	"encoding/json"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/server"
)

// RegisterMCPTools registers all non-hidden commands as MCP tools
// in the given ToolRegistry, using each command's description and
// auto-generated JSON schema.
func (a *App) RegisterMCPTools(registry *server.ToolRegistry) {
	for name, cmd := range a.AllCommands() {
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
