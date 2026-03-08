package localplugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFindGeneratedPlugin(t *testing.T) {
	outDir := t.TempDir()

	// Simulate _generate output structure
	pluginDir := filepath.Join(outDir, "share", "purse-first", "lux", ".claude-plugin")
	os.MkdirAll(pluginDir, 0o755)
	plugin := map[string]any{
		"name": "lux",
		"mcpServers": map[string]any{
			"lux": map[string]any{
				"type":    "stdio",
				"command": "lux",
				"args":    []any{"mcp-stdio"},
			},
		},
	}
	data, _ := json.MarshalIndent(plugin, "", "  ")
	os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0o644)

	got, err := findGeneratedPlugin(outDir)
	if err != nil {
		t.Fatalf("findGeneratedPlugin: %v", err)
	}

	expected := filepath.Join(pluginDir, "plugin.json")
	if got != expected {
		t.Errorf("got %s, want %s", got, expected)
	}
}

func TestFindGeneratedPluginNoMatch(t *testing.T) {
	outDir := t.TempDir()

	_, err := findGeneratedPlugin(outDir)
	if err == nil {
		t.Error("expected error for empty dir")
	}
}
