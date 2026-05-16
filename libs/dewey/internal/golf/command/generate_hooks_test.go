package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateHooksCreatesHooksJSON(t *testing.T) {
	app := NewUtility("grit", "Git operations")

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		MapsTools: []ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git status"}, UseWhen: "checking status"},
		},
	})

	app.AddCommand(&Command{
		Name:        "diff",
		Description: Description{Short: "Show changes"},
		MapsTools: []ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git diff"}, UseWhen: "viewing changes"},
		},
	})

	dir := t.TempDir()
	if err := app.GenerateHooks(dir); err != nil {
		t.Fatalf("GenerateHooks: %v", err)
	}

	path := filepath.Join(dir, "grit", "hooks", "hooks.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read hooks.json: %v", err)
	}

	var hooksFile struct {
		Hooks struct {
			PreToolUse []struct {
				Matcher string `json:"matcher"`
				Hooks   []struct {
					Type    string `json:"type"`
					Command string `json:"command"`
					Timeout int    `json:"timeout"`
				} `json:"hooks"`
			} `json:"PreToolUse"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &hooksFile); err != nil {
		t.Fatalf("unmarshal hooks.json: %v", err)
	}

	if len(hooksFile.Hooks.PreToolUse) != 1 {
		t.Fatalf("PreToolUse length = %d, want 1", len(hooksFile.Hooks.PreToolUse))
	}

	entry := hooksFile.Hooks.PreToolUse[0]
	if entry.Matcher != "Bash" {
		t.Errorf("matcher = %q, want Bash", entry.Matcher)
	}

	if len(entry.Hooks) != 1 {
		t.Fatalf("hooks length = %d, want 1", len(entry.Hooks))
	}

	hook := entry.Hooks[0]
	if hook.Type != "command" {
		t.Errorf("hook type = %q, want command", hook.Type)
	}
	if hook.Command != "${CLAUDE_PLUGIN_ROOT}/hooks/pre-tool-use" {
		t.Errorf("hook command = %q, want ${CLAUDE_PLUGIN_ROOT}/hooks/pre-tool-use", hook.Command)
	}
	if hook.Timeout != 5 {
		t.Errorf("hook timeout = %d, want 5", hook.Timeout)
	}
}

func TestGenerateHooksSkipsWhenNoMappings(t *testing.T) {
	app := NewUtility("test", "test")
	app.AddCommand(&Command{Name: "foo", Description: Description{Short: "a command"}})

	dir := t.TempDir()
	if err := app.GenerateHooks(dir); err != nil {
		t.Fatalf("GenerateHooks: %v", err)
	}

	hooksDir := filepath.Join(dir, "test", "hooks")
	if _, err := os.Stat(hooksDir); !os.IsNotExist(err) {
		t.Error("hooks/ directory should not exist when no commands have tool mappings")
	}
}

func TestGenerateHooksCreatesBinaryWrapper(t *testing.T) {
	app := NewUtility("grit", "Git operations")

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		MapsTools: []ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git status"}, UseWhen: "checking status"},
		},
	})

	dir := t.TempDir()
	if err := app.GenerateHooks(dir); err != nil {
		t.Fatalf("GenerateHooks: %v", err)
	}

	path := filepath.Join(dir, "grit", "hooks", "pre-tool-use")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat pre-tool-use: %v", err)
	}

	if info.Mode()&0o111 == 0 {
		t.Errorf("pre-tool-use is not executable: mode = %v", info.Mode())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pre-tool-use: %v", err)
	}

	content := string(data)
	if !strings.HasPrefix(content, "#!/bin/sh\n") {
		t.Errorf("pre-tool-use should start with shebang, got %q", content)
	}
	if !strings.Contains(content, "hook") {
		t.Errorf("pre-tool-use should contain 'hook' subcommand, got %q", content)
	}
}

func TestGenerateHooksMatcher(t *testing.T) {
	app := NewUtility("lux", "LSP multiplexer")

	app.AddCommand(&Command{
		Name:        "hover",
		Description: Description{Short: "Get type info"},
		MapsTools: []ToolMapping{
			{Replaces: "Read", Extensions: []string{".go"}, UseWhen: "getting type info"},
		},
	})

	app.AddCommand(&Command{
		Name:        "definition",
		Description: Description{Short: "Go to definition"},
		MapsTools: []ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"grep"}, UseWhen: "finding definitions"},
		},
	})

	// Add another command that also maps Read — should be deduplicated
	app.AddCommand(&Command{
		Name:        "references",
		Description: Description{Short: "Find references"},
		MapsTools: []ToolMapping{
			{Replaces: "Read", Extensions: []string{".go"}, UseWhen: "finding references"},
		},
	})

	dir := t.TempDir()
	if err := app.GenerateHooks(dir); err != nil {
		t.Fatalf("GenerateHooks: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "lux", "hooks", "hooks.json"))
	if err != nil {
		t.Fatalf("read hooks.json: %v", err)
	}

	var hooksFile struct {
		Hooks struct {
			PreToolUse []struct {
				Matcher string `json:"matcher"`
			} `json:"PreToolUse"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &hooksFile); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(hooksFile.Hooks.PreToolUse) != 1 {
		t.Fatalf("PreToolUse length = %d, want 1", len(hooksFile.Hooks.PreToolUse))
	}

	// Should be sorted alphabetically: Bash|Read
	if hooksFile.Hooks.PreToolUse[0].Matcher != "Bash|Read" {
		t.Errorf("matcher = %q, want Bash|Read", hooksFile.Hooks.PreToolUse[0].Matcher)
	}
}
