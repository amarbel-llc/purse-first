package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/amarbel-llc/purse-first/libs/dewey/internal/alfa/test_ui"
)

func TestInstallMCPCreatesNewConfig(t *testing.T) {
	tt := test_ui.T{T: t}
	app := NewUtility("chrest", "Chrome REST client")
	app.MCPArgs = []string{"mcp"}

	configPath := filepath.Join(t.TempDir(), ".claude.json")

	if err := app.installMCPTo("/nix/store/abc-chrest/bin/chrest", configPath); err != nil {
		t.Fatalf("installMCPTo: %v", err)
	}

	config := readTestConfig(tt, configPath)
	server := getTestServer(tt, config, "chrest")

	assertField(tt, server, "type", "stdio")
	assertField(tt, server, "command", "/nix/store/abc-chrest/bin/chrest")
	assertArgs(tt, server, []string{"mcp"})
}

func TestInstallMCPPreservesExistingEntries(t *testing.T) {
	tt := test_ui.T{T: t}
	configPath := filepath.Join(t.TempDir(), ".claude.json")

	existing := map[string]any{
		"mcpServers": map[string]any{
			"other-server": map[string]any{
				"type":    "stdio",
				"command": "/usr/bin/other",
				"args":    []any{},
			},
		},
	}
	writeTestConfig(tt, configPath, existing)

	app := NewUtility("grit", "Git MCP server")

	if err := app.installMCPTo("/nix/store/xyz-grit/bin/grit", configPath); err != nil {
		t.Fatalf("installMCPTo: %v", err)
	}

	config := readTestConfig(tt, configPath)

	// Original entry preserved
	_ = getTestServer(tt, config, "other-server")

	// New entry added
	server := getTestServer(tt, config, "grit")
	assertField(tt, server, "command", "/nix/store/xyz-grit/bin/grit")
}

func TestInstallMCPUpdatesExistingEntry(t *testing.T) {
	tt := test_ui.T{T: t}
	configPath := filepath.Join(t.TempDir(), ".claude.json")

	existing := map[string]any{
		"mcpServers": map[string]any{
			"chrest": map[string]any{
				"type":    "stdio",
				"command": "/nix/store/old-chrest/bin/chrest",
				"args":    []any{"mcp"},
			},
		},
	}
	writeTestConfig(tt, configPath, existing)

	app := NewUtility("chrest", "Chrome REST client")
	app.MCPArgs = []string{"mcp"}

	if err := app.installMCPTo("/nix/store/new-chrest/bin/chrest", configPath); err != nil {
		t.Fatalf("installMCPTo: %v", err)
	}

	config := readTestConfig(tt, configPath)
	server := getTestServer(tt, config, "chrest")
	assertField(tt, server, "command", "/nix/store/new-chrest/bin/chrest")
}

func TestInstallMCPWithEmptyArgs(t *testing.T) {
	tt := test_ui.T{T: t}
	app := NewUtility("simple", "Simple server")
	// MCPArgs not set — should use empty slice

	configPath := filepath.Join(t.TempDir(), ".claude.json")

	if err := app.installMCPTo("/usr/bin/simple", configPath); err != nil {
		t.Fatalf("installMCPTo: %v", err)
	}

	config := readTestConfig(tt, configPath)
	server := getTestServer(tt, config, "simple")
	assertArgs(tt, server, []string{})
}

func TestInstallMCPPreservesNonServerFields(t *testing.T) {
	tt := test_ui.T{T: t}
	configPath := filepath.Join(t.TempDir(), ".claude.json")

	existing := map[string]any{
		"someOtherField": "value",
		"mcpServers":     map[string]any{},
	}
	writeTestConfig(tt, configPath, existing)

	app := NewUtility("test", "Test server")
	if err := app.installMCPTo("/usr/bin/test", configPath); err != nil {
		t.Fatalf("installMCPTo: %v", err)
	}

	config := readTestConfig(tt, configPath)
	if config["someOtherField"] != "value" {
		t.Errorf("non-server field lost: got %v", config["someOtherField"])
	}
}

// Test helpers

func readTestConfig(t test_ui.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("parsing config: %v", err)
	}
	return config
}

func writeTestConfig(t test_ui.T, path string, config map[string]any) {
	t.Helper()
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("marshaling config: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
}

func getTestServer(t test_ui.T, config map[string]any, name string) map[string]any {
	t.Helper()
	servers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers not found or wrong type")
	}
	server, ok := servers[name].(map[string]any)
	if !ok {
		t.Fatalf("server %q not found", name)
	}
	return server
}

func assertField(t test_ui.T, server map[string]any, key, want string) {
	t.Helper()
	got, _ := server[key].(string)
	if got != want {
		t.Errorf("%s = %q, want %q", key, got, want)
	}
}

func assertArgs(t test_ui.T, server map[string]any, want []string) {
	t.Helper()
	raw, ok := server["args"].([]any)
	if !ok {
		t.Fatalf("args not found or wrong type: %T", server["args"])
	}
	if len(raw) != len(want) {
		t.Fatalf("args length = %d, want %d", len(raw), len(want))
	}
	for i, v := range raw {
		s, _ := v.(string)
		if s != want[i] {
			t.Errorf("args[%d] = %q, want %q", i, s, want[i])
		}
	}
}
