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
		Repo:        "example/test-marketplace",
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

	m := Generate(config, discovered, nil)

	if m.Name != "test-marketplace" {
		t.Errorf("name = %q, want %q", m.Name, "test-marketplace")
	}
	if m.Metadata == nil || m.Metadata.Description != "A test marketplace for unit tests" {
		t.Errorf("metadata.description = %v, want %q", m.Metadata, "A test marketplace for unit tests")
	}
	if len(m.Plugins) != 1 {
		t.Fatalf("len(plugins) = %d, want 1", len(m.Plugins))
	}

	plugin := m.Plugins[0]
	if plugin.Name != "test-marketplace" {
		t.Errorf("plugin.name = %q, want %q", plugin.Name, "test-marketplace")
	}
	if plugin.Description != "A test marketplace for unit tests" {
		t.Errorf("plugin.description = %q", plugin.Description)
	}

	source, ok := plugin.Source.(GitHubSource)
	if !ok {
		t.Fatalf("plugin.source type = %T, want GitHubSource", plugin.Source)
	}
	if source.Repo != "example/test-marketplace" {
		t.Errorf("plugin.source.repo = %q, want %q", source.Repo, "example/test-marketplace")
	}

	if plugin.Strict == nil || *plugin.Strict != false {
		t.Error("plugin.strict should be false")
	}

	if len(plugin.McpServers) != 2 {
		t.Fatalf("len(mcpServers) = %d, want 2", len(plugin.McpServers))
	}

	alphaSrv, ok := plugin.McpServers["alpha"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers.alpha missing")
	}
	if alphaSrv["command"] != "alpha-server" {
		t.Errorf("mcpServers.alpha.command = %v", alphaSrv["command"])
	}
	if args, ok := alphaSrv["args"].([]string); !ok || len(args) != 1 || args[0] != "serve" {
		t.Errorf("mcpServers.alpha.args = %v", alphaSrv["args"])
	}

	betaSrv, ok := plugin.McpServers["beta"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers.beta missing")
	}
	if betaSrv["command"] != "beta-server" {
		t.Errorf("mcpServers.beta.command = %v", betaSrv["command"])
	}
	if _, hasArgs := betaSrv["args"]; hasArgs {
		t.Errorf("mcpServers.beta should not have args")
	}
}

func TestGenerateNoRepo(t *testing.T) {
	config := Config{
		Name:    "test-marketplace",
		Owner:   Owner{Name: "test"},
		Plugins: map[string]PluginMeta{},
	}

	discovered := []DiscoveredPlugin{
		{Name: "alpha", Type: "stdio", Command: "alpha-server"},
	}

	m := Generate(config, discovered, nil)

	if len(m.Plugins) != 1 {
		t.Fatalf("len(plugins) = %d, want 1", len(m.Plugins))
	}

	source, ok := m.Plugins[0].Source.(string)
	if !ok {
		t.Fatalf("source type = %T, want string", m.Plugins[0].Source)
	}
	if source != "." {
		t.Errorf("source = %q, want %q", source, ".")
	}
}

func TestGenerateEmpty(t *testing.T) {
	config := Config{
		Name:    "test-marketplace",
		Owner:   Owner{Name: "test"},
		Plugins: map[string]PluginMeta{},
	}

	m := Generate(config, nil, nil)

	if len(m.Plugins) != 0 {
		t.Errorf("len(plugins) = %d, want 0", len(m.Plugins))
	}
}

func TestDiscoverPlugins(t *testing.T) {
	dir := t.TempDir()

	alphaDir := filepath.Join(dir, "alpha")
	os.MkdirAll(alphaDir, 0o755)

	plugin := map[string]any{
		"name": "alpha",
		"mcpServers": map[string]any{
			"alpha": map[string]any{
				"type":    "stdio",
				"command": "alpha-cmd",
				"args":    []string{"--flag"},
			},
		},
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
		Repo:        "example/my-marketplace",
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
	if got.Repo != "example/my-marketplace" {
		t.Errorf("repo = %q", got.Repo)
	}
	if got.Plugins["tool"].Version != "2.0.0" {
		t.Errorf("plugins.tool.version = %q", got.Plugins["tool"].Version)
	}
}

func TestGenerateWithSkillsOnly(t *testing.T) {
	config := Config{
		Name:        "test-marketplace",
		Description: "A marketplace with skills only",
		Repo:        "example/test-marketplace",
		Owner:       Owner{Name: "test"},
		Plugins:     map[string]PluginMeta{},
	}

	skills := []DiscoveredSkill{
		{Name: "plugin-mcp", Path: "skills/plugin-mcp/SKILL.md"},
	}

	m := Generate(config, nil, skills)

	if len(m.Plugins) != 1 {
		t.Fatalf("len(plugins) = %d, want 1", len(m.Plugins))
	}

	plugin := m.Plugins[0]
	if plugin.Name != "test-marketplace" {
		t.Errorf("plugin.name = %q, want %q", plugin.Name, "test-marketplace")
	}

	if len(plugin.Skills) != 1 {
		t.Fatalf("len(skills) = %d, want 1", len(plugin.Skills))
	}

	skill, ok := plugin.Skills["plugin-mcp"].(map[string]any)
	if !ok {
		t.Fatal("skills.plugin-mcp missing")
	}
	if skill["path"] != "skills/plugin-mcp/SKILL.md" {
		t.Errorf("skills.plugin-mcp.path = %v", skill["path"])
	}

	if plugin.McpServers != nil {
		t.Errorf("plugin should not have mcpServers")
	}
}

func TestGenerateWithMcpAndSkills(t *testing.T) {
	config := Config{
		Name:        "test-marketplace",
		Description: "A marketplace with both MCP and skills",
		Repo:        "example/test-marketplace",
		Owner:       Owner{Name: "test"},
		Plugins:     map[string]PluginMeta{},
	}

	discovered := []DiscoveredPlugin{
		{Name: "alpha", Type: "stdio", Command: "alpha-server"},
	}

	skills := []DiscoveredSkill{
		{Name: "plugin-mcp", Path: "skills/plugin-mcp/SKILL.md"},
	}

	m := Generate(config, discovered, skills)

	if len(m.Plugins) != 1 {
		t.Fatalf("len(plugins) = %d, want 1 (skills merged into MCP plugin)", len(m.Plugins))
	}

	plugin := m.Plugins[0]
	if len(plugin.McpServers) != 1 {
		t.Errorf("len(mcpServers) = %d, want 1", len(plugin.McpServers))
	}
	if len(plugin.Skills) != 1 {
		t.Errorf("len(skills) = %d, want 1", len(plugin.Skills))
	}
}

func TestDiscoverSkills(t *testing.T) {
	dir := t.TempDir()

	skillDir := filepath.Join(dir, "my-skill")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# My Skill"), 0o644)

	skills, err := DiscoverSkills(dir)
	if err != nil {
		t.Fatalf("DiscoverSkills: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("len(skills) = %d, want 1", len(skills))
	}

	if skills[0].Name != "my-skill" {
		t.Errorf("name = %q, want %q", skills[0].Name, "my-skill")
	}
	if skills[0].Path != "skills/my-skill/SKILL.md" {
		t.Errorf("path = %q, want %q", skills[0].Path, "skills/my-skill/SKILL.md")
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
