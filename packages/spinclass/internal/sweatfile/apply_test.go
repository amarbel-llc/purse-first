package sweatfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestHardcodedDefaultsGitExcludes(t *testing.T) {
	defaults := HardcodedDefaults()

	if len(defaults.GitExcludes) != 2 {
		t.Fatalf("expected 2 git excludes, got %d: %v", len(defaults.GitExcludes), defaults.GitExcludes)
	}

	want := []string{".claude/settings.local.json", ".tmp"}
	for i, w := range want {
		if defaults.GitExcludes[i] != w {
			t.Errorf("GitExcludes[%d]: got %q, want %q", i, defaults.GitExcludes[i], w)
		}
	}
}

func TestHardcodedDefaultsClaudeAllow(t *testing.T) {
	defaults := HardcodedDefaults()

	home, _ := os.UserHomeDir()
	if home == "" {
		if defaults.ClaudeAllow != nil {
			t.Errorf("expected nil ClaudeAllow when HOME is empty, got %v", defaults.ClaudeAllow)
		}
		return
	}

	if len(defaults.ClaudeAllow) != 1 {
		t.Fatalf("expected 1 claude allow rule, got %d: %v", len(defaults.ClaudeAllow), defaults.ClaudeAllow)
	}

	wantRule := "Read(" + filepath.Join(home, ".claude") + "/*)"
	if defaults.ClaudeAllow[0] != wantRule {
		t.Errorf("ClaudeAllow[0]: got %q, want %q", defaults.ClaudeAllow[0], wantRule)
	}
}

