package command

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
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

func TestRegisterMCPToolsV1(t *testing.T) {
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
	app := NewApp("test", "test")

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

func TestRegisterMCPToolsV1ResourceLink(t *testing.T) {
	app := NewApp("test", "test")

	app.AddCommand(&Command{
		Name: "merge",
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			return MultiContentResult(
				protocol.TextContentV1("ok 1 - merged\n  ---\n  output: spinclass://merge-output/abc123\n  ...\n"),
				protocol.ResourceLinkContent(
					"spinclass://merge-output/abc123",
					"merge log",
					"TAP output from the merge run",
					"text/plain",
				),
			), nil
		},
	})

	registry := server.NewToolRegistryV1()
	app.RegisterMCPToolsV1(registry)

	v1result, err := registry.CallToolV1(
		context.Background(),
		"merge",
		json.RawMessage(`{}`),
	)
	if err != nil {
		t.Fatalf("CallToolV1: %v", err)
	}

	if len(v1result.Content) != 2 {
		t.Fatalf("Content len = %d, want 2", len(v1result.Content))
	}
	if v1result.Content[0].Type != "text" {
		t.Errorf("Content[0].Type = %q, want %q", v1result.Content[0].Type, "text")
	}
	link := v1result.Content[1]
	if link.Type != "resource_link" {
		t.Errorf("Content[1].Type = %q, want %q", link.Type, "resource_link")
	}
	if link.URI != "spinclass://merge-output/abc123" {
		t.Errorf("Content[1].URI = %q", link.URI)
	}
	if link.Name != "merge log" {
		t.Errorf("Content[1].Name = %q", link.Name)
	}
	if link.MimeType != "text/plain" {
		t.Errorf("Content[1].MimeType = %q", link.MimeType)
	}
	if v1result.IsError {
		t.Error("unexpected IsError=true")
	}
}

func TestRegisterMCPToolsV1ResourceLinkHelper(t *testing.T) {
	app := NewApp("test", "test")

	app.AddCommand(&Command{
		Name: "open",
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			return ResourceLinkResult(
				"file:///tmp/x.log",
				"x.log",
				"raw log",
			), nil
		},
	})

	registry := server.NewToolRegistryV1()
	app.RegisterMCPToolsV1(registry)

	v1result, err := registry.CallToolV1(
		context.Background(),
		"open",
		json.RawMessage(`{}`),
	)
	if err != nil {
		t.Fatalf("CallToolV1: %v", err)
	}

	if len(v1result.Content) != 1 {
		t.Fatalf("Content len = %d, want 1", len(v1result.Content))
	}
	if v1result.Content[0].Type != "resource_link" {
		t.Errorf("Type = %q, want %q", v1result.Content[0].Type, "resource_link")
	}
	if v1result.Content[0].URI != "file:///tmp/x.log" {
		t.Errorf("URI = %q", v1result.Content[0].URI)
	}
}

func TestRegisterMCPToolsV1Annotations(t *testing.T) {
	app := NewApp("test", "test")

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
		Params: []Param{
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
