package command

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateAll(t *testing.T) {
	app := NewUtility("grit", "Git operations")
	app.Version = "0.1.0"

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		OldParams: []OldParam{
			{Name: "repo_path", Type: String, Description: "Path to repo", Required: true},
		},
		MapsTools: []ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git status"}, UseWhen: "checking status"},
		},
	})

	dir := t.TempDir()
	if err := app.GenerateAll(dir); err != nil {
		t.Fatalf("GenerateAll: %v", err)
	}

	expected := []string{
		filepath.Join("share", "purse-first", "grit", ".claude-plugin", "plugin.json"),
		filepath.Join("share", "purse-first", "grit", "mappings.json"),
		filepath.Join("share", "man", "man1", "grit.1"),
		filepath.Join("share", "man", "man1", "grit-status.1"),
		filepath.Join("share", "bash-completion", "completions", "grit"),
		filepath.Join("share", "zsh", "site-functions", "_grit"),
		filepath.Join("share", "fish", "vendor_completions.d", "grit.fish"),
	}

	for _, rel := range expected {
		path := filepath.Join(dir, rel)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file missing: %s", rel)
		}
	}
}

func TestGenerateAllWithSkills(t *testing.T) {
	app := NewUtility("chix", "Nix MCP server")
	app.Version = "0.1.0"
	app.PluginDescription = "Nix MCP server and skills"
	app.PluginAuthor = "friedenberg"

	app.AddCommand(&Command{
		Name:        "eval",
		Description: Description{Short: "Evaluate nix expression"},
		OldParams: []OldParam{
			{Name: "expr", Type: String, Description: "Nix expression", Required: true},
		},
	})

	// Create temp skills directory with two skill subdirs
	skillsDir := t.TempDir()
	for _, name := range []string{"nix-patterns", "flake-debugging"} {
		skillDir := filepath.Join(skillsDir, name)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
		content := "# " + name + "\n\nSkill content for " + name + ".\n"
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatalf("write SKILL.md for %s: %v", name, err)
		}
	}

	dir := t.TempDir()
	if err := app.GenerateAllWithSkills(dir, skillsDir); err != nil {
		t.Fatalf("GenerateAllWithSkills: %v", err)
	}

	// Assert plugin.json contains skills array with 2 entries in sorted order
	pluginPath := filepath.Join(dir, "share", "purse-first", "chix", ".claude-plugin", "plugin.json")
	data, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatalf("read plugin.json: %v", err)
	}

	var plugin map[string]any
	if err := json.Unmarshal(data, &plugin); err != nil {
		t.Fatalf("unmarshal plugin.json: %v", err)
	}

	skillsRaw, ok := plugin["skills"]
	if !ok {
		t.Fatal("plugin.json missing 'skills' key")
	}

	skills := skillsRaw.([]any)
	if len(skills) != 2 {
		t.Fatalf("skills length = %d, want 2", len(skills))
	}

	// Should be sorted alphabetically
	if skills[0] != "./skills/flake-debugging" {
		t.Errorf("skills[0] = %v, want ./skills/flake-debugging", skills[0])
	}
	if skills[1] != "./skills/nix-patterns" {
		t.Errorf("skills[1] = %v, want ./skills/nix-patterns", skills[1])
	}

	// Assert skills were physically copied
	for _, name := range []string{"flake-debugging", "nix-patterns"} {
		copiedSkill := filepath.Join(dir, "share", "purse-first", "chix", "skills", name, "SKILL.md")
		if _, err := os.Stat(copiedSkill); err != nil {
			t.Errorf("expected copied skill missing: skills/%s/SKILL.md", name)
		}
	}

	// Assert other standard artifacts still exist
	standardFiles := []string{
		filepath.Join("share", "purse-first", "chix", ".claude-plugin", "plugin.json"),
		filepath.Join("share", "man", "man1", "chix.1"),
		filepath.Join("share", "bash-completion", "completions", "chix"),
		filepath.Join("share", "zsh", "site-functions", "_chix"),
		filepath.Join("share", "fish", "vendor_completions.d", "chix.fish"),
	}
	for _, rel := range standardFiles {
		path := filepath.Join(dir, rel)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file missing: %s", rel)
		}
	}
}

func TestGenerateAllWithSkillsEmptySkillsDir(t *testing.T) {
	app := NewUtility("grit", "Git operations")
	app.Version = "0.1.0"

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		OldParams: []OldParam{
			{Name: "repo_path", Type: String, Description: "Path to repo", Required: true},
		},
		MapsTools: []ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git status"}, UseWhen: "checking status"},
		},
	})

	dir := t.TempDir()
	if err := app.GenerateAllWithSkills(dir, ""); err != nil {
		t.Fatalf("GenerateAllWithSkills with empty skillsDir: %v", err)
	}

	// Should behave identically to GenerateAll — no skills in plugin.json
	pluginPath := filepath.Join(dir, "share", "purse-first", "grit", ".claude-plugin", "plugin.json")
	data, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatalf("read plugin.json: %v", err)
	}

	var plugin map[string]any
	if err := json.Unmarshal(data, &plugin); err != nil {
		t.Fatalf("unmarshal plugin.json: %v", err)
	}

	if _, ok := plugin["skills"]; ok {
		t.Error("skills should be omitted when skillsDir is empty")
	}
}

