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

func TestMappingBuilder(t *testing.T) {
	b := NewPluginBuilder("grit").
		Command("grit").
		Mapping("Bash").
		CommandPrefixes("git ").
		Tool("status", "checking repository status").
		Tool("diff", "viewing changes").
		Reason("Use grit MCP tools for git operations").
		Done()

	mf := b.BuildMappings()
	if mf == nil {
		t.Fatal("expected non-nil MappingFile")
	}

	if mf.Server != "grit" {
		t.Errorf("server = %q, want %q", mf.Server, "grit")
	}

	if len(mf.Mappings) != 1 {
		t.Fatalf("mappings len = %d, want 1", len(mf.Mappings))
	}

	m := mf.Mappings[0]
	if m.Replaces != "Bash" {
		t.Errorf("replaces = %q, want %q", m.Replaces, "Bash")
	}
	if len(m.CommandPrefixes) != 1 || m.CommandPrefixes[0] != "git " {
		t.Errorf("command_prefixes = %v, want [git ]", m.CommandPrefixes)
	}
	if len(m.Tools) != 2 {
		t.Fatalf("tools len = %d, want 2", len(m.Tools))
	}
	if m.Tools[0].Name != "status" {
		t.Errorf("tools[0].name = %q, want %q", m.Tools[0].Name, "status")
	}
	if m.Reason != "Use grit MCP tools for git operations" {
		t.Errorf("reason = %q", m.Reason)
	}
}

func TestBuildMappingsNilWhenEmpty(t *testing.T) {
	b := NewPluginBuilder("test").Command("test")
	mf := b.BuildMappings()
	if mf != nil {
		t.Errorf("expected nil MappingFile when no mappings declared, got %+v", mf)
	}
}

func TestMappingBuilderWithExtensions(t *testing.T) {
	b := NewPluginBuilder("lux").
		Command("lux", "mcp", "stdio").
		Mapping("Read").
		Extensions(".go", ".py").
		Tool("lsp_hover", "getting type info").
		Reason("Use lux").
		Done()

	mf := b.BuildMappings()
	if mf == nil {
		t.Fatal("expected non-nil MappingFile")
	}

	m := mf.Mappings[0]
	if len(m.Extensions) != 2 || m.Extensions[0] != ".go" {
		t.Errorf("extensions = %v, want [.go .py]", m.Extensions)
	}
	if len(m.CommandPrefixes) != 0 {
		t.Errorf("command_prefixes should be empty, got %v", m.CommandPrefixes)
	}
}

func TestMappingJSONRoundTrip(t *testing.T) {
	b := NewPluginBuilder("grit").
		Command("grit").
		Mapping("Bash").
		CommandPrefixes("git ").
		Tool("status", "checking status").
		Reason("Use grit").
		Done()

	mf := b.BuildMappings()

	data, err := json.MarshalIndent(mf, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got MappingFile
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Server != "grit" {
		t.Errorf("server = %q, want %q", got.Server, "grit")
	}
	if len(got.Mappings) != 1 {
		t.Fatalf("mappings len = %d, want 1", len(got.Mappings))
	}
	if got.Mappings[0].CommandPrefixes[0] != "git " {
		t.Errorf("command_prefixes[0] = %q, want %q", got.Mappings[0].CommandPrefixes[0], "git ")
	}
}
