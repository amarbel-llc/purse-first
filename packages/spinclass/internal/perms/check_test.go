package perms

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckMatchProducesAllow(t *testing.T) {
	tiersDir := t.TempDir()

	globalTier := Tier{Allow: []string{"Bash(go test:*)"}}
	data, _ := json.MarshalIndent(globalTier, "", "  ")
	os.WriteFile(filepath.Join(tiersDir, "global.json"), data, 0o644)

	input := map[string]any{
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": "go test ./..."},
		"cwd":        "/home/user/eng/worktrees/myrepo/feature",
	}
	inputJSON, _ := json.Marshal(input)

	var out bytes.Buffer
	err := RunCheck(bytes.NewReader(inputJSON), &out, tiersDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() == 0 {
		t.Fatal("expected output for a matching rule, got empty")
	}

	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}

	hookOutput, ok := result["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("expected hookSpecificOutput object, got %v", result["hookSpecificOutput"])
	}

	if hookOutput["hookEventName"] != "PermissionRequest" {
		t.Errorf("expected hookEventName PermissionRequest, got %v", hookOutput["hookEventName"])
	}

	decision, ok := hookOutput["decision"].(map[string]any)
	if !ok {
		t.Fatalf("expected decision object, got %v", hookOutput["decision"])
	}

	if decision["behavior"] != "allow" {
		t.Errorf("expected behavior allow, got %v", decision["behavior"])
	}

	sysMsg, ok := result["systemMessage"].(string)
	if !ok {
		t.Fatalf("expected systemMessage string, got %v", result["systemMessage"])
	}

	if !strings.Contains(sysMsg, "auto-approved") {
		t.Errorf("systemMessage should contain 'auto-approved', got %q", sysMsg)
	}

	if !strings.Contains(sysMsg, "global tier") {
		t.Errorf("systemMessage should indicate global tier, got %q", sysMsg)
	}

	if !strings.Contains(sysMsg, "Bash(go test ./...)") {
		t.Errorf("systemMessage should contain permission string, got %q", sysMsg)
	}
}

func TestCheckNoMatchProducesEmptyOutput(t *testing.T) {
	tiersDir := t.TempDir()

	globalTier := Tier{Allow: []string{"Bash(go test:*)"}}
	data, _ := json.MarshalIndent(globalTier, "", "  ")
	os.WriteFile(filepath.Join(tiersDir, "global.json"), data, 0o644)

	input := map[string]any{
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": "rm -rf /"},
		"cwd":        "/home/user/eng/worktrees/myrepo/feature",
	}
	inputJSON, _ := json.Marshal(input)

	var out bytes.Buffer
	err := RunCheck(bytes.NewReader(inputJSON), &out, tiersDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() != 0 {
		t.Errorf("expected empty output for non-matching rule, got %q", out.String())
	}
}

func TestCheckUsesRepoTier(t *testing.T) {
	tiersDir := t.TempDir()

	globalTier := Tier{Allow: []string{"Bash(git *)"}}
	globalData, _ := json.MarshalIndent(globalTier, "", "  ")
	os.WriteFile(filepath.Join(tiersDir, "global.json"), globalData, 0o644)

	repoDir := filepath.Join(tiersDir, "repos")
	os.MkdirAll(repoDir, 0o755)
	repoTier := Tier{Allow: []string{"Bash(cargo test:*)"}}
	repoData, _ := json.MarshalIndent(repoTier, "", "  ")
	os.WriteFile(filepath.Join(repoDir, "ssh-agent-mux.json"), repoData, 0o644)

	input := map[string]any{
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": "cargo test --release"},
		"cwd":        "/home/user/eng/worktrees/ssh-agent-mux/feature-branch",
	}
	inputJSON, _ := json.Marshal(input)

	var out bytes.Buffer
	err := RunCheck(bytes.NewReader(inputJSON), &out, tiersDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() == 0 {
		t.Fatal("expected output for a matching repo rule, got empty")
	}

	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}

	sysMsg, ok := result["systemMessage"].(string)
	if !ok {
		t.Fatalf("expected systemMessage string, got %v", result["systemMessage"])
	}

	if !strings.Contains(sysMsg, "ssh-agent-mux tier") {
		t.Errorf("systemMessage should indicate repo tier name, got %q", sysMsg)
	}

	if !strings.Contains(sysMsg, "Bash(cargo test --release)") {
		t.Errorf("systemMessage should contain permission string, got %q", sysMsg)
	}
}

func TestRepoFromCWD(t *testing.T) {
	tests := []struct {
		name string
		cwd  string
		want string
	}{
		{
			name: "worktree path",
			cwd:  "/home/user/eng/worktrees/myrepo/feature",
			want: "myrepo",
		},
		{
			name: "repos path",
			cwd:  "/home/user/eng/repos/dodder",
			want: "dodder",
		},
		{
			name: "deep worktree path",
			cwd:  "/home/user/eng/worktrees/ssh-agent-mux/main/src/lib",
			want: "ssh-agent-mux",
		},
		{
			name: "no match",
			cwd:  "/tmp/random",
			want: "",
		},
		{
			name: "repos with subdir",
			cwd:  "/home/user/eng/repos/lux/internal/mux",
			want: "lux",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := repoFromCWD(tt.cwd)
			if got != tt.want {
				t.Errorf("repoFromCWD(%q) = %q, want %q", tt.cwd, got, tt.want)
			}
		})
	}
}
