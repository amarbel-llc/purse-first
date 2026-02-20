package localplugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallMCPServersWritesToSettings(t *testing.T) {
	root := t.TempDir()

	// Create plugin.json with an MCP server
	pluginDir := filepath.Join(root, ".claude-plugin")
	os.MkdirAll(pluginDir, 0o755)

	plugin := map[string]any{
		"name": "test-plugin",
		"mcpServers": map[string]any{
			"test-plugin": map[string]any{
				"type":    "stdio",
				"command": "test-plugin",
				"args":    []any{"mcp", "stdio"},
			},
		},
	}
	data, _ := json.MarshalIndent(plugin, "", "  ")
	os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0o644)

	// Create settings directory
	settingsDir := filepath.Join(root, ".claude")
	os.MkdirAll(settingsDir, 0o755)
	settingsPath := filepath.Join(settingsDir, "settings.json")

	count, err := installMCPServers(root, settingsPath)
	if err != nil {
		t.Fatalf("installMCPServers: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 server installed, got %d", count)
	}

	// Verify settings.json
	settingsData, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(settingsData, &settings); err != nil {
		t.Fatalf("parsing settings.json: %v", err)
	}

	mcpServers, ok := settings["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers not found in settings.json")
	}

	server, ok := mcpServers["test-plugin"].(map[string]any)
	if !ok {
		t.Fatal("test-plugin server not found")
	}

	if server["command"] != "go" {
		t.Errorf("command = %q, want \"go\"", server["command"])
	}

	args, _ := server["args"].([]any)
	if len(args) < 2 || args[0] != "run" || args[1] != "./cmd/test-plugin" {
		t.Errorf("args = %v, want [run ./cmd/test-plugin mcp stdio]", args)
	}
}

func TestInstallMCPServersNoServers(t *testing.T) {
	root := t.TempDir()

	pluginDir := filepath.Join(root, ".claude-plugin")
	os.MkdirAll(pluginDir, 0o755)

	plugin := map[string]any{"name": "skill-only"}
	data, _ := json.MarshalIndent(plugin, "", "  ")
	os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0o644)

	settingsPath := filepath.Join(root, ".claude", "settings.json")

	count, err := installMCPServers(root, settingsPath)
	if err != nil {
		t.Fatalf("installMCPServers: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 servers, got %d", count)
	}
}

func TestInstallMCPServersPreservesExistingSettings(t *testing.T) {
	root := t.TempDir()

	// Create plugin.json with an MCP server
	pluginDir := filepath.Join(root, ".claude-plugin")
	os.MkdirAll(pluginDir, 0o755)

	plugin := map[string]any{
		"name": "my-mcp",
		"mcpServers": map[string]any{
			"my-mcp": map[string]any{
				"type":    "stdio",
				"command": "my-mcp",
				"args":    []any{"serve"},
			},
		},
	}
	data, _ := json.MarshalIndent(plugin, "", "  ")
	os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0o644)

	// Create existing settings.json with other content
	settingsDir := filepath.Join(root, ".claude")
	os.MkdirAll(settingsDir, 0o755)
	settingsPath := filepath.Join(settingsDir, "settings.json")

	existing := map[string]any{
		"permissions": map[string]any{"allow": []string{"Read"}},
		"mcpServers": map[string]any{
			"other-server": map[string]any{
				"command": "other",
				"args":    []any{},
			},
		},
	}
	existingData, _ := json.MarshalIndent(existing, "", "  ")
	os.WriteFile(settingsPath, existingData, 0o644)

	count, err := installMCPServers(root, settingsPath)
	if err != nil {
		t.Fatalf("installMCPServers: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 server, got %d", count)
	}

	settingsData, _ := os.ReadFile(settingsPath)
	var settings map[string]any
	json.Unmarshal(settingsData, &settings)

	// Check existing server preserved
	mcpServers := settings["mcpServers"].(map[string]any)
	if _, ok := mcpServers["other-server"]; !ok {
		t.Error("existing other-server was removed")
	}

	// Check permissions preserved
	if _, ok := settings["permissions"]; !ok {
		t.Error("existing permissions were removed")
	}
}
