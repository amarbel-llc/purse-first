package marketplace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/amarbel-llc/purse-first/purse"
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
		{
			Name: "alpha",
			McpServers: map[string]purse.McpServer{
				"alpha": {Type: "stdio", Command: "alpha-server", Args: []string{"serve"}},
				"beta":  {Type: "stdio", Command: "beta-server"},
			},
			StorePath: "/nix/store/abc123-alpha-1.0.0",
		},
	}

	m := Generate(config, discovered)

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
	if plugin.Name != "alpha" {
		t.Errorf("plugin.name = %q, want %q", plugin.Name, "alpha")
	}
	if plugin.Description != "Alpha MCP server for testing" {
		t.Errorf("plugin.description = %q, want %q", plugin.Description, "Alpha MCP server for testing")
	}
	if plugin.Version != "1.0.0" {
		t.Errorf("plugin.version = %q, want %q", plugin.Version, "1.0.0")
	}
	if plugin.Homepage != "https://github.com/example/alpha" {
		t.Errorf("plugin.homepage = %q", plugin.Homepage)
	}
	if plugin.Category != "development" {
		t.Errorf("plugin.category = %q", plugin.Category)
	}
	if len(plugin.Tags) != 2 || plugin.Tags[0] != "test" {
		t.Errorf("plugin.tags = %v", plugin.Tags)
	}

	source, ok := plugin.Source.(GitHubSource)
	if !ok {
		t.Fatalf("plugin.source type = %T, want GitHubSource", plugin.Source)
	}
	if source.Repo != "example/alpha" {
		t.Errorf("plugin.source.repo = %q, want %q", source.Repo, "example/alpha")
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
		{
			Name: "alpha",
			McpServers: map[string]purse.McpServer{
				"alpha": {Type: "stdio", Command: "alpha-server"},
			},
		},
	}

	m := Generate(config, discovered)

	if len(m.Plugins) != 1 {
		t.Fatalf("len(plugins) = %d, want 1", len(m.Plugins))
	}

	if m.Plugins[0].Name != "alpha" {
		t.Errorf("plugin.name = %q, want %q", m.Plugins[0].Name, "alpha")
	}

	source, ok := m.Plugins[0].Source.(string)
	if !ok {
		t.Fatalf("source type = %T, want string", m.Plugins[0].Source)
	}
	if source != "." {
		t.Errorf("source = %q, want %q", source, ".")
	}
}

func TestGenerateNoRepoFallsBackToConfig(t *testing.T) {
	config := Config{
		Name:  "test-marketplace",
		Repo:  "example/fallback",
		Owner: Owner{Name: "test"},
		Plugins: map[string]PluginMeta{
			"alpha": {Description: "no repo on meta"},
		},
	}

	discovered := []DiscoveredPlugin{
		{
			Name: "alpha",
			McpServers: map[string]purse.McpServer{
				"alpha": {Type: "stdio", Command: "alpha-server"},
			},
		},
	}

	m := Generate(config, discovered)

	if len(m.Plugins) != 1 {
		t.Fatalf("len(plugins) = %d, want 1", len(m.Plugins))
	}

	source, ok := m.Plugins[0].Source.(GitHubSource)
	if !ok {
		t.Fatalf("source type = %T, want GitHubSource", m.Plugins[0].Source)
	}
	if source.Repo != "example/fallback" {
		t.Errorf("source.repo = %q, want %q", source.Repo, "example/fallback")
	}
}

func TestGenerateEmpty(t *testing.T) {
	config := Config{
		Name:    "test-marketplace",
		Owner:   Owner{Name: "test"},
		Plugins: map[string]PluginMeta{},
	}

	m := Generate(config, nil)

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
	if len(plugins[0].McpServers) != 1 {
		t.Errorf("len(McpServers) = %d, want 1", len(plugins[0].McpServers))
	}
	srv, ok := plugins[0].McpServers["alpha"]
	if !ok {
		t.Fatal("McpServers[alpha] missing")
	}
	if srv.Command != "alpha-cmd" {
		t.Errorf("command = %q", srv.Command)
	}
}

