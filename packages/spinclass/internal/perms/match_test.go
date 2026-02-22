package perms

import (
	"testing"
)

func TestMatchExactTool(t *testing.T) {
	rules := []string{"Read"}

	if !MatchesAnyRule(rules, "Read", nil) {
		t.Error("expected Read to match rule 'Read'")
	}

	if MatchesAnyRule(rules, "Write", nil) {
		t.Error("expected Write not to match rule 'Read'")
	}
}

func TestMatchBashWildcard(t *testing.T) {
	rules := []string{"Bash(go test:*)"}

	if !MatchesAnyRule(rules, "Bash", map[string]any{"command": "go test ./..."}) {
		t.Error("expected 'go test ./...' to match 'Bash(go test:*)'")
	}

	if !MatchesAnyRule(rules, "Bash", map[string]any{"command": "go test"}) {
		t.Error("expected 'go test' to match 'Bash(go test:*)'")
	}

	if MatchesAnyRule(rules, "Bash", map[string]any{"command": "go build"}) {
		t.Error("expected 'go build' not to match 'Bash(go test:*)'")
	}
}

func TestMatchBashExact(t *testing.T) {
	rules := []string{"Bash(git status)"}

	if !MatchesAnyRule(rules, "Bash", map[string]any{"command": "git status"}) {
		t.Error("expected 'git status' to match 'Bash(git status)'")
	}

	if MatchesAnyRule(rules, "Bash", map[string]any{"command": "git status -s"}) {
		t.Error("expected 'git status -s' not to match 'Bash(git status)'")
	}
}

func TestMatchMCPTool(t *testing.T) {
	rules := []string{"mcp__plugin_nix_nix__build"}

	if !MatchesAnyRule(rules, "mcp__plugin_nix_nix__build", nil) {
		t.Error("expected mcp__plugin_nix_nix__build to match")
	}

	if MatchesAnyRule(rules, "mcp__plugin_nix_nix__eval", nil) {
		t.Error("expected mcp__plugin_nix_nix__eval not to match")
	}
}

func TestMatchBashColonWildcard(t *testing.T) {
	rules := []string{"Bash(npm run:*)"}

	if !MatchesAnyRule(rules, "Bash", map[string]any{"command": "npm run build"}) {
		t.Error("expected 'npm run build' to match 'Bash(npm run:*)'")
	}

	if !MatchesAnyRule(rules, "Bash", map[string]any{"command": "npm run"}) {
		t.Error("expected 'npm run' to match 'Bash(npm run:*)'")
	}

	if MatchesAnyRule(rules, "Bash", map[string]any{"command": "npm install"}) {
		t.Error("expected 'npm install' not to match 'Bash(npm run:*)'")
	}
}

func TestMatchBashTrailingWildcard(t *testing.T) {
	rules := []string{"Bash(git *)"}

	if !MatchesAnyRule(rules, "Bash", map[string]any{"command": "git status"}) {
		t.Error("expected 'git status' to match 'Bash(git *)'")
	}

	if !MatchesAnyRule(rules, "Bash", map[string]any{"command": "git log --oneline"}) {
		t.Error("expected 'git log --oneline' to match 'Bash(git *)'")
	}

	if MatchesAnyRule(rules, "Bash", map[string]any{"command": "nix build"}) {
		t.Error("expected 'nix build' not to match 'Bash(git *)'")
	}
}

func TestBuildPermissionString(t *testing.T) {
	tests := []struct {
		name      string
		toolName  string
		toolInput map[string]any
		want      string
	}{
		{
			name:      "bash command",
			toolName:  "Bash",
			toolInput: map[string]any{"command": "go test ./..."},
			want:      "Bash(go test ./...)",
		},
		{
			name:      "plain read",
			toolName:  "Read",
			toolInput: nil,
			want:      "Read",
		},
		{
			name:      "read with file_path",
			toolName:  "Read",
			toolInput: map[string]any{"file_path": "/home/user/file.go"},
			want:      "Read(/home/user/file.go)",
		},
		{
			name:      "write with file_path",
			toolName:  "Write",
			toolInput: map[string]any{"file_path": "/tmp/out.txt"},
			want:      "Write(/tmp/out.txt)",
		},
		{
			name:      "edit with file_path",
			toolName:  "Edit",
			toolInput: map[string]any{"file_path": "/tmp/out.txt"},
			want:      "Edit(/tmp/out.txt)",
		},
		{
			name:      "web fetch with url",
			toolName:  "WebFetch",
			toolInput: map[string]any{"url": "https://example.com"},
			want:      "WebFetch(https://example.com)",
		},
		{
			name:      "mcp tool",
			toolName:  "mcp__plugin_nix_nix__build",
			toolInput: map[string]any{"installable": ".#default"},
			want:      "mcp__plugin_nix_nix__build",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildPermissionString(tt.toolName, tt.toolInput)
			if got != tt.want {
				t.Errorf("BuildPermissionString(%q, ...) = %q, want %q",
					tt.toolName, got, tt.want)
			}
		})
	}
}

func TestMatchSlashStarWildcard(t *testing.T) {
	rules := []string{"Read(/home/user/eng/worktrees/repo/branch/*)"}

	if !MatchesAnyRule(rules, "Read", map[string]any{"file_path": "/home/user/eng/worktrees/repo/branch/file.go"}) {
		t.Error("expected file in worktree to match")
	}

	if !MatchesAnyRule(rules, "Read", map[string]any{"file_path": "/home/user/eng/worktrees/repo/branch/sub/dir/file.go"}) {
		t.Error("expected nested file in worktree to match")
	}

	if MatchesAnyRule(rules, "Read", map[string]any{"file_path": "/home/user/eng/worktrees/repo/other/file.go"}) {
		t.Error("expected file outside worktree not to match")
	}

	if MatchesAnyRule(rules, "Write", map[string]any{"file_path": "/home/user/eng/worktrees/repo/branch/file.go"}) {
		t.Error("expected wrong tool not to match")
	}
}

func TestMatchSlashStarEdit(t *testing.T) {
	rules := []string{"Edit(/tmp/wt/*)"}

	if !MatchesAnyRule(rules, "Edit", map[string]any{"file_path": "/tmp/wt/main.go"}) {
		t.Error("expected Edit in dir to match")
	}

	if MatchesAnyRule(rules, "Edit", map[string]any{"file_path": "/tmp/other/main.go"}) {
		t.Error("expected Edit outside dir not to match")
	}
}

func TestMatchingRuleName(t *testing.T) {
	rules := []string{"Read", "Bash(git *)", "mcp__plugin_nix_nix__build"}

	rule, ok := MatchingRule(rules, "Bash", map[string]any{"command": "git status"})
	if !ok {
		t.Fatal("expected a matching rule")
	}
	if rule != "Bash(git *)" {
		t.Errorf("expected matched rule 'Bash(git *)', got %q", rule)
	}

	rule, ok = MatchingRule(rules, "Read", nil)
	if !ok {
		t.Fatal("expected Read to match")
	}
	if rule != "Read" {
		t.Errorf("expected matched rule 'Read', got %q", rule)
	}

	rule, ok = MatchingRule(rules, "mcp__plugin_nix_nix__build", nil)
	if !ok {
		t.Fatal("expected mcp tool to match")
	}
	if rule != "mcp__plugin_nix_nix__build" {
		t.Errorf("expected matched rule 'mcp__plugin_nix_nix__build', got %q", rule)
	}

	_, ok = MatchingRule(rules, "Write", nil)
	if ok {
		t.Error("expected no match for Write")
	}
}
