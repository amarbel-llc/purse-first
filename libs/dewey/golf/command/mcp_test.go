package command

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/amarbel-llc/purse-first/libs/dewey/golf/protocol"
	"github.com/amarbel-llc/purse-first/libs/dewey/golf/server"
)

func TestAppRegisterMCPTools(t *testing.T) {
	app := NewUtility("grit", "Git MCP server")

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		OldParams: []OldParam{
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

func TestRegisterMCPToolsV1(t *testing.T) {
	app := NewUtility("grit", "Git MCP server")

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		OldParams: []OldParam{
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

	registry := server.NewToolRegistryV1()
	app.RegisterMCPToolsV1(registry)

	result, err := registry.ListToolsV1(context.Background(), "")
	if err != nil {
		t.Fatalf("ListToolsV1: %v", err)
	}

	if len(result.Tools) != 1 {
		t.Fatalf("tools len = %d, want 1 (hidden and CLI-only excluded)", len(result.Tools))
	}

	if result.Tools[0].Name != "status" {
		t.Errorf("tools[0].Name = %q, want %q", result.Tools[0].Name, "status")
	}

	if result.Tools[0].Description != "Show status" {
		t.Errorf("tools[0].Description = %q, want %q", result.Tools[0].Description, "Show status")
	}

	// Verify the schema has the right structure
	var schema map[string]any
	json.Unmarshal(result.Tools[0].InputSchema, &schema)
	props := schema["properties"].(map[string]any)
	if _, ok := props["repo_path"]; !ok {
		t.Error("schema missing repo_path property")
	}
}

func TestRegisterMCPToolsV1CallTool(t *testing.T) {
	app := NewUtility("test", "test")

	app.AddCommand(&Command{
		Name: "echo",
		OldParams: []OldParam{
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

	registry := server.NewToolRegistryV1()
	app.RegisterMCPToolsV1(registry)

	v1result, err := registry.CallToolV1(
		context.Background(),
		"echo",
		json.RawMessage(`{"message":"hello"}`),
	)
	if err != nil {
		t.Fatalf("CallToolV1: %v", err)
	}
	if v1result.Content[0].Text != "hello" {
		t.Errorf("result = %q, want %q", v1result.Content[0].Text, "hello")
	}
	if v1result.IsError {
		t.Error("unexpected IsError=true")
	}
}

func TestResultToMCPV1JSON(t *testing.T) {
	app := NewUtility("test", "test")

	app.AddCommand(&Command{
		Name: "json-cmd",
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			return JSONResult(map[string]string{"key": "value"}), nil
		},
	})

	registry := server.NewToolRegistryV1()
	app.RegisterMCPToolsV1(registry)

	v1result, err := registry.CallToolV1(
		context.Background(),
		"json-cmd",
		json.RawMessage(`{}`),
	)
	if err != nil {
		t.Fatalf("CallToolV1: %v", err)
	}
	if v1result.Content[0].Text != `{"key":"value"}` {
		t.Errorf("result = %q, want %q", v1result.Content[0].Text, `{"key":"value"}`)
	}
}

func TestAppMCPToolCall(t *testing.T) {
	app := NewUtility("test", "test")

	app.AddCommand(&Command{
		Name: "echo",
		OldParams: []OldParam{
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

func TestRegisterMCPToolsV1Annotations(t *testing.T) {
	app := NewUtility("test", "test")

	readOnly := true
	destructive := false
	idempotent := true
	openWorld := false

	app.AddCommand(&Command{
		Name:        "status",
		Title:       "Show Working Tree Status",
		Description: Description{Short: "Show status"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    &readOnly,
			DestructiveHint: &destructive,
			IdempotentHint:  &idempotent,
			OpenWorldHint:   &openWorld,
		},
		OldParams: []OldParam{
			{Name: "repo_path", Type: String, Required: true},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			return TextResult("ok"), nil
		},
	})

	registry := server.NewToolRegistryV1()
	app.RegisterMCPToolsV1(registry)

	result, err := registry.ListToolsV1(context.Background(), "")
	if err != nil {
		t.Fatalf("ListToolsV1: %v", err)
	}

	tool := result.Tools[0]

	if tool.Title != "Show Working Tree Status" {
		t.Errorf("title = %q, want %q", tool.Title, "Show Working Tree Status")
	}

	if tool.Annotations == nil {
		t.Fatal("annotations is nil")
	}

	if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint {
		t.Error("readOnlyHint should be true")
	}

	if tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
		t.Error("destructiveHint should be false")
	}
}
