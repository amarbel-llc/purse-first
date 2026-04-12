package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDevMCPGeneratesArtifacts(t *testing.T) {
	// Simulate a nix build output directory
	buildDir := t.TempDir()

	// Create bin/
	binDir := filepath.Join(buildDir, "bin")
	os.MkdirAll(binDir, 0o755)
	os.WriteFile(filepath.Join(binDir, "lux"), []byte("#!/bin/sh\n"), 0o755)

	// Create share/purse-first/lux/.claude-plugin/plugin.json
	pluginDir := filepath.Join(buildDir, "share", "purse-first", "lux")
	claudePluginDir := filepath.Join(pluginDir, ".claude-plugin")
	os.MkdirAll(claudePluginDir, 0o755)

	plugin := pluginManifest{
		Name: "lux",
		McpServers: map[string]pluginMcpServer{
			"lux": {Type: "stdio", Command: "lux", Args: []string{"mcp", "stdio"}},
		},
	}
	pluginData, _ := json.MarshalIndent(plugin, "", "  ")
	os.WriteFile(filepath.Join(claudePluginDir, "plugin.json"), pluginData, 0o644)

	// Create share/purse-first/lux/mappings.json
	mappings := mappingFile{
		Server: "lux",
		Mappings: []mappingEntry{
			{
				Replaces:   "Read",
				Extensions: []string{".go"},
				Tools:      []mappingToolSuggestion{{Name: "hover", UseWhen: "getting type info"}},
				Reason:     "Use the lux MCP tool instead",
			},
		},
	}
	mappingsData, _ := json.MarshalIndent(mappings, "", "  ")
	os.WriteFile(filepath.Join(pluginDir, "mappings.json"), mappingsData, 0o644)

	// Run dev-mcp generation
	projectDir := t.TempDir()
	err := generateDevMCP(buildDir, projectDir, "dev")
	if err != nil {
		t.Fatalf("generateDevMCP: %v", err)
	}

	// Verify .mcp.json
	mcpData, err := os.ReadFile(filepath.Join(projectDir, ".mcp.json"))
	if err != nil {
		t.Fatalf("read .mcp.json: %v", err)
	}

	var mcpConfig struct {
		McpServers map[string]struct {
			Type    string   `json:"type"`
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(mcpData, &mcpConfig); err != nil {
		t.Fatalf("unmarshal .mcp.json: %v", err)
	}

	server, ok := mcpConfig.McpServers["lux-dev"]
	if !ok {
		t.Fatal("expected lux-dev server in .mcp.json")
	}
	if server.Type != "stdio" {
		t.Errorf("type = %q, want stdio", server.Type)
	}
	expectedBin := filepath.Join(buildDir, "bin", "lux")
	if server.Command != expectedBin {
		t.Errorf("command = %q, want %q", server.Command, expectedBin)
	}

	// Verify .purse-first/lux.json
	purseData, err := os.ReadFile(filepath.Join(projectDir, ".purse-first", "lux.json"))
	if err != nil {
		t.Fatalf("read .purse-first/lux.json: %v", err)
	}

	var purseMappings struct {
		Server     string `json:"server"`
		ToolPrefix string `json:"tool_prefix"`
	}
	if err := json.Unmarshal(purseData, &purseMappings); err != nil {
		t.Fatalf("unmarshal .purse-first/lux.json: %v", err)
	}

	if purseMappings.ToolPrefix != "mcp__lux-dev" {
		t.Errorf("tool_prefix = %q, want mcp__lux-dev", purseMappings.ToolPrefix)
	}
}

func TestAppHasDevMCPCommand(t *testing.T) {
	app := NewUtility("lux", "LSP multiplexer")

	cmd, ok := app.GetCommand("dev-mcp")
	if !ok {
		t.Fatal("dev-mcp command not registered")
	}
	if !cmd.Hidden {
		t.Error("dev-mcp should be hidden")
	}
}

func TestDevMCPClean(t *testing.T) {
	projectDir := t.TempDir()

	// Create the artifacts
	os.WriteFile(filepath.Join(projectDir, ".mcp.json"), []byte("{}"), 0o644)
	os.MkdirAll(filepath.Join(projectDir, ".purse-first"), 0o755)
	os.WriteFile(filepath.Join(projectDir, ".purse-first", "lux.json"), []byte("{}"), 0o644)

	err := cleanDevMCP(projectDir)
	if err != nil {
		t.Fatalf("cleanDevMCP: %v", err)
	}

	if _, err := os.Stat(filepath.Join(projectDir, ".mcp.json")); !os.IsNotExist(err) {
		t.Error(".mcp.json should be removed")
	}

	if _, err := os.Stat(filepath.Join(projectDir, ".purse-first")); !os.IsNotExist(err) {
		t.Error(".purse-first/ should be removed")
	}
}
