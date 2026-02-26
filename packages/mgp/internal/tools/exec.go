package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/mgp/internal/catalog"
	"github.com/amarbel-llc/mgp/internal/mcpclient"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
)

func registerExecCommand(app *command.App, cat *catalog.Catalog) {
	app.AddCommand(&command.Command{
		Name:        "exec",
		Title:       "Execute MCP Tool",
		Description: command.Description{Short: "Execute a tool on an MCP server"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint: protocol.BoolPtr(false),
		},
		Params: []command.Param{
			{Name: "server", Type: command.String, Description: "MCP server name (e.g. grit, get-hubbed, chix)", Required: true},
			{Name: "tool", Type: command.String, Description: "Tool name to call", Required: true},
			{Name: "args", Type: command.Object, Description: "Arguments to pass to the tool as JSON object"},
		},
		Run: func(ctx context.Context, rawArgs json.RawMessage, _ command.Prompter) (*command.Result, error) {
			var params struct {
				Server string          `json:"server"`
				Tool   string          `json:"tool"`
				Args   json.RawMessage `json:"args"`
			}
			if err := json.Unmarshal(rawArgs, &params); err != nil {
				return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
			}

			entry, ok := cat.FindServer(params.Server)
			if !ok {
				return command.TextErrorResult(fmt.Sprintf("unknown server: %s", params.Server)), nil
			}

			client, err := mcpclient.Spawn(ctx, entry.Command, entry.Args...)
			if err != nil {
				return command.TextErrorResult(fmt.Sprintf("failed to spawn %s: %v", params.Server, err)), nil
			}
			defer client.Close()

			if err := client.Initialize(ctx); err != nil {
				return command.TextErrorResult(fmt.Sprintf("failed to initialize %s: %v", params.Server, err)), nil
			}

			result, err := client.CallTool(ctx, params.Tool, params.Args)
			if err != nil {
				return command.TextErrorResult(fmt.Sprintf("tool call failed: %v", err)), nil
			}

			return command.TextResult(string(result)), nil
		},
	})
}
