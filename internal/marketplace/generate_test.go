package marketplace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerate(t *testing.T) {
	config := Config{
		Name:        "test-marketplace",
		Description: "A test marketplace for unit tests",
		Owner:       Owner{Name: "test", Email: "test@example.com"},
		Plugins: map[string]PluginMeta{
			"alpha": {
				Description: "Alpha MCP server for testing",
				Version:     "1.0.0",
				Homepage:    "https://example.com/alpha",
				Category:    "development",
				Tags:        []string{"test", "alpha"},
			},
		},
	}

	discovered := []DiscoveredPlugin{
		{Name: "alpha", Type: "stdio", Command: "alpha-server", Args: []string{"serve"}},
		{Name: "beta", Type: "stdio", Command: "beta-server"},
	}

	m := Generate(config, discovered)

	if m.Schema != SchemaURL {
		t.Errorf("schema = %q, want %q", m.Schema, SchemaURL)
	}
	if m.Name != "test-marketplace" {
		t.Errorf("name = %q, want %q", m.Name, "test-marketplace")
	}
	if len(m.Plugins) != 2 {
		t.Fatalf("len(plugins) = %d, want 2", len(m.Plugins))
	}

	// Plugins should be sorted by name
	if m.Plugins[0].Name != "alpha" {
		t.Errorf("plugins[0].name = %q, want %q", m.Plugins[0].Name, "alpha")
	}
	if m.Plugins[1].Name != "beta" {
		t.Errorf("plugins[1].name = %q, want %q", m.Plugins[1].Name, "beta")
	}

	// Alpha should have config metadata
	alpha := m.Plugins[0]
	if alpha.Description != "Alpha MCP server for testing" {
		t.Errorf("alpha.description = %q", alpha.Description)
	}
	if alpha.Version != "1.0.0" {
		t.Errorf("alpha.version = %q", alpha.Version)
	}
	if alpha.Homepage != "https://example.com/alpha" {
		t.Errorf("alpha.homepage = %q", alpha.Homepage)
	}
	if alpha.Source != "./bin/alpha" {
		t.Errorf("alpha.source = %q, want %q", alpha.Source, "./bin/alpha")
	}
	if alpha.Strict == nil || *alpha.Strict != false {
		t.Error("alpha.strict should be false")
	}

	mcpServers := alpha.McpServers
	srv, ok := mcpServers["alpha"].(map[string]any)
	if !ok {
		t.Fatal("alpha.mcpServers.alpha missing")
	}
	if srv["command"] != "alpha-server" {
		t.Errorf("mcpServers.alpha.command = %v", srv["command"])
	}
	if args, ok := srv["args"].([]string); !ok || len(args) != 1 || args[0] != "serve" {
		t.Errorf("mcpServers.alpha.args = %v", srv["args"])
	}

	// Beta should have defaults
	beta := m.Plugins[1]
	if beta.Description != "MCP server: beta" {
		t.Errorf("beta.description = %q, want default", beta.Description)
	}
	if beta.Version != "0.1.0" {
		t.Errorf("beta.version = %q, want default", beta.Version)
	}
}

func TestDiscoverPlugins(t *testing.T) {
	dir := t.TempDir()

	alphaDir := filepath.Join(dir, "alpha")
	os.MkdirAll(alphaDir, 0o755)

	plugin := map[string]any{
		"name":    "alpha",
		"type":    "stdio",
		"command": "alpha-cmd",
		"args":    []string{"--flag"},
	}
	data, _ := json.MarshalIndent(plugin, "", "  ")
	os.WriteFile(filepath.Join(alphaDir, "plugin.json"), data, 0o644)

	plugins, err := DiscoverPlugins(dir)
	if err != nil {
		t.Fatalf("DiscoverPlugins: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("len(plugins) = %d, want 1", len(plugins))
	}

	if plugins[0].Name != "alpha" {
		t.Errorf("name = %q", plugins[0].Name)
	}
	if plugins[0].Command != "alpha-cmd" {
		t.Errorf("command = %q", plugins[0].Command)
	}
}

func TestReadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := Config{
		Name:        "my-marketplace",
		Description: "Test marketplace config for validation",
		Owner:       Owner{Name: "owner", Email: "owner@example.com"},
		Plugins: map[string]PluginMeta{
			"tool": {Description: "A tool for testing purposes", Version: "2.0.0"},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0o644)

	got, err := ReadConfig(path)
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}

	if got.Name != "my-marketplace" {
		t.Errorf("name = %q", got.Name)
	}
	if got.Plugins["tool"].Version != "2.0.0" {
		t.Errorf("plugins.tool.version = %q", got.Plugins["tool"].Version)
	}
}

func TestWriteAndRead(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, ".claude-plugin", "marketplace.json")

	strict := false
	m := Marketplace{
		Schema:      SchemaURL,
		Name:        "roundtrip",
		Description: "Roundtrip test marketplace for write and read",
		Owner:       Owner{Name: "test", Email: "test@example.com"},
		Plugins: []Plugin{
			{
				Name:        "tool",
				Description: "Roundtrip test tool for verification",
				Version:     "1.0.0",
				Source:      "./tool",
				Strict:      &strict,
				McpServers:  map[string]any{"tool": map[string]any{"type": "stdio", "command": "tool"}},
			},
		},
	}

	if err := Write(m, outputPath); err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var got Marketplace
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Name != "roundtrip" {
		t.Errorf("name = %q", got.Name)
	}
	if len(got.Plugins) != 1 {
		t.Errorf("len(plugins) = %d", len(got.Plugins))
	}
}
