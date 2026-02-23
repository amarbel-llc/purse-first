package localplugin

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallLocalSkillsAndMCP(t *testing.T) {
	root := t.TempDir()

	// Create a skill
	skillDir := filepath.Join(root, "skills", "my-skill")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# skill"), 0o644)

	// Create plugin.json (no MCP servers)
	pluginDir := filepath.Join(root, ".claude-plugin")
	os.MkdirAll(pluginDir, 0o755)
	plugin := map[string]any{"name": "test-pkg"}
	data, _ := json.MarshalIndent(plugin, "", "  ")
	os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0o644)

	var buf bytes.Buffer
	err := InstallLocal(&buf, root, InstallLocalOptions{})
	if err != nil {
		t.Fatalf("InstallLocal: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "TAP version 14") {
		t.Error("missing TAP version header")
	}
	if !strings.Contains(output, "1..2") {
		t.Error("missing test plan")
	}
	if !strings.Contains(output, "ok 1") {
		t.Error("missing ok 1 for skills")
	}
	// MCP should be skipped (no servers)
	if !strings.Contains(output, "ok 2") {
		t.Error("missing ok 2 for MCP")
	}
	if !strings.Contains(output, "# SKIP") {
		t.Error("MCP step should be SKIP when no servers declared")
	}

	// Verify plugin.json was updated with skills
	pluginData, _ := os.ReadFile(filepath.Join(pluginDir, "plugin.json"))
	var got map[string]any
	json.Unmarshal(pluginData, &got)
	skills, _ := got["skills"].([]any)
	if len(skills) != 1 {
		t.Errorf("expected 1 skill in plugin.json, got %d", len(skills))
	}
}

func TestInstallLocalWithMCPServers(t *testing.T) {
	root := t.TempDir()

	// Create plugin.json with MCP server
	pluginDir := filepath.Join(root, ".claude-plugin")
	os.MkdirAll(pluginDir, 0o755)
	plugin := map[string]any{
		"name": "my-mcp",
		"mcpServers": map[string]any{
			"my-mcp": map[string]any{
				"type":    "stdio",
				"command": "my-mcp",
				"args":    []any{"mcp", "stdio"},
			},
		},
	}
	data, _ := json.MarshalIndent(plugin, "", "  ")
	os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0o644)

	var buf bytes.Buffer
	err := InstallLocal(&buf, root, InstallLocalOptions{})
	if err != nil {
		t.Fatalf("InstallLocal: %v", err)
	}

	output := buf.String()

	// MCP step should NOT be skipped
	if strings.Contains(output, "# SKIP") {
		t.Error("MCP step should not be SKIP when servers are declared")
	}
	if !strings.Contains(output, "1 server") {
		t.Error("expected '1 server' in MCP step description")
	}

	// Verify .claude/settings.json was created with MCP entry
	settingsData, err := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}

	var settings map[string]any
	json.Unmarshal(settingsData, &settings)

	mcpServers, ok := settings["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers not in settings.json")
	}

	if _, ok := mcpServers["my-mcp"]; !ok {
		t.Error("my-mcp not found in settings.json mcpServers")
	}
}

func TestInstallLocalWithBinaryPreCreated(t *testing.T) {
	root := t.TempDir()

	// Pre-create the _generate output structure (simulating what _generate produces)
	pluginDir := filepath.Join(root, ".claude-plugin", "share", "purse-first", "test-mcp")
	os.MkdirAll(pluginDir, 0o755)
	plugin := map[string]any{
		"name": "test-mcp",
		"mcpServers": map[string]any{
			"test-mcp": map[string]any{
				"type":    "stdio",
				"command": "test-mcp",
				"args":    []any{"mcp", "stdio"},
			},
		},
	}
	data, _ := json.MarshalIndent(plugin, "", "  ")
	os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0o644)

	// Test that findGeneratedPlugin discovers the right path
	generatedPath, err := findGeneratedPlugin(filepath.Join(root, ".claude-plugin"))
	if err != nil {
		t.Fatalf("findGeneratedPlugin: %v", err)
	}

	if filepath.Base(filepath.Dir(generatedPath)) != "test-mcp" {
		t.Errorf("expected test-mcp dir, got %s", filepath.Dir(generatedPath))
	}

	// Test that installMCPServers works with the generated path
	settingsPath := filepath.Join(root, ".claude", "settings.json")
	count, err := installMCPServers(generatedPath, settingsPath)
	if err != nil {
		t.Fatalf("installMCPServers: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 server, got %d", count)
	}

	// Verify settings.json
	settingsData, _ := os.ReadFile(settingsPath)
	var settings map[string]any
	json.Unmarshal(settingsData, &settings)

	mcpServers, _ := settings["mcpServers"].(map[string]any)
	if _, ok := mcpServers["test-mcp"]; !ok {
		t.Error("test-mcp not found in settings.json")
	}
}
