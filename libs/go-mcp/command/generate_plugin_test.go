package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratePlugin(t *testing.T) {
	app := NewApp("grit", "Git operations")
	app.Version = "0.1.0"

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
	})

	dir := t.TempDir()
	if err := app.GeneratePlugin(dir); err != nil {
		t.Fatalf("GeneratePlugin: %v", err)
	}

	path := filepath.Join(dir, "grit", "plugin.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read plugin.json: %v", err)
	}

	var plugin map[string]any
	if err := json.Unmarshal(data, &plugin); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if plugin["name"] != "grit" {
		t.Errorf("name = %v, want grit", plugin["name"])
	}

	servers := plugin["mcpServers"].(map[string]any)
	srv := servers["grit"].(map[string]any)
	if srv["type"] != "stdio" {
		t.Errorf("type = %v, want stdio", srv["type"])
	}
	if srv["command"] != "grit" {
		t.Errorf("command = %v, want grit", srv["command"])
	}
}

func TestGeneratePluginWithArgs(t *testing.T) {
	app := NewApp("lux", "LSP multiplexer")
	app.MCPArgs = []string{"mcp", "stdio"}

	dir := t.TempDir()
	if err := app.GeneratePlugin(dir); err != nil {
		t.Fatalf("GeneratePlugin: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "lux", "plugin.json"))
	var plugin map[string]any
	json.Unmarshal(data, &plugin)

	servers := plugin["mcpServers"].(map[string]any)
	srv := servers["lux"].(map[string]any)
	args := srv["args"].([]any)
	if len(args) != 2 || args[0] != "mcp" || args[1] != "stdio" {
		t.Errorf("args = %v, want [mcp stdio]", args)
	}
}
