package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallMCPCreatesNewConfig(t *testing.T) {
	app := NewUtility("chrest", "Chrome REST client")
	app.MCPArgs = []string{"mcp"}

	configPath := filepath.Join(t.TempDir(), ".claude.json")

	if err := app.installMCPTo("/nix/store/abc-chrest/bin/chrest", configPath); err != nil {
		t.Fatalf("installMCPTo: %v", err)
	}

	config := readTestConfig(t, configPath)
	server := getTestServer(t, config, "chrest")

	assertField(t, server, "type", "stdio")
	assertField(t, server, "command", "/nix/store/abc-chrest/bin/chrest")
	assertArgs(t, server, []string{"mcp"})
}

func TestInstallMCPPreservesExistingEntries(t *testing.T) {
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
	writeTestConfig(t, configPath, existing)

	app := NewUtility("grit", "Git MCP server")

	if err := app.installMCPTo("/nix/store/xyz-grit/bin/grit", configPath); err != nil {
		t.Fatalf("installMCPTo: %v", err)
	}

	config := readTestConfig(t, configPath)

	// Original entry preserved
	_ = getTestServer(t, config, "other-server")

	// New entry added
	server := getTestServer(t, config, "grit")
	assertField(t, server, "command", "/nix/store/xyz-grit/bin/grit")
}

func TestInstallMCPUpdatesExistingEntry(t *testing.T) {
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
	writeTestConfig(t, configPath, existing)

	app := NewUtility("chrest", "Chrome REST client")
	app.MCPArgs = []string{"mcp"}

	if err := app.installMCPTo("/nix/store/new-chrest/bin/chrest", configPath); err != nil {
		t.Fatalf("installMCPTo: %v", err)
	}

	config := readTestConfig(t, configPath)
	server := getTestServer(t, config, "chrest")
	assertField(t, server, "command", "/nix/store/new-chrest/bin/chrest")
}

func TestInstallMCPWithEmptyArgs(t *testing.T) {
	app := NewUtility("simple", "Simple server")
	// MCPArgs not set — should use empty slice

	configPath := filepath.Join(t.TempDir(), ".claude.json")

	if err := app.installMCPTo("/usr/bin/simple", configPath); err != nil {
		t.Fatalf("installMCPTo: %v", err)
	}

	config := readTestConfig(t, configPath)
	server := getTestServer(t, config, "simple")
	assertArgs(t, server, []string{})
}

func TestInstallMCPPreservesNonServerFields(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), ".claude.json")

	existing := map[string]any{
		"someOtherField": "value",
		"mcpServers":     map[string]any{},
	}
	writeTestConfig(t, configPath, existing)

	app := NewUtility("test", "Test server")
	if err := app.installMCPTo("/usr/bin/test", configPath); err != nil {
		t.Fatalf("installMCPTo: %v", err)
	}

	config := readTestConfig(t, configPath)
	if config["someOtherField"] != "value" {
		t.Errorf("non-server field lost: got %v", config["someOtherField"])
	}
}

// Test helpers

func readTestConfig(t *testing.T, path string) map[string]any {
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

func writeTestConfig(t *testing.T, path string, config map[string]any) {
	t.Helper()
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("marshaling config: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
}

func getTestServer(t *testing.T, config map[string]any, name string) map[string]any {
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

func assertField(t *testing.T, server map[string]any, key, want string) {
	t.Helper()
	got, _ := server[key].(string)
	if got != want {
		t.Errorf("%s = %q, want %q", key, got, want)
	}
}

func assertArgs(t *testing.T, server map[string]any, want []string) {
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