func TestApplyGitExcludes(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git", "info")
	os.MkdirAll(gitDir, 0o755)
	excludePath := filepath.Join(gitDir, "exclude")

	err := applyGitExcludes(excludePath, []string{".claude/", ".direnv/", ".tmp"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(excludePath)
	if string(data) != ".claude/\n.direnv/\n.tmp\n" {
		t.Errorf("exclude content: got %q", string(data))
	}
}

func TestApplyClaudeSettings(t *testing.T) {
	dir := t.TempDir()
	rules := []string{"Read", "Glob", "Bash(git *)"}

	err := ApplyClaudeSettings(dir, Sweatfile{ClaudeAllow: rules})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.local.json"))
	if err != nil {
		t.Fatalf("reading settings: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parsing settings: %v", err)
	}

	permsMap, _ := doc["permissions"].(map[string]any)
	if permsMap == nil {
		t.Fatal("expected permissions key")
	}

	defaultMode, _ := permsMap["defaultMode"].(string)
	if defaultMode != "acceptEdits" {
		t.Errorf("defaultMode: got %q, want %q", defaultMode, "acceptEdits")
	}

	allowRaw, _ := permsMap["allow"].([]any)
	if len(allowRaw) != 6 {
		t.Fatalf("expected 6 rules (3 passed + 3 scoped), got %d: %v", len(allowRaw), allowRaw)
	}

	// First 3 are from the passed rules
	for i, want := range rules {
		got, _ := allowRaw[i].(string)
		if got != want {
			t.Errorf("rule %d: got %q, want %q", i, got, want)
		}
	}

	// Last 3 are auto-injected scoped rules
	readRule, _ := allowRaw[3].(string)
	editRule, _ := allowRaw[4].(string)
	writeRule, _ := allowRaw[5].(string)

	wantRead := "Read(" + dir + "/*)"
	wantEdit := "Edit(" + dir + "/*)"
	wantWrite := "Write(" + dir + "/*)"
	if readRule != wantRead {
		t.Errorf("read rule: got %q, want %q", readRule, wantRead)
	}
	if editRule != wantEdit {
		t.Errorf("edit rule: got %q, want %q", editRule, wantEdit)
	}
	if writeRule != wantWrite {
		t.Errorf("write rule: got %q, want %q", writeRule, wantWrite)
	}
}

func TestApplyClaudeSettingsEmpty(t *testing.T) {
	dir := t.TempDir()

	err := ApplyClaudeSettings(dir, Sweatfile{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.local.json"))
	if err != nil {
		t.Fatalf("reading settings: %v", err)
	}

	var doc map[string]any
	json.Unmarshal(data, &doc)
	permsMap, _ := doc["permissions"].(map[string]any)
	allowRaw, _ := permsMap["allow"].([]any)

	// Even with no passed rules, the 3 scoped rules are injected
	if len(allowRaw) != 3 {
		t.Fatalf("expected 3 scoped rules, got %d: %v", len(allowRaw), allowRaw)
	}
}

func TestApplyClaudeSettingsPreservesExistingKeys(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	os.MkdirAll(claudeDir, 0o755)

	existing := map[string]any{
		"mcpServers": map[string]any{"test": true},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	os.WriteFile(filepath.Join(claudeDir, "settings.local.json"), data, 0o644)

	err := ApplyClaudeSettings(dir, Sweatfile{ClaudeAllow: []string{"Read"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, _ := os.ReadFile(filepath.Join(claudeDir, "settings.local.json"))
	var doc map[string]any
	json.Unmarshal(result, &doc)

	if _, ok := doc["mcpServers"]; !ok {
		t.Error("expected mcpServers key to be preserved")
	}
}

func TestApplyClaudeSettingsWritesHooksForWorktree(t *testing.T) {
	dir := t.TempDir()

	// Simulate a worktree by creating .git as a file (not directory)
	os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /tmp/fake"), 0o644)

	err := ApplyClaudeSettings(dir, Sweatfile{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.local.json"))
	if err != nil {
		t.Fatalf("reading settings: %v", err)
	}

	var doc map[string]any
	json.Unmarshal(data, &doc)

	hooksRaw, ok := doc["hooks"]
	if !ok {
		t.Fatal("expected hooks key in settings")
	}

	hooks := hooksRaw.(map[string]any)
	preToolUse, ok := hooks["PreToolUse"]
	if !ok {
		t.Fatal("expected PreToolUse key in hooks")
	}

	entries := preToolUse.([]any)
	if len(entries) != 1 {
		t.Fatalf("expected 1 PreToolUse entry, got %d", len(entries))
	}

	entry := entries[0].(map[string]any)
	matcher := entry["matcher"].(string)
	if matcher != "Read|Write|Edit|Glob|Grep|Bash|Task" {
		t.Errorf("matcher: got %q", matcher)
	}

	hooksList := entry["hooks"].([]any)
	hook := hooksList[0].(map[string]any)
	if hook["type"] != "command" {
		t.Errorf("hook type: got %q", hook["type"])
	}
	if hook["command"] != "spinclass hooks --worktree-boundary-violations-notification" {
		t.Errorf("hook command: got %q", hook["command"])
	}
}

func TestApplyClaudeSettingsNoHooksForMainRepo(t *testing.T) {
	dir := t.TempDir()

	// Simulate a main repo by creating .git as a directory
	os.MkdirAll(filepath.Join(dir, ".git"), 0o755)

	err := ApplyClaudeSettings(dir, Sweatfile{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".claude", "settings.local.json"))
	var doc map[string]any
	json.Unmarshal(data, &doc)

	if _, ok := doc["hooks"]; ok {
		t.Error("expected no hooks key for main repo")
	}
}

func TestApplyClaudeSettingsWritesStopHookWhenConfigured(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /tmp/fake"), 0o644)

	cmd := "just test"
	err := ApplyClaudeSettings(dir, Sweatfile{StopHook: &cmd})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".claude", "settings.local.json"))
	var doc map[string]any
	json.Unmarshal(data, &doc)

	hooks := doc["hooks"].(map[string]any)

	stopRaw, ok := hooks["Stop"]
	if !ok {
		t.Fatal("expected Stop key in hooks")
	}

	entries := stopRaw.([]any)
	if len(entries) != 1 {
		t.Fatalf("expected 1 Stop entry, got %d", len(entries))
	}

	entry := entries[0].(map[string]any)
	if entry["matcher"] != "*" {
		t.Errorf("matcher: got %q", entry["matcher"])
	}
}

func TestApplyClaudeSettingsNoStopHookWhenNotConfigured(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /tmp/fake"), 0o644)

	err := ApplyClaudeSettings(dir, Sweatfile{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".claude", "settings.local.json"))
	var doc map[string]any
	json.Unmarshal(data, &doc)

	hooks := doc["hooks"].(map[string]any)
	if _, ok := hooks["Stop"]; ok {
		t.Error("expected no Stop key when stop_hook is not configured")
	}
}

func TestPrepareDirenvSkipsWhenNoFlakeNix(t *testing.T) {
	dir := t.TempDir()

	err := prepareDirenvIfNecessary(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	envrcPath := filepath.Join(dir, ".envrc")
	if _, err := os.Stat(envrcPath); err == nil {
		t.Error("expected no .envrc when flake.nix is absent")
	}
}

func TestPrepareDirenvSkipsWhenDirenvNotInPath(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "flake.nix"), []byte("{}"), 0o644)

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir())
	defer os.Setenv("PATH", origPath)

	err := prepareDirenvIfNecessary(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	envrcPath := filepath.Join(dir, ".envrc")
	if _, err := os.Stat(envrcPath); err == nil {
		t.Error("expected no .envrc when direnv is not in PATH")
	}
}

func TestPrepareDirenvWritesEnvrc(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "flake.nix"), []byte("{}"), 0o644)

	// Create a fake direnv that just exits 0
	fakeBin := t.TempDir()
	fakeDirenv := filepath.Join(fakeBin, "direnv")
	os.WriteFile(fakeDirenv, []byte("#!/bin/sh\nexit 0\n"), 0o755)

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", fakeBin)
	defer os.Setenv("PATH", origPath)

	err := prepareDirenvIfNecessary(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".envrc"))
	if err != nil {
		t.Fatalf("reading .envrc: %v", err)
	}

	want := "source_up\nuse flake\n"
	if string(data) != want {
		t.Errorf(".envrc content: got %q, want %q", string(data), want)
	}
}

func TestPrepareDirenvOverwritesExistingEnvrc(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "flake.nix"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(dir, ".envrc"), []byte("old content\n"), 0o644)

	fakeBin := t.TempDir()
	fakeDirenv := filepath.Join(fakeBin, "direnv")
	os.WriteFile(fakeDirenv, []byte("#!/bin/sh\nexit 0\n"), 0o755)

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", fakeBin)
	defer os.Setenv("PATH", origPath)

	err := prepareDirenvIfNecessary(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".envrc"))
	if err != nil {
		t.Fatalf("reading .envrc: %v", err)
	}

	want := "source_up\nuse flake\n"
	if string(data) != want {
		t.Errorf(".envrc content: got %q, want %q (old content should be replaced)", string(data), want)
	}
}
