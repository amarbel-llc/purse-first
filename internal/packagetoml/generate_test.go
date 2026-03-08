package packagetoml

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratePluginJSON(t *testing.T) {
	pkg := &Package{
		Name:        "chix",
		Description: "Nix MCP server and skills for Claude Code",
		Author:      Author{Name: "friedenberg"},
		MCP: map[string]MCPServer{
			"chix": {Command: "chix"},
		},
		Hooks: map[string][]Hook{
			"PostToolUse": {
				{
					Matcher: "Edit|Write",
					Command: "${CLAUDE_PLUGIN_ROOT}/hooks/format-nix",
					Timeout: 30,
				},
			},
		},
	}

	outputDir := t.TempDir()

	// Set up a skills directory with two skills
	skillsDir := t.TempDir()
	for _, name := range []string{"nix-codebase", "design_patterns-no_cycles"} {
		dir := filepath.Join(skillsDir, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := GeneratePluginJSON(pkg, outputDir, skillsDir); err != nil {
		t.Fatalf("GeneratePluginJSON failed: %v", err)
	}

	// Read generated plugin.json
	pluginPath := filepath.Join(outputDir, "share", "purse-first", "chix", ".claude-plugin", "plugin.json")
	data, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatalf("reading plugin.json: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("parsing plugin.json: %v", err)
	}

	// Verify name
	if got["name"] != "chix" {
		t.Errorf("name = %v, want %q", got["name"], "chix")
	}

	// Verify description
	if got["description"] != "Nix MCP server and skills for Claude Code" {
		t.Errorf("description = %v, want %q", got["description"], "Nix MCP server and skills for Claude Code")
	}

	// Verify author
	author, ok := got["author"].(map[string]any)
	if !ok {
		t.Fatalf("author missing or wrong type: %v", got["author"])
	}
	if author["name"] != "friedenberg" {
		t.Errorf("author.name = %v, want %q", author["name"], "friedenberg")
	}

	// Verify mcpServers
	mcpServers, ok := got["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers missing or wrong type: %v", got["mcpServers"])
	}
	chixServer, ok := mcpServers["chix"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers.chix missing or wrong type: %v", mcpServers["chix"])
	}
	if chixServer["type"] != "stdio" {
		t.Errorf("mcpServers.chix.type = %v, want %q", chixServer["type"], "stdio")
	}
	if chixServer["command"] != "chix" {
		t.Errorf("mcpServers.chix.command = %v, want %q", chixServer["command"], "chix")
	}

	// Verify skills (sorted alphabetically)
	skills, ok := got["skills"].([]any)
	if !ok {
		t.Fatalf("skills missing or wrong type: %v", got["skills"])
	}
	if len(skills) != 2 {
		t.Fatalf("len(skills) = %d, want 2", len(skills))
	}
	if skills[0] != "./skills/design_patterns-no_cycles" {
		t.Errorf("skills[0] = %v, want %q", skills[0], "./skills/design_patterns-no_cycles")
	}
	if skills[1] != "./skills/nix-codebase" {
		t.Errorf("skills[1] = %v, want %q", skills[1], "./skills/nix-codebase")
	}

	// Verify hooks structure
	hooks, ok := got["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks missing or wrong type: %v", got["hooks"])
	}
	postToolUse, ok := hooks["PostToolUse"].([]any)
	if !ok {
		t.Fatalf("hooks.PostToolUse missing or wrong type: %v", hooks["PostToolUse"])
	}
	if len(postToolUse) != 1 {
		t.Fatalf("len(PostToolUse) = %d, want 1", len(postToolUse))
	}
	matcherObj, ok := postToolUse[0].(map[string]any)
	if !ok {
		t.Fatalf("PostToolUse[0] wrong type: %v", postToolUse[0])
	}
	if matcherObj["matcher"] != "Edit|Write" {
		t.Errorf("matcher = %v, want %q", matcherObj["matcher"], "Edit|Write")
	}
	innerHooks, ok := matcherObj["hooks"].([]any)
	if !ok {
		t.Fatalf("hooks array missing or wrong type: %v", matcherObj["hooks"])
	}
	if len(innerHooks) != 1 {
		t.Fatalf("len(innerHooks) = %d, want 1", len(innerHooks))
	}
	hookEntry, ok := innerHooks[0].(map[string]any)
	if !ok {
		t.Fatalf("innerHooks[0] wrong type: %v", innerHooks[0])
	}
	if hookEntry["type"] != "command" {
		t.Errorf("hook.type = %v, want %q", hookEntry["type"], "command")
	}
	if hookEntry["command"] != "${CLAUDE_PLUGIN_ROOT}/hooks/format-nix" {
		t.Errorf("hook.command = %v, want %q", hookEntry["command"], "${CLAUDE_PLUGIN_ROOT}/hooks/format-nix")
	}
	if hookEntry["timeout"] != float64(30) {
		t.Errorf("hook.timeout = %v, want 30", hookEntry["timeout"])
	}

	// Verify skill files were copied
	copiedSkill := filepath.Join(outputDir, "share", "purse-first", "chix", "skills", "nix-codebase", "SKILL.md")
	if _, err := os.Stat(copiedSkill); err != nil {
		t.Errorf("skill file not copied: %v", err)
	}
	copiedSkill2 := filepath.Join(outputDir, "share", "purse-first", "chix", "skills", "design_patterns-no_cycles", "SKILL.md")
	if _, err := os.Stat(copiedSkill2); err != nil {
		t.Errorf("skill file not copied: %v", err)
	}
}

