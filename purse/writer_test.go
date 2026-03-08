package purse

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWritePlugin(t *testing.T) {
	dir := t.TempDir()

	p := NewPluginBuilder("test-server").
		Command("test-server").
		Build()

	if err := WritePlugin(dir, p); err != nil {
		t.Fatalf("WritePlugin: %v", err)
	}

	path := filepath.Join(dir, "test-server", ".claude-plugin", "plugin.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if data[len(data)-1] != '\n' {
		t.Error("expected trailing newline")
	}

	var got Plugin
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Name != "test-server" {
		t.Errorf("name = %q, want %q", got.Name, "test-server")
	}

	srv, ok := got.McpServers["test-server"]
	if !ok {
		t.Fatal("mcpServers missing key 'test-server'")
	}
	if srv.Command != "test-server" {
		t.Errorf("command = %q, want %q", srv.Command, "test-server")
	}
	if srv.Type != "stdio" {
		t.Errorf("type = %q, want %q", srv.Type, "stdio")
	}
}

func TestWritePluginCreatesSubdir(t *testing.T) {
	dir := t.TempDir()

	p := Plugin{
		Name: "my-plugin",
		McpServers: map[string]McpServer{
			"my-plugin": {
				Type:    "stdio",
				Command: "my-plugin",
			},
		},
	}

	if err := WritePlugin(dir, p); err != nil {
		t.Fatalf("WritePlugin: %v", err)
	}

	path := filepath.Join(dir, "my-plugin", ".claude-plugin", "plugin.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected plugin.json to exist")
	}
}

func TestWritePluginWithArgs(t *testing.T) {
	dir := t.TempDir()

	p := NewPluginBuilder("lux").
		Command("lux", "mcp", "stdio").
		Build()

	if err := WritePlugin(dir, p); err != nil {
		t.Fatalf("WritePlugin: %v", err)
	}

	path := filepath.Join(dir, "lux", ".claude-plugin", "plugin.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var got Plugin
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	srv := got.McpServers["lux"]
	if len(srv.Args) != 2 || srv.Args[0] != "mcp" || srv.Args[1] != "stdio" {
		t.Errorf("args = %v, want [mcp stdio]", srv.Args)
	}
}

func TestWriteMappings(t *testing.T) {
	dir := t.TempDir()

	mf := &MappingFile{
		Server: "grit",
		Mappings: []MappingEntry{
			{
				Replaces:        "Bash",
				CommandPrefixes: []string{"git "},
				Tools: []ToolSuggestion{
					{Name: "status", UseWhen: "checking status"},
				},
				Reason: "Use grit",
			},
		},
	}

	if err := WriteMappings(dir, "grit", mf); err != nil {
		t.Fatalf("WriteMappings: %v", err)
	}

	path := filepath.Join(dir, "grit", "mappings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if data[len(data)-1] != '\n' {
		t.Error("expected trailing newline")
	}

	var got MappingFile
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
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

func TestWriteMappingsNilNoOp(t *testing.T) {
	dir := t.TempDir()

	if err := WriteMappings(dir, "test", nil); err != nil {
		t.Fatalf("WriteMappings: %v", err)
	}

	path := filepath.Join(dir, "test", "mappings.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected mappings.json to not exist for nil MappingFile")
	}
}

func TestWritePluginAndMappingsTogether(t *testing.T) {
	dir := t.TempDir()

	b := NewPluginBuilder("grit").
		Command("grit").
		StdioTransport().
		Mapping("Bash").
		CommandPrefixes("git ").
		Tool("status", "checking status").
		Reason("Use grit").
		Done()

	p := b.Build()
	if err := WritePlugin(dir, p); err != nil {
		t.Fatalf("WritePlugin: %v", err)
	}

	mf := b.BuildMappings()
	if err := WriteMappings(dir, p.Name, mf); err != nil {
		t.Fatalf("WriteMappings: %v", err)
	}

	// plugin.json goes in .claude-plugin/, mappings.json stays at package root
	pluginPath := filepath.Join(dir, "grit", ".claude-plugin", "plugin.json")
	mappingPath := filepath.Join(dir, "grit", "mappings.json")

	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		t.Error("expected plugin.json to exist")
	}
	if _, err := os.Stat(mappingPath); os.IsNotExist(err) {
		t.Error("expected mappings.json to exist")
	}
}
