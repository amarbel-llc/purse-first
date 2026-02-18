package command

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/server"
)

func TestAppRegisterMCPTools(t *testing.T) {
	app := NewApp("grit", "Git MCP server")

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		Params: []Param{
			{Name: "repo_path", Type: String, Description: "Path to repo", Required: true},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			return TextResult("ok"), nil
		},
	})

	app.AddCommand(&Command{
		Name:   "internal",
		Hidden: true,
	})

	app.AddCommand(&Command{
		Name: "interactive",
		RunCLI: func(ctx context.Context, args json.RawMessage) error {
			return nil
		},
	})

	registry := server.NewToolRegistry()
	app.RegisterMCPTools(registry)

	tools, err := registry.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	if len(tools) != 1 {
		t.Fatalf("tools len = %d, want 1 (hidden and CLI-only excluded)", len(tools))
	}

	if tools[0].Name != "status" {
		t.Errorf("tools[0].Name = %q, want %q", tools[0].Name, "status")
	}

	if tools[0].Description != "Show status" {
		t.Errorf("tools[0].Description = %q, want %q", tools[0].Description, "Show status")
	}

	// Verify the schema has the right structure
	var schema map[string]any
	json.Unmarshal(tools[0].InputSchema, &schema)
	props := schema["properties"].(map[string]any)
	if _, ok := props["repo_path"]; !ok {
		t.Error("schema missing repo_path property")
	}
}

func TestAppMCPToolCall(t *testing.T) {
	app := NewApp("test", "test")

	app.AddCommand(&Command{
		Name: "echo",
		Params: []Param{
			{Name: "message", Type: String, Description: "Message to echo"},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			var params struct {
				Message string `json:"message"`
			}
			json.Unmarshal(args, &params)
			return TextResult(params.Message), nil
		},
	})

	registry := server.NewToolRegistry()
	app.RegisterMCPTools(registry)

	result, err := registry.CallTool(
		context.Background(),
		"echo",
		json.RawMessage(`{"message":"hello"}`),
	)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result.Content[0].Text != "hello" {
		t.Errorf("result = %q, want %q", result.Content[0].Text, "hello")
	}
}