func TestGenerateSkillOnlyPluginJSON(t *testing.T) {
	pkg := &Package{
		Name:        "robin",
		Description: "Expert skill for setting up and writing BATS integration tests",
		Author:      Author{Name: "friedenberg"},
	}

	outputDir := t.TempDir()

	// Set up a skills directory with one skill
	skillsDir := t.TempDir()
	dir := filepath.Join(skillsDir, "bats-testing")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# bats-testing"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := GeneratePluginJSON(pkg, outputDir, skillsDir); err != nil {
		t.Fatalf("GeneratePluginJSON failed: %v", err)
	}

	// Read generated plugin.json
	pluginPath := filepath.Join(outputDir, "share", "purse-first", "robin", ".claude-plugin", "plugin.json")
	data, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatalf("reading plugin.json: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("parsing plugin.json: %v", err)
	}

	// Verify name and description
	if got["name"] != "robin" {
		t.Errorf("name = %v, want %q", got["name"], "robin")
	}
	if got["description"] != "Expert skill for setting up and writing BATS integration tests" {
		t.Errorf("description = %v, want %q", got["description"], "Expert skill for setting up and writing BATS integration tests")
	}

	// Verify author
	author, ok := got["author"].(map[string]any)
	if !ok {
		t.Fatalf("author missing or wrong type: %v", got["author"])
	}
	if author["name"] != "friedenberg" {
		t.Errorf("author.name = %v, want %q", author["name"], "friedenberg")
	}

	// Verify mcpServers is omitted
	if _, exists := got["mcpServers"]; exists {
		t.Errorf("mcpServers should be omitted for skill-only package, got: %v", got["mcpServers"])
	}

	// Verify hooks is omitted
	if _, exists := got["hooks"]; exists {
		t.Errorf("hooks should be omitted for skill-only package, got: %v", got["hooks"])
	}

	// Verify skills present
	skills, ok := got["skills"].([]any)
	if !ok {
		t.Fatalf("skills missing or wrong type: %v", got["skills"])
	}
	if len(skills) != 1 {
		t.Fatalf("len(skills) = %d, want 1", len(skills))
	}
	if skills[0] != "./skills/bats-testing" {
		t.Errorf("skills[0] = %v, want %q", skills[0], "./skills/bats-testing")
	}
}
