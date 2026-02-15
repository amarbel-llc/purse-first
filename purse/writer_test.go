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

	path := filepath.Join(dir, "test-server", "plugin.json")
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

	path := filepath.Join(dir, "my-plugin", "plugin.json")
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

	path := filepath.Join(dir, "lux", "plugin.json")
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
