package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initTestRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	return dir
}

func TestDetectInProgressStateClean(t *testing.T) {
	repo := initTestRepo(t)

	state, err := DetectInProgressState(context.Background(), repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state != nil {
		t.Errorf("expected nil state for clean repo, got %+v", state)
	}
}

func TestDetectInProgressStateRebaseMerge(t *testing.T) {
	repo := initTestRepo(t)
	ctx := context.Background()

	gitDir, err := resolveGitDir(ctx, repo)
	if err != nil {
		t.Fatalf("resolveGitDir: %v", err)
	}

	rebaseDir := filepath.Join(gitDir, "rebase-merge")
	if err := os.MkdirAll(rebaseDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	os.WriteFile(filepath.Join(rebaseDir, "head-name"), []byte("refs/heads/feature\n"), 0o644)
	os.WriteFile(filepath.Join(rebaseDir, "msgnum"), []byte("3\n"), 0o644)
	os.WriteFile(filepath.Join(rebaseDir, "end"), []byte("10\n"), 0o644)

	state, err := DetectInProgressState(ctx, repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state == nil {
		t.Fatal("expected non-nil state")
	}

	if state.Operation != "rebase" {
		t.Errorf("operation = %q, want %q", state.Operation, "rebase")
	}

	if state.Branch != "feature" {
		t.Errorf("branch = %q, want %q", state.Branch, "feature")
	}

	if state.Step != "3/10" {
		t.Errorf("step = %q, want %q", state.Step, "3/10")
	}
}

func TestDetectInProgressStateRebaseApply(t *testing.T) {
	repo := initTestRepo(t)
	ctx := context.Background()

	gitDir, err := resolveGitDir(ctx, repo)
	if err != nil {
		t.Fatalf("resolveGitDir: %v", err)
	}

	rebaseDir := filepath.Join(gitDir, "rebase-apply")
	if err := os.MkdirAll(rebaseDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	os.WriteFile(filepath.Join(rebaseDir, "msgnum"), []byte("1\n"), 0o644)
	os.WriteFile(filepath.Join(rebaseDir, "end"), []byte("5\n"), 0o644)

	state, err := DetectInProgressState(ctx, repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state == nil {
		t.Fatal("expected non-nil state")
	}

	if state.Operation != "rebase" {
		t.Errorf("operation = %q, want %q", state.Operation, "rebase")
	}

	if state.Branch != "" {
		t.Errorf("branch = %q, want empty", state.Branch)
	}

	if state.Step != "1/5" {
		t.Errorf("step = %q, want %q", state.Step, "1/5")
	}
}

func TestDetectInProgressStateMerge(t *testing.T) {
	repo := initTestRepo(t)
	ctx := context.Background()

	gitDir, err := resolveGitDir(ctx, repo)
	if err != nil {
		t.Fatalf("resolveGitDir: %v", err)
	}

	os.WriteFile(filepath.Join(gitDir, "MERGE_HEAD"), []byte("abc123\n"), 0o644)

	state, err := DetectInProgressState(ctx, repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state == nil {
		t.Fatal("expected non-nil state")
	}

	if state.Operation != "merge" {
		t.Errorf("operation = %q, want %q", state.Operation, "merge")
	}
}

func TestDetectInProgressStateCherryPick(t *testing.T) {
	repo := initTestRepo(t)
	ctx := context.Background()

	gitDir, err := resolveGitDir(ctx, repo)
	if err != nil {
		t.Fatalf("resolveGitDir: %v", err)
	}

	os.WriteFile(filepath.Join(gitDir, "CHERRY_PICK_HEAD"), []byte("abc123\n"), 0o644)

	state, err := DetectInProgressState(ctx, repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state == nil {
		t.Fatal("expected non-nil state")
	}

	if state.Operation != "cherry-pick" {
		t.Errorf("operation = %q, want %q", state.Operation, "cherry-pick")
	}
}

func TestDetectInProgressStateRevert(t *testing.T) {
	repo := initTestRepo(t)
	ctx := context.Background()

	gitDir, err := resolveGitDir(ctx, repo)
	if err != nil {
		t.Fatalf("resolveGitDir: %v", err)
	}

	os.WriteFile(filepath.Join(gitDir, "REVERT_HEAD"), []byte("abc123\n"), 0o644)

	state, err := DetectInProgressState(ctx, repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state == nil {
		t.Fatal("expected non-nil state")
	}

	if state.Operation != "revert" {
		t.Errorf("operation = %q, want %q", state.Operation, "revert")
	}
}
