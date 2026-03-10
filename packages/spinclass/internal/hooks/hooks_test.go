package hooks

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeInput(toolName string, toolInput map[string]any, cwd string) []byte {
	input := map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       toolName,
		"tool_input":      toolInput,
		"cwd":             cwd,
	}
	data, _ := json.Marshal(input)
	return data
}

func TestDisallowMainWorktreeOffAllowsEverything(t *testing.T) {
	mainRepo := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(mainRepo, "secret.go")
	input := makeInput("Read", map[string]any{"file_path": target}, outside)
	var stdout bytes.Buffer
	err := Run(bytes.NewReader(input), &stdout, mainRepo, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("expected no output when flag is off, got %q", stdout.String())
	}
}

func TestDisallowMainWorktreeOnDeniesMainRepoPath(t *testing.T) {
	mainRepo := t.TempDir()
	worktreeCwd := t.TempDir()
	target := filepath.Join(mainRepo, "main.go")
	input := makeInput("Read", map[string]any{"file_path": target}, worktreeCwd)
	var stdout bytes.Buffer
	err := Run(bytes.NewReader(input), &stdout, mainRepo, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.Len() == 0 {
		t.Fatal("expected deny output for path in main worktree")
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON, got %q: %v", stdout.String(), err)
	}
	hso, ok := result["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("expected hookSpecificOutput in output")
	}
	if hso["permissionDecision"] != "deny" {
		t.Errorf("expected permissionDecision deny, got %v", hso["permissionDecision"])
	}
	reason, ok := hso["permissionDecisionReason"].(string)
	if !ok || reason == "" {
		t.Fatal("expected permissionDecisionReason in output")
	}
	if !strings.Contains(reason, "main worktree") {
		t.Errorf("expected permissionDecisionReason to mention main worktree, got %q", reason)
	}
}

func TestDisallowMainWorktreeOnAllowsWorktreePath(t *testing.T) {
	mainRepo := t.TempDir()
	worktreeCwd := t.TempDir()
	target := filepath.Join(worktreeCwd, "file.go")
	input := makeInput("Read", map[string]any{"file_path": target}, worktreeCwd)
	var stdout bytes.Buffer
	err := Run(bytes.NewReader(input), &stdout, mainRepo, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("expected no output for worktree path, got %q", stdout.String())
	}
}

func TestDisallowMainWorktreeOnAllowsUnrelatedPath(t *testing.T) {
	mainRepo := t.TempDir()
	worktreeCwd := t.TempDir()
	unrelated := t.TempDir()
	target := filepath.Join(unrelated, "file.go")
	input := makeInput("Read", map[string]any{"file_path": target}, worktreeCwd)
	var stdout bytes.Buffer
	err := Run(bytes.NewReader(input), &stdout, mainRepo, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("expected no output for unrelated path, got %q", stdout.String())
	}
}

func TestDisallowMainWorktreeEmptyMainRepoAllows(t *testing.T) {
	worktreeCwd := t.TempDir()
	target := filepath.Join(worktreeCwd, "file.go")
	input := makeInput("Read", map[string]any{"file_path": target}, worktreeCwd)
	var stdout bytes.Buffer
	err := Run(bytes.NewReader(input), &stdout, "", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("expected no output with empty main repo, got %q", stdout.String())
	}
}

func TestDisallowMainWorktreeGlobInMainRepo(t *testing.T) {
	mainRepo := t.TempDir()
	worktreeCwd := t.TempDir()
	input := makeInput("Glob", map[string]any{"path": mainRepo}, worktreeCwd)
	var stdout bytes.Buffer
	err := Run(bytes.NewReader(input), &stdout, mainRepo, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.Len() == 0 {
		t.Fatal("expected deny output for Glob targeting main worktree")
	}
}

func TestDisallowMainWorktreeBashAbsolutePathInMainRepo(t *testing.T) {
	mainRepo := t.TempDir()
	worktreeCwd := t.TempDir()
	target := filepath.Join(mainRepo, "src/main.go")
	input := makeInput("Bash", map[string]any{"command": "cat " + target}, worktreeCwd)
	var stdout bytes.Buffer
	err := Run(bytes.NewReader(input), &stdout, mainRepo, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.Len() == 0 {
		t.Fatal("expected deny output for Bash command targeting main worktree")
	}
}

func TestDisallowMainWorktreeSymlinkResolution(t *testing.T) {
	mainRepo := t.TempDir()
	worktreeCwd := t.TempDir()
	target := filepath.Join(mainRepo, "real.go")
	os.WriteFile(target, []byte("package main"), 0o644)
	link := filepath.Join(worktreeCwd, "link.go")
	os.Symlink(target, link)
	input := makeInput("Read", map[string]any{"file_path": link}, worktreeCwd)
	var stdout bytes.Buffer
	err := Run(bytes.NewReader(input), &stdout, mainRepo, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.Len() == 0 {
		t.Fatal("expected deny output for symlink resolving to main worktree")
	}
}

func TestDisallowMainWorktreeNonExistentFileInMainRepo(t *testing.T) {
	mainRepo := t.TempDir()
	worktreeCwd := t.TempDir()
	subdir := filepath.Join(mainRepo, "src")
	os.MkdirAll(subdir, 0o755)
	target := filepath.Join(subdir, "new.go")
	input := makeInput("Write", map[string]any{"file_path": target}, worktreeCwd)
	var stdout bytes.Buffer
	err := Run(bytes.NewReader(input), &stdout, mainRepo, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.Len() == 0 {
		t.Fatal("expected deny output for new file targeting main worktree")
	}
}

func TestStopHookEventRouteApproves(t *testing.T) {
	input, _ := json.Marshal(map[string]any{
		"hook_event_name": "Stop",
		"session_id":      "test-session-123",
		"cwd":             t.TempDir(),
	})

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No stop-hook configured -> approve (no output)
	if out.Len() != 0 {
		t.Errorf("expected no output for Stop with no stop-hook, got %q", out.String())
	}
}

func TestStopHookBlocksOnFailure(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TMPDIR", tmpDir)

	// Create a sweatfile with a failing stop-hook
	cwd := t.TempDir()
	os.WriteFile(filepath.Join(cwd, "sweatfile"), []byte("[hooks]\nstop = \"false\""), 0o644)

	input, _ := json.Marshal(map[string]any{
		"hook_event_name": "Stop",
		"session_id":      "block-test-session",
		"cwd":             cwd,
	})

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() == 0 {
		t.Fatal("expected block output for failing stop-hook")
	}

	var result map[string]any
	json.Unmarshal(out.Bytes(), &result)
	if result["decision"] != "block" {
		t.Errorf("expected block decision, got %v", result["decision"])
	}

	// Sentinel file should exist
	sentinel := filepath.Join(tmpDir, "stop-hook-block-test-session")
	if _, err := os.Stat(sentinel); os.IsNotExist(err) {
		t.Error("expected sentinel file to be created")
	}
}

func TestStopHookApprovesOnSecondInvocation(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TMPDIR", tmpDir)

	cwd := t.TempDir()
	os.WriteFile(filepath.Join(cwd, "sweatfile"), []byte("[hooks]\nstop = \"false\""), 0o644)

	// Create sentinel file (simulating first invocation already happened)
	sentinel := filepath.Join(tmpDir, "stop-hook-approve-test-session")
	os.WriteFile(sentinel, []byte("previous failure output"), 0o644)

	input, _ := json.Marshal(map[string]any{
		"hook_event_name": "Stop",
		"session_id":      "approve-test-session",
		"cwd":             cwd,
	})

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Sentinel exists -> approve (no output)
	if out.Len() != 0 {
		t.Errorf("expected no output on second invocation, got %q", out.String())
	}
}

func TestStopHookApprovesOnSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TMPDIR", tmpDir)

	cwd := t.TempDir()
	os.WriteFile(filepath.Join(cwd, "sweatfile"), []byte("[hooks]\nstop = \"true\""), 0o644)

	input, _ := json.Marshal(map[string]any{
		"hook_event_name": "Stop",
		"session_id":      "success-test-session",
		"cwd":             cwd,
	})

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() != 0 {
		t.Errorf("expected no output for passing stop-hook, got %q", out.String())
	}

	// No sentinel should exist on success
	sentinel := filepath.Join(tmpDir, "stop-hook-success-test-session")
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Error("expected no sentinel file for successful stop-hook")
	}
}
