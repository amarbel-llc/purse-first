package purse

import (
	"encoding/json"
	"testing"
)

func TestPluginBuilderBasic(t *testing.T) {
	p := NewPluginBuilder("grit").
		Command("grit").
		StdioTransport().
		Build()

	if p.Name != "grit" {
		t.Errorf("name = %q, want %q", p.Name, "grit")
	}
	if len(p.McpServers) != 1 {
		t.Fatalf("mcpServers len = %d, want 1", len(p.McpServers))
	}

	srv, ok := p.McpServers["grit"]
	if !ok {
		t.Fatal("mcpServers missing key 'grit'")
	}
	if srv.Type != "stdio" {
		t.Errorf("type = %q, want %q", srv.Type, "stdio")
	}
	if srv.Command != "grit" {
		t.Errorf("command = %q, want %q", srv.Command, "grit")
	}
	if len(srv.Args) != 0 {
		t.Errorf("args len = %d, want 0", len(srv.Args))
	}
}

func TestPluginBuilderWithArgs(t *testing.T) {
	p := NewPluginBuilder("lux").
		Command("lux", "mcp", "stdio").
		Build()

	srv := p.McpServers["lux"]
	if srv.Command != "lux" {
		t.Errorf("command = %q, want %q", srv.Command, "lux")
	}
	if len(srv.Args) != 2 || srv.Args[0] != "mcp" || srv.Args[1] != "stdio" {
		t.Errorf("args = %v, want [mcp stdio]", srv.Args)
	}
}

func TestPluginJSONRoundTrip(t *testing.T) {
	p := NewPluginBuilder("test").
		Command("test-cmd", "--flag").
		Build()

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Plugin
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Name != "test" {
		t.Errorf("name = %q, want %q", got.Name, "test")
	}

	srv, ok := got.McpServers["test"]
	if !ok {
		t.Fatal("mcpServers missing key 'test'")
	}
	if srv.Command != "test-cmd" {
		t.Errorf("command = %q, want %q", srv.Command, "test-cmd")
	}
	if len(srv.Args) != 1 || srv.Args[0] != "--flag" {
		t.Errorf("args = %v, want [--flag]", srv.Args)
	}
}

func TestPluginJSONShape(t *testing.T) {
	p := NewPluginBuilder("grit").
		Command("grit").
		StdioTransport().
		Build()

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var wire map[string]any
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatalf("unmarshal wire: %v", err)
	}

	if wire["name"] != "grit" {
		t.Errorf("wire name = %v", wire["name"])
	}

	servers, ok := wire["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("wire mcpServers is not a map")
	}

	srv, ok := servers["grit"].(map[string]any)
	if !ok {
		t.Fatal("wire mcpServers.grit is not a map")
	}

	if srv["type"] != "stdio" {
		t.Errorf("wire type = %v", srv["type"])
	}
	if srv["command"] != "grit" {
		t.Errorf("wire command = %v", srv["command"])
	}

	// Args should be omitted when empty
	if _, hasArgs := srv["args"]; hasArgs {
		t.Error("args should be omitted when empty")
	}
}

func TestPluginArgsOmitEmpty(t *testing.T) {
	p := NewPluginBuilder("test").
		Command("test").
		Build()

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var wire map[string]any
	json.Unmarshal(data, &wire)

	servers := wire["mcpServers"].(map[string]any)
	srv := servers["test"].(map[string]any)

	if _, hasArgs := srv["args"]; hasArgs {
		t.Error("args should be omitted when nil/empty")
	}
}
