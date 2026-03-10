package sweatfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHardcodedDefaultsGitExcludes(t *testing.T) {
	defaults := GetDefault()

	if defaults.GitSkipIndex == nil {
		t.Fatal("expected non-nil git excludes slice")
	}

	if len(defaults.GitSkipIndex) != 0 {
		t.Fatalf("expected 0 git excludes, got %d: %v", len(defaults.GitSkipIndex), defaults.GitSkipIndex)
	}
}

func TestHardcodedDefaultsClaudeAllow(t *testing.T) {
	defaults := GetDefault()

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

func TestApplyClaudeSettingsOverwritesExistingKeys(t *testing.T) {
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

	if _, ok := doc["mcpServers"]; ok {
		t.Error("expected mcpServers key to be overwritten")
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
	if hook["command"] != "spinclass hooks" {
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
	err := ApplyClaudeSettings(dir, Sweatfile{Hooks: &Hooks{Stop: &cmd}})
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
		t.Error("expected no Stop key when stop-hook is not configured")
	}
}

func TestPrepareDirenvWritesEnvrcWithoutUseFlakeWhenNoFlakeNix(t *testing.T) {
	dir := t.TempDir()

	// Create a fake direnv that just exits 0
	fakeBin := t.TempDir()
	fakeDirenv := filepath.Join(fakeBin, "direnv")
	os.WriteFile(fakeDirenv, []byte("#!/bin/sh\nexit 0\n"), 0o755)

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", fakeBin)
	defer os.Setenv("PATH", origPath)

	err := Sweatfile{}.prepareDirenv(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".envrc"))
	if err != nil {
		t.Fatalf("reading .envrc: %v", err)
	}

	content := string(data)

	binAbs, _ := filepath.Abs(".git/spinclass/bin")
	wantPathAdd := fmt.Sprintf("PATH_add \"%s\"\n", binAbs)

	// Should have source_up and PATH_add but NOT use flake
	want := "source_up\n" + wantPathAdd
	if content != want {
		t.Errorf(".envrc content: got %q, want %q", content, want)
	}
}

func TestPrepareDirenvSkipsWhenDirenvNotInPath(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "flake.nix"), []byte("{}"), 0o644)

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir())
	defer os.Setenv("PATH", origPath)

	err := Sweatfile{}.prepareDirenv(dir)
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

	err := Sweatfile{}.prepareDirenv(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".envrc"))
	if err != nil {
		t.Fatalf("reading .envrc: %v", err)
	}

	binAbs, _ := filepath.Abs(".git/spinclass/bin")
	wantPathAdd := fmt.Sprintf("PATH_add \"%s\"\n", binAbs)
	want := "source_up\nuse flake\n" + wantPathAdd
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

	err := Sweatfile{}.prepareDirenv(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".envrc"))
	if err != nil {
		t.Fatalf("reading .envrc: %v", err)
	}

	binAbs, _ := filepath.Abs(".git/spinclass/bin")
	wantPathAdd := fmt.Sprintf("PATH_add \"%s\"\n", binAbs)
	want := "source_up\nuse flake\n" + wantPathAdd
	if string(data) != want {
		t.Errorf(".envrc content: got %q, want %q (old content should be replaced)", string(data), want)
	}
}

func TestWriteEnvrcWithDirectives(t *testing.T) {
	dir := t.TempDir()

	fakeBin := t.TempDir()
	os.WriteFile(filepath.Join(fakeBin, "direnv"), []byte("#!/bin/sh\nexit 0\n"), 0o755)
	t.Setenv("PATH", fakeBin)

	sf := Sweatfile{EnvrcDirectives: []string{"source_up", "dotenv_if_exists"}}
	err := sf.prepareDirenv(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".envrc"))
	content := string(data)

	binAbs, _ := filepath.Abs(".git/spinclass/bin")
	wantPathAdd := fmt.Sprintf("PATH_add \"%s\"\n", binAbs)
	want := "source_up\ndotenv_if_exists\n" + wantPathAdd
	if content != want {
		t.Errorf(".envrc content:\ngot  %q\nwant %q", content, want)
	}
}

func TestWriteEnvrcDefaultFallbackWithFlake(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "flake.nix"), []byte("{}"), 0o644)

	fakeBin := t.TempDir()
	os.WriteFile(filepath.Join(fakeBin, "direnv"), []byte("#!/bin/sh\nexit 0\n"), 0o755)
	t.Setenv("PATH", fakeBin)

	sf := Sweatfile{}
	err := sf.prepareDirenv(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".envrc"))
	content := string(data)

	binAbs, _ := filepath.Abs(".git/spinclass/bin")
	wantPathAdd := fmt.Sprintf("PATH_add \"%s\"\n", binAbs)
	want := "source_up\nuse flake\n" + wantPathAdd
	if content != want {
		t.Errorf(".envrc content:\ngot  %q\nwant %q", content, want)
	}
}

