package localplugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverSkillsSingle(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "my-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# skill"), 0o644); err != nil {
		t.Fatal(err)
	}

	skills, err := DiscoverSkills(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0] != "./skills/my-skill" {
		t.Errorf("expected ./skills/my-skill, got %s", skills[0])
	}
}

func TestDiscoverSkillsMultiple(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"alpha", "bravo", "charlie"} {
		skillDir := filepath.Join(root, "skills", name)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# skill"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	skills, err := DiscoverSkills(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(skills) != 3 {
		t.Fatalf("expected 3 skills, got %d", len(skills))
	}
}

func TestDiscoverSkillsNone(t *testing.T) {
	root := t.TempDir()

	skills, err := DiscoverSkills(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(skills) != 0 {
		t.Fatalf("expected 0 skills, got %d", len(skills))
	}
}

func TestGeneratePreservesUnknownFields(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "test-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# skill"), 0o644); err != nil {
		t.Fatal(err)
	}

	pluginDir := filepath.Join(root, ".claude-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}

	original := map[string]any{
		"name":        "test-plugin",
		"description": "A test plugin",
		"author":      map[string]any{"name": "tester"},
		"customField": "should be preserved",
	}
	data, _ := json.MarshalIndent(original, "", "  ")
	pluginPath := filepath.Join(pluginDir, "plugin.json")
	if err := os.WriteFile(pluginPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Generate(root, pluginPath); err != nil {
		t.Fatal(err)
	}

	result, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatal(err)
	}

	if got["name"] != "test-plugin" {
		t.Errorf("name not preserved: %v", got["name"])
	}
	if got["customField"] != "should be preserved" {
		t.Errorf("customField not preserved: %v", got["customField"])
	}

	skills, ok := got["skills"].([]any)
	if !ok {
		t.Fatalf("skills not found or wrong type: %v", got["skills"])
	}
	if len(skills) != 1 || skills[0] != "./skills/test-skill" {
		t.Errorf("unexpected skills: %v", skills)
	}
}

func TestGenerateRemovesSkillsWhenNone(t *testing.T) {
	root := t.TempDir()

	pluginDir := filepath.Join(root, ".claude-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}

	original := map[string]any{
		"name":   "test-plugin",
		"skills": []string{"./skills/old-skill"},
	}
	data, _ := json.MarshalIndent(original, "", "  ")
	pluginPath := filepath.Join(pluginDir, "plugin.json")
	if err := os.WriteFile(pluginPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Generate(root, pluginPath); err != nil {
		t.Fatal(err)
	}

	result, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatal(err)
	}

	if _, exists := got["skills"]; exists {
		t.Errorf("skills key should have been removed, got: %v", got["skills"])
	}
}