func TestDiscoverPluginsWithSkills(t *testing.T) {
	dir := t.TempDir()

	alphaDir := filepath.Join(dir, "alpha")
	skillDir := filepath.Join(alphaDir, "skills", "my-skill")
	os.MkdirAll(skillDir, 0o755)

	plugin := map[string]any{
		"name": "alpha",
		"mcpServers": map[string]any{
			"alpha": map[string]any{
				"type":    "stdio",
				"command": "alpha-cmd",
			},
		},
	}
	data, _ := json.MarshalIndent(plugin, "", "  ")
	os.WriteFile(filepath.Join(alphaDir, "plugin.json"), data, 0o644)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# My Skill"), 0o644)

	plugins, err := DiscoverPlugins(dir)
	if err != nil {
		t.Fatalf("DiscoverPlugins: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("len(plugins) = %d, want 1", len(plugins))
	}

	if len(plugins[0].Skills) != 1 {
		t.Fatalf("len(Skills) = %d, want 1", len(plugins[0].Skills))
	}
	if plugins[0].Skills[0].Name != "my-skill" {
		t.Errorf("skill name = %q, want %q", plugins[0].Skills[0].Name, "my-skill")
	}
	expectedPath := "./skills/my-skill"
	if plugins[0].Skills[0].Path != expectedPath {
		t.Errorf("skill path = %q, want %q", plugins[0].Skills[0].Path, expectedPath)
	}
}

func TestGenerateWithSkillsOnly(t *testing.T) {
	config := Config{
		Name:        "test-marketplace",
		Description: "A marketplace with skills only",
		Repo:        "example/test-marketplace",
		Owner:       Owner{Name: "test"},
		Plugins: map[string]PluginMeta{
			"purse-first": {
				Description: "Plugin integration skill",
				Version:     "0.1.0",
				Repo:        "example/purse-first",
			},
		},
	}

	discovered := []DiscoveredPlugin{
		{
			Name: "purse-first",
			Skills: []DiscoveredSkill{
				{Name: "plugin-mcp", Path: "./skills/plugin-mcp"},
			},
		},
	}

	m := Generate(config, discovered)

	if len(m.Plugins) != 1 {
		t.Fatalf("len(plugins) = %d, want 1", len(m.Plugins))
	}

	plugin := m.Plugins[0]
	if plugin.Name != "purse-first" {
		t.Errorf("plugin.name = %q, want %q", plugin.Name, "purse-first")
	}

	source, ok := plugin.Source.(GitHubSource)
	if !ok {
		t.Fatalf("source type = %T, want GitHubSource", plugin.Source)
	}
	if source.Repo != "example/purse-first" {
		t.Errorf("source.repo = %q, want %q", source.Repo, "example/purse-first")
	}

	if len(plugin.Skills) != 1 {
		t.Fatalf("len(skills) = %d, want 1", len(plugin.Skills))
	}

	if plugin.Skills[0] != "./skills/plugin-mcp" {
		t.Errorf("skills[0] = %q, want %q", plugin.Skills[0], "./skills/plugin-mcp")
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
		Plugins: map[string]PluginMeta{
			"alpha": {
				Description: "Alpha server",
				Repo:        "example/alpha",
			},
			"purse-first": {
				Description: "Plugin skill",
				Repo:        "example/purse-first",
			},
		},
	}

	discovered := []DiscoveredPlugin{
		{
			Name: "alpha",
			McpServers: map[string]purse.McpServer{
				"alpha": {Type: "stdio", Command: "alpha-server"},
			},
		},
		{
			Name: "purse-first",
			Skills: []DiscoveredSkill{
				{Name: "plugin-mcp", Path: "./skills/plugin-mcp"},
			},
		},
	}

	m := Generate(config, discovered)

	if len(m.Plugins) != 2 {
		t.Fatalf("len(plugins) = %d, want 2 (one per discovered plugin)", len(m.Plugins))
	}

	alphaPlugin := m.Plugins[0]
	if alphaPlugin.Name != "alpha" {
		t.Errorf("plugins[0].name = %q, want %q", alphaPlugin.Name, "alpha")
	}
	if len(alphaPlugin.McpServers) != 1 {
		t.Errorf("plugins[0] len(mcpServers) = %d, want 1", len(alphaPlugin.McpServers))
	}
	if alphaPlugin.Skills != nil {
		t.Errorf("plugins[0] should not have skills")
	}

	pursePlugin := m.Plugins[1]
	if pursePlugin.Name != "purse-first" {
		t.Errorf("plugins[1].name = %q, want %q", pursePlugin.Name, "purse-first")
	}
	if len(pursePlugin.Skills) != 1 {
		t.Errorf("plugins[1] len(skills) = %d, want 1", len(pursePlugin.Skills))
	}
	if pursePlugin.McpServers != nil {
		t.Errorf("plugins[1] should not have mcpServers")
	}
}

func TestGenerateSelfPluginOmitsSkills(t *testing.T) {
	config := Config{
		Name:        "my-marketplace",
		Description: "Self-plugin should not emit skills",
		Repo:        "example/my-marketplace",
		Owner:       Owner{Name: "test"},
		Plugins: map[string]PluginMeta{
			"my-marketplace": {
				Description: "Self-referencing plugin with skills",
				Version:     "0.1.0",
				Repo:        "example/my-marketplace",
			},
		},
	}

	discovered := []DiscoveredPlugin{
		{
			Name: "my-marketplace",
			Skills: []DiscoveredSkill{
				{Name: "skill-a", Path: "./skills/skill-a"},
				{Name: "skill-b", Path: "./skills/skill-b"},
			},
		},
	}

	m := Generate(config, discovered)

	if len(m.Plugins) != 1 {
		t.Fatalf("len(plugins) = %d, want 1", len(m.Plugins))
	}

	plugin := m.Plugins[0]
	if plugin.Name != "my-marketplace" {
		t.Errorf("plugin.name = %q, want %q", plugin.Name, "my-marketplace")
	}
	if plugin.Skills != nil {
		t.Errorf("self-plugin should not have skills in marketplace entry, got %v", plugin.Skills)
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
