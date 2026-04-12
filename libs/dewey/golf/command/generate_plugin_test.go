package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratePlugin(t *testing.T) {
	app := NewUtility("grit", "Git operations")
	app.Version = "0.1.0"

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
	})

	dir := t.TempDir()
	if err := app.GeneratePlugin(dir); err != nil {
		t.Fatalf("GeneratePlugin: %v", err)
	}

	path := filepath.Join(dir, "grit", ".claude-plugin", "plugin.json")
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
	app := NewUtility("lux", "LSP multiplexer")
	app.MCPArgs = []string{"mcp-stdio"}

	dir := t.TempDir()
	if err := app.GeneratePlugin(dir); err != nil {
		t.Fatalf("GeneratePlugin: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "lux", ".claude-plugin", "plugin.json"))
	var plugin map[string]any
	json.Unmarshal(data, &plugin)

	servers := plugin["mcpServers"].(map[string]any)
	srv := servers["lux"].(map[string]any)
	args := srv["args"].([]any)
	if len(args) != 1 || args[0] != "mcp-stdio" {
		t.Errorf("args = %v, want [mcp-stdio]", args)
	}
}

func TestGeneratePluginUsesMCPBinary(t *testing.T) {
	app := NewUtility("sweatshop", "Worktree manager")
	app.MCPBinary = "sweatshop-mcp"

	dir := t.TempDir()
	if err := app.GeneratePlugin(dir); err != nil {
		t.Fatalf("GeneratePlugin: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "sweatshop", ".claude-plugin", "plugin.json"))
	var plugin map[string]any
	json.Unmarshal(data, &plugin)

	servers := plugin["mcpServers"].(map[string]any)
	srv := servers["sweatshop"].(map[string]any)
	if srv["command"] != "sweatshop-mcp" {
		t.Errorf("command = %v, want sweatshop-mcp", srv["command"])
	}
}

func TestGeneratePluginDefaultsMCPBinaryToName(t *testing.T) {
	app := NewUtility("grit", "Git operations")

	dir := t.TempDir()
	if err := app.GeneratePlugin(dir); err != nil {
		t.Fatalf("GeneratePlugin: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "grit", ".claude-plugin", "plugin.json"))
	var plugin map[string]any
	json.Unmarshal(data, &plugin)

	servers := plugin["mcpServers"].(map[string]any)
	srv := servers["grit"].(map[string]any)
	if srv["command"] != "grit" {
		t.Errorf("command = %v, want grit (default)", srv["command"])
	}
}

func TestGeneratePluginWithDescriptionAndAuthor(t *testing.T) {
	app := NewUtility("chix", "Nix MCP server")
	app.PluginDescription = "Nix MCP server and skills for Claude Code"
	app.PluginAuthor = "friedenberg"

	dir := t.TempDir()
	if err := app.GeneratePlugin(dir); err != nil {
		t.Fatalf("GeneratePlugin: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "chix", ".claude-plugin", "plugin.json"))
	var plugin map[string]any
	json.Unmarshal(data, &plugin)

	if plugin["description"] != "Nix MCP server and skills for Claude Code" {
		t.Errorf("description = %v, want 'Nix MCP server and skills for Claude Code'", plugin["description"])
	}

	author := plugin["author"].(map[string]any)
	if author["name"] != "friedenberg" {
		t.Errorf("author.name = %v, want friedenberg", author["name"])
	}
}

func TestGeneratePluginOmitsEmptyDescriptionAndAuthor(t *testing.T) {
	app := NewUtility("grit", "Git operations")

	dir := t.TempDir()
	if err := app.GeneratePlugin(dir); err != nil {
		t.Fatalf("GeneratePlugin: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "grit", ".claude-plugin", "plugin.json"))
	var plugin map[string]any
	json.Unmarshal(data, &plugin)

	if _, ok := plugin["description"]; ok {
		t.Errorf("description should be omitted when empty")
	}
	if _, ok := plugin["author"]; ok {
		t.Errorf("author should be omitted when empty")
	}
}
