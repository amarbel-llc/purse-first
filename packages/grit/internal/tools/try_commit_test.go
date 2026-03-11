package tools

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/friedenberg/grit/internal/git"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "commit.gpgSign", "false"},
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

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func initialCommit(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, dir, ".gitkeep", "")

	cmds := [][]string{
		{"git", "add", ".gitkeep"},
		{"git", "commit", "-m", "initial"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}
}

func TestHandleTryCommit(t *testing.T) {
	dir := setupTestRepo(t)
	writeFile(t, dir, "hello.txt", "hello world\n")

	args, _ := json.Marshal(map[string]any{
		"repo_path": dir,
		"message":   "add hello",
		"paths":     []string{"hello.txt"},
	})

	result, err := handleTryCommit(context.Background(), args, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsErr {
		t.Fatalf("result is error: %s", result.Text)
	}

	tcr, ok := result.JSON.(git.TryCommitResult)
	if !ok {
		t.Fatalf("unexpected result type: %T", result.JSON)
	}

	if tcr.Commit.Status != "committed" {
		t.Errorf("commit.status = %q, want %q", tcr.Commit.Status, "committed")
	}

	if tcr.Commit.Subject != "add hello" {
		t.Errorf("commit.subject = %q, want %q", tcr.Commit.Subject, "add hello")
	}

	if len(tcr.Staged) != 1 {
		t.Fatalf("staged len = %d, want 1", len(tcr.Staged))
	}

	if tcr.Staged[0].Path != "hello.txt" {
		t.Errorf("staged[0].path = %q, want %q", tcr.Staged[0].Path, "hello.txt")
	}

	if len(tcr.Status.Entries) != 0 {
		t.Errorf("status.entries len = %d, want 0 (clean after commit)", len(tcr.Status.Entries))
	}
}

func TestHandleTryCommitWithRemainingChanges(t *testing.T) {
	dir := setupTestRepo(t)
	writeFile(t, dir, "a.txt", "aaa\n")
	writeFile(t, dir, "b.txt", "bbb\n")

	args, _ := json.Marshal(map[string]any{
		"repo_path": dir,
		"message":   "add a only",
		"paths":     []string{"a.txt"},
	})

	result, err := handleTryCommit(context.Background(), args, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tcr, ok := result.JSON.(git.TryCommitResult)
	if !ok {
		t.Fatalf("unexpected result type: %T", result.JSON)
	}

	if tcr.Commit.Status != "committed" {
		t.Errorf("commit.status = %q, want %q", tcr.Commit.Status, "committed")
	}

	// b.txt should appear as untracked in post-commit status
	found := false
	for _, e := range tcr.Status.Entries {
		if e.Path == "b.txt" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected b.txt in post-commit status entries, got: %v", tcr.Status.Entries)
	}
}

func TestHandleTryCommitBadPaths(t *testing.T) {
	dir := setupTestRepo(t)

	args, _ := json.Marshal(map[string]any{
		"repo_path": dir,
		"message":   "commit nothing",
		"paths":     []string{"nonexistent.txt"},
	})

	result, err := handleTryCommit(context.Background(), args, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsErr {
		t.Errorf("expected error result for nonexistent paths")
	}
}

func TestHandleTryCommitNothingToCommit(t *testing.T) {
	dir := setupTestRepo(t)
	initialCommit(t, dir)
	writeFile(t, dir, "a.txt", "aaa\n")

	// Create initial commit with a.txt
	cmd := exec.Command("git", "add", "a.txt")
	cmd.Dir = dir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "add a")
	cmd.Dir = dir
	cmd.Run()

	// Try to commit a.txt again with no changes — commit should fail
	args, _ := json.Marshal(map[string]any{
		"repo_path": dir,
		"message":   "empty commit",
		"paths":     []string{"a.txt"},
	})

	result, err := handleTryCommit(context.Background(), args, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Spec: commit failure returns JSONResult (not error) with empty Commit
	if result.IsErr {
		t.Fatalf("expected structured result, got error: %s", result.Text)
	}

	tcr, ok := result.JSON.(git.TryCommitResult)
	if !ok {
		t.Fatalf("unexpected result type: %T", result.JSON)
	}

	// Commit field should be zero-value (empty)
	if tcr.Commit.Status != "" {
		t.Errorf("commit.status = %q, want empty", tcr.Commit.Status)
	}

	// Status should still be populated
	if tcr.Status.Branch.Head == "" {
		t.Errorf("expected status.branch.head to be populated")
	}
}
