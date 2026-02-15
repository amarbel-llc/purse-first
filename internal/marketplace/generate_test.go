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
				Homepage:    "https://github.com/example/alpha",
				Repo:        "example/alpha",
				Category:    "development",
				Tags:        []string{"test", "alpha"},
			},
		},
	}

	discovered := []DiscoveredPlugin{
		{Name: "alpha", Type: "stdio", Command: "alpha-server", Args: []string{"serve"}, StorePath: "/nix/store/abc123-alpha-1.0.0"},
		{Name: "beta", Type: "stdio", Command: "beta-server"},
	}

	m := Generate(config, discovered)

	if m.Schema != "" {
		t.Errorf("schema = %q, want empty (omitted)", m.Schema)
	}
	if m.Name != "test-marketplace" {
		t.Errorf("name = %q, want %q", m.Name, "test-marketplace")
	}
	if m.Metadata == nil || m.Metadata.Description != "A test marketplace for unit tests" {
		t.Errorf("metadata.description = %v, want %q", m.Metadata, "A test marketplace for unit tests")
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
	if alpha.Homepage != "https://github.com/example/alpha" {
		t.Errorf("alpha.homepage = %q", alpha.Homepage)
	}

	// Alpha has a repo, so source should be a GitHubSource
	alphaSource, ok := alpha.Source.(GitHubSource)
	if !ok {
		t.Fatalf("alpha.source type = %T, want GitHubSource", alpha.Source)
	}
	if alphaSource.Source != "github" {
		t.Errorf("alpha.source.source = %q, want %q", alphaSource.Source, "github")
	}
	if alphaSource.Repo != "example/alpha" {
		t.Errorf("alpha.source.repo = %q, want %q", alphaSource.Repo, "example/alpha")
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

	// Beta should have no description/version (no defaults set)
	beta := m.Plugins[1]
	if beta.Description != "" {
		t.Errorf("beta.description = %q, want empty", beta.Description)
	}
	if beta.Version != "" {
		t.Errorf("beta.version = %q, want empty", beta.Version)
	}

	// Beta has no repo/homepage, so source should be a fallback string
	betaSource, ok := beta.Source.(string)
	if !ok {
		t.Fatalf("beta.source type = %T, want string", beta.Source)
	}
	if betaSource != "./beta" {
		t.Errorf("beta.source = %q, want %q", betaSource, "./beta")
	}
}

func TestRepoFromHomepage(t *testing.T) {
	tests := []struct {
		homepage string
		want     string
	}{
		{"https://github.com/amarbel-llc/grit", "amarbel-llc/grit"},
		{"https://github.com/friedenberg/lux/", "friedenberg/lux"},
		{"https://example.com/foo", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := repoFromHomepage(tt.homepage)
		if got != tt.want {
			t.Errorf("repoFromHomepage(%q) = %q, want %q", tt.homepage, got, tt.want)
		}
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
		Name:     "roundtrip",
		Metadata: &Metadata{Description: "Roundtrip test marketplace for write and read"},
		Owner:    Owner{Name: "test", Email: "test@example.com"},
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
	if got.Schema != "" {
		t.Errorf("schema = %q, want empty", got.Schema)
	}
	if len(got.Plugins) != 1 {
		t.Errorf("len(plugins) = %d", len(got.Plugins))
	}
}