func newTestApp() *Utility {
	app := NewUtility("grit", "Git operations")
	app.Version = "0.1.0"
	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		OldParams: []OldParam{
			{Name: "repo_path", Type: String, Description: "Path to repo", Required: true},
		},
		MapsTools: []ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git status"}, UseWhen: "checking status"},
		},
	})
	return app
}

func TestHandleGeneratePluginDirectoryMode(t *testing.T) {
	app := newTestApp()
	dir := t.TempDir()

	if err := app.HandleGeneratePlugin([]string{dir}, os.Stdout); err != nil {
		t.Fatalf("HandleGeneratePlugin directory mode: %v", err)
	}

	pluginPath := filepath.Join(dir, "share", "purse-first", "grit", ".claude-plugin", "plugin.json")
	if _, err := os.Stat(pluginPath); err != nil {
		t.Errorf("expected plugin.json at %s", pluginPath)
	}
}

func TestHandleGeneratePluginPWDDefault(t *testing.T) {
	app := newTestApp()
	dir := t.TempDir()

	// Change to temp dir so PWD-default writes there
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	if err := app.HandleGeneratePlugin([]string{}, os.Stdout); err != nil {
		t.Fatalf("HandleGeneratePlugin PWD default: %v", err)
	}

	pluginPath := filepath.Join(dir, "share", "purse-first", "grit", ".claude-plugin", "plugin.json")
	if _, err := os.Stat(pluginPath); err != nil {
		t.Errorf("expected plugin.json at %s", pluginPath)
	}
}

func TestHandleGeneratePluginStdoutMode(t *testing.T) {
	app := newTestApp()
	var buf bytes.Buffer

	if err := app.HandleGeneratePlugin([]string{"-"}, &buf); err != nil {
		t.Fatalf("HandleGeneratePlugin stdout mode: %v", err)
	}

	var plugin map[string]any
	if err := json.Unmarshal(buf.Bytes(), &plugin); err != nil {
		t.Fatalf("unmarshal stdout JSON: %v", err)
	}

	if plugin["name"] != "grit" {
		t.Errorf("name = %v, want grit", plugin["name"])
	}
}

func TestHandleGeneratePluginStdoutWritesNoFiles(t *testing.T) {
	app := newTestApp()
	dir := t.TempDir()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	var buf bytes.Buffer
	if err := app.HandleGeneratePlugin([]string{"-"}, &buf); err != nil {
		t.Fatalf("HandleGeneratePlugin stdout mode: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no files in %s, got %d", dir, len(entries))
	}
}

func TestHandleGeneratePluginWithSkillsDir(t *testing.T) {
	app := NewUtility("chix", "Nix MCP server")
	app.Version = "0.1.0"
	app.AddCommand(&Command{
		Name:        "eval",
		Description: Description{Short: "Evaluate nix expression"},
		OldParams: []OldParam{
			{Name: "expr", Type: String, Description: "Nix expression", Required: true},
		},
	})

	skillsDir := t.TempDir()
	for _, name := range []string{"nix-patterns", "flake-debugging"} {
		skillDir := filepath.Join(skillsDir, name)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
		content := "# " + name + "\n\nSkill content for " + name + ".\n"
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatalf("write SKILL.md for %s: %v", name, err)
		}
	}

	dir := t.TempDir()
	if err := app.HandleGeneratePlugin([]string{"--skills-dir", skillsDir, dir}, os.Stdout); err != nil {
		t.Fatalf("HandleGeneratePlugin with --skills-dir: %v", err)
	}

	pluginPath := filepath.Join(dir, "share", "purse-first", "chix", ".claude-plugin", "plugin.json")
	data, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatalf("read plugin.json: %v", err)
	}

	var plugin map[string]any
	if err := json.Unmarshal(data, &plugin); err != nil {
		t.Fatalf("unmarshal plugin.json: %v", err)
	}

	skillsRaw, ok := plugin["skills"]
	if !ok {
		t.Fatal("plugin.json missing 'skills' key")
	}

	skills := skillsRaw.([]any)
	if len(skills) != 2 {
		t.Fatalf("skills length = %d, want 2", len(skills))
	}

	if skills[0] != "./skills/flake-debugging" {
		t.Errorf("skills[0] = %v, want ./skills/flake-debugging", skills[0])
	}
	if skills[1] != "./skills/nix-patterns" {
		t.Errorf("skills[1] = %v, want ./skills/nix-patterns", skills[1])
	}
}
