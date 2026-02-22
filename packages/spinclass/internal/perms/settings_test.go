package perms

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadClaudeSettings(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, ".claude", "settings.local.json")

	os.MkdirAll(filepath.Dir(path), 0o755)

	settings := map[string]any{
		"permissions": map[string]any{
			"allow": []string{"Read", "Edit", "Bash(go test:*)"},
		},
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	os.WriteFile(path, data, 0o644)

	rules, err := LoadClaudeSettings(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rules))
	}
	if rules[0] != "Read" {
		t.Errorf("expected Read, got %q", rules[0])
	}
	if rules[1] != "Edit" {
		t.Errorf("expected Edit, got %q", rules[1])
	}
	if rules[2] != "Bash(go test:*)" {
		t.Errorf("expected Bash(go test:*), got %q", rules[2])
	}
}

func TestLoadClaudeSettingsMissing(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, ".claude", "settings.local.json")

	rules, err := LoadClaudeSettings(path)
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if rules != nil {
		t.Errorf("expected nil for missing file, got %v", rules)
	}
}

func TestSaveClaudeSettings(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sub", ".claude", "settings.local.json")

	rules := []string{"Read", "Bash(git *)", "mcp__plugin_nix_nix__build"}
	err := SaveClaudeSettings(path, rules)
	if err != nil {
		t.Fatalf("unexpected error saving: %v", err)
	}

	loaded, err := LoadClaudeSettings(path)
	if err != nil {
		t.Fatalf("unexpected error loading: %v", err)
	}
	if len(loaded) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(loaded))
	}
	if loaded[0] != "Read" {
		t.Errorf("expected Read, got %q", loaded[0])
	}
	if loaded[1] != "Bash(git *)" {
		t.Errorf("expected Bash(git *), got %q", loaded[1])
	}
	if loaded[2] != "mcp__plugin_nix_nix__build" {
		t.Errorf("expected mcp__plugin_nix_nix__build, got %q", loaded[2])
	}
}

func TestDiffRules(t *testing.T) {
	before := []string{"Read", "Edit"}
	after := []string{"Read", "Edit", "Bash(go test:*)", "Write"}

	diff := DiffRules(before, after)
	if len(diff) != 2 {
		t.Fatalf("expected 2 new rules, got %d: %v", len(diff), diff)
	}
	if diff[0] != "Bash(go test:*)" {
		t.Errorf("expected Bash(go test:*), got %q", diff[0])
	}
	if diff[1] != "Write" {
		t.Errorf("expected Write, got %q", diff[1])
	}
}

func TestDiffRulesNoChanges(t *testing.T) {
	rules := []string{"Read", "Edit", "Bash(go test:*)"}

	diff := DiffRules(rules, rules)
	if len(diff) != 0 {
		t.Errorf("expected no diff for identical rules, got %v", diff)
	}
}

func TestSaveClaudeSettingsPreservesOtherFields(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "settings.local.json")

	// Write a file with extra fields beyond permissions.allow
	initial := `{
  "permissions": {
    "allow": ["Read", "Edit"],
    "deny": ["Bash(rm -rf:*)"]
  },
  "env": {
    "FOO": "bar"
  }
}
`
	os.WriteFile(path, []byte(initial), 0o644)

	// Save with updated allow list
	err := SaveClaudeSettings(path, []string{"Read"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read back raw JSON and verify other fields survived
	data, _ := os.ReadFile(path)
	var doc map[string]any
	json.Unmarshal(data, &doc)

	permsMap, ok := doc["permissions"].(map[string]any)
	if !ok {
		t.Fatal("expected permissions key")
	}
	deny, ok := permsMap["deny"].([]any)
	if !ok || len(deny) != 1 {
		t.Errorf("expected deny list preserved, got %v", permsMap["deny"])
	}
	envMap, ok := doc["env"].(map[string]any)
	if !ok {
		t.Fatal("expected env key preserved")
	}
	if envMap["FOO"] != "bar" {
		t.Errorf("expected env.FOO=bar, got %v", envMap["FOO"])
	}
}

func TestRemoveRules(t *testing.T) {
	rules := []string{"Read", "Edit", "Bash(go test:*)", "Write"}
	toRemove := []string{"Edit", "Write"}

	result := RemoveRules(rules, toRemove)
	if len(result) != 2 {
		t.Fatalf("expected 2 remaining rules, got %d: %v", len(result), result)
	}
	if result[0] != "Read" {
		t.Errorf("expected Read, got %q", result[0])
	}
	if result[1] != "Bash(go test:*)" {
		t.Errorf("expected Bash(go test:*), got %q", result[1])
	}
}