func TestWriteEnvrcDefaultFallbackWithoutFlake(t *testing.T) {
	dir := t.TempDir()

	fakeBin := t.TempDir()
	os.WriteFile(filepath.Join(fakeBin, "direnv"), []byte("#!/bin/sh\nexit 0\n"), 0o755)
	t.Setenv("PATH", fakeBin)

	sf := Sweatfile{}
	err := sf.prepareDirenv(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".envrc"))
	content := string(data)

	binAbs, _ := filepath.Abs(".git/spinclass/bin")
	wantPathAdd := fmt.Sprintf("PATH_add \"%s\"\n", binAbs)
	want := "source_up\n" + wantPathAdd
	if content != want {
		t.Errorf(".envrc content:\ngot  %q\nwant %q", content, want)
	}
}

func TestWriteSpinclassEnv(t *testing.T) {
	dir := t.TempDir()

	fakeBin := t.TempDir()
	os.WriteFile(filepath.Join(fakeBin, "direnv"), []byte("#!/bin/sh\nexit 0\n"), 0o755)
	t.Setenv("PATH", fakeBin)

	sf := Sweatfile{
		Env: map[string]string{
			"FOO": "bar",
			"BAZ": "qux",
		},
	}
	err := sf.Apply(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".spinclass.env"))
	if err != nil {
		t.Fatalf("reading .spinclass.env: %v", err)
	}

	content := string(data)
	if content != "BAZ=qux\nFOO=bar\n" {
		t.Errorf(".spinclass.env content: got %q", content)
	}
}

func TestWriteSpinclassEnvInterpolatesWorktree(t *testing.T) {
	dir := t.TempDir()

	fakeBin := t.TempDir()
	os.WriteFile(filepath.Join(fakeBin, "direnv"), []byte("#!/bin/sh\nexit 0\n"), 0o755)
	t.Setenv("PATH", fakeBin)

	sf := Sweatfile{
		Env: map[string]string{
			"INCLUDE_PATH": "$WORKTREE/lib:.",
		},
	}
	err := sf.Apply(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".spinclass.env"))
	want := fmt.Sprintf("INCLUDE_PATH=%s/lib:.\n", dir)
	if string(data) != want {
		t.Errorf(".spinclass.env content:\ngot  %q\nwant %q", string(data), want)
	}
}

func TestEnvAutoDotenvDirective(t *testing.T) {
	dir := t.TempDir()

	fakeBin := t.TempDir()
	os.WriteFile(filepath.Join(fakeBin, "direnv"), []byte("#!/bin/sh\nexit 0\n"), 0o755)
	t.Setenv("PATH", fakeBin)

	sf := Sweatfile{
		Env: map[string]string{"FOO": "bar"},
	}
	err := sf.Apply(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".envrc"))
	content := string(data)
	if !strings.Contains(content, "dotenv .spinclass.env") {
		t.Errorf("expected dotenv .spinclass.env in .envrc, got %q", content)
	}
}

func TestNoEnvNoDotenvDirective(t *testing.T) {
	dir := t.TempDir()

	fakeBin := t.TempDir()
	os.WriteFile(filepath.Join(fakeBin, "direnv"), []byte("#!/bin/sh\nexit 0\n"), 0o755)
	t.Setenv("PATH", fakeBin)

	sf := Sweatfile{}
	err := sf.Apply(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".envrc"))
	content := string(data)
	if strings.Contains(content, "dotenv") {
		t.Errorf("expected no dotenv in .envrc when env is empty, got %q", content)
	}
}

func TestRunCreateHookExecutes(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "hook-ran")

	cmd := fmt.Sprintf("touch %s", marker)
	sf := Sweatfile{Hooks: &Hooks{Create: &cmd}}

	err := sf.RunCreateHook(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(marker); os.IsNotExist(err) {
		t.Error("expected create hook to run and create marker file")
	}
}

func TestRunCreateHookReceivesWorktreeEnv(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "worktree-path")

	cmd := fmt.Sprintf("echo $WORKTREE > %s", output)
	sf := Sweatfile{Hooks: &Hooks{Create: &cmd}}

	err := sf.RunCreateHook(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(output)
	got := strings.TrimSpace(string(data))
	if got != dir {
		t.Errorf("WORKTREE env: got %q, want %q", got, dir)
	}
}

func TestRunCreateHookFailureReturnsError(t *testing.T) {
	dir := t.TempDir()

	cmd := "exit 1"
	sf := Sweatfile{Hooks: &Hooks{Create: &cmd}}

	err := sf.RunCreateHook(dir)
	if err == nil {
		t.Error("expected error from failing create hook")
	}
}

func TestRunCreateHookNilIsNoop(t *testing.T) {
	dir := t.TempDir()
	sf := Sweatfile{}

	err := sf.RunCreateHook(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCreateHookEmptyStringIsNoop(t *testing.T) {
	dir := t.TempDir()
	empty := ""
	sf := Sweatfile{Hooks: &Hooks{Create: &empty}}

	err := sf.RunCreateHook(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
