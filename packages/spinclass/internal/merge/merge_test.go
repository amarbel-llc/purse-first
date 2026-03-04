package merge

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tap "github.com/amarbel-llc/purse-first/packages/tap-dancer/go"
)

type mockExecutor struct {
	detachCalled bool
}

func (m *mockExecutor) Attach(dir string, key string, command []string, dryRun bool, tp *tap.TestPoint) error {
	return nil
}

func (m *mockExecutor) Detach() error {
	m.detachCalled = true
	return nil
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func setupRepo(t *testing.T) (repoDir string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", root)

	// Isolate git config to prevent interference from global settings
	gitConfigDir := filepath.Join(root, "gitconfig")
	if err := os.MkdirAll(gitConfigDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(gitConfigDir, "config"))
	t.Setenv("HOME", root)

	repoDir = filepath.Join(root, "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	runGit(t, repoDir, "init", "-b", "main")
	runGit(t, repoDir, "config", "user.email", "test@test.com")
	runGit(t, repoDir, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("initial"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoDir, "add", "file.txt")
	runGit(t, repoDir, "commit", "-m", "initial")

	return repoDir
}

func TestResolvedMergesAndRemovesWorktree(t *testing.T) {
	repoDir := setupRepo(t)

	// Create a worktree with a new commit
	wtDir := filepath.Join(repoDir, ".worktrees")
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wtPath := filepath.Join(wtDir, "feature-merge")
	runGit(t, repoDir, "worktree", "add", "-b", "feature-merge", wtPath)

	if err := os.WriteFile(filepath.Join(wtPath, "new.txt"), []byte("new content"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, wtPath, "add", "new.txt")
	runGit(t, wtPath, "commit", "-m", "add new file")

	mock := &mockExecutor{}
	var buf bytes.Buffer

	err := Resolved(mock, &buf, nil, "tap", repoDir, wtPath, "feature-merge", "main", false, false)
	if err != nil {
		t.Fatalf("Resolved() error: %v", err)
	}

	// Commit should be on main now
	mainLog := runGit(t, repoDir, "log", "--oneline")
	if !strings.Contains(mainLog, "add new file") {
		t.Errorf("expected commit on main, got: %s", mainLog)
	}

	// Worktree directory should be removed
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("expected worktree to be removed, but it still exists")
	}

	// Detach should have been called
	if !mock.detachCalled {
		t.Error("expected Detach() to be called")
	}

	// TAP output should contain all three steps
	got := buf.String()
	if !strings.Contains(got, "ok") {
		t.Errorf("expected TAP ok lines, got: %q", got)
	}
}

func TestResolvedTapOutput(t *testing.T) {
	repoDir := setupRepo(t)

	wtDir := filepath.Join(repoDir, ".worktrees")
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wtPath := filepath.Join(wtDir, "feature-tap")
	runGit(t, repoDir, "worktree", "add", "-b", "feature-tap", wtPath)

	if err := os.WriteFile(filepath.Join(wtPath, "tap.txt"), []byte("tap"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, wtPath, "add", "tap.txt")
	runGit(t, wtPath, "commit", "-m", "tap commit")

	mock := &mockExecutor{}
	var buf bytes.Buffer

	err := Resolved(mock, &buf, nil, "tap", repoDir, wtPath, "feature-tap", "main", false, false)
	if err != nil {
		t.Fatalf("Resolved() error: %v", err)
	}

	got := buf.String()

	if !strings.Contains(got, "ok 1 - rebase feature-tap") {
		t.Errorf("expected rebase test point, got: %q", got)
	}
	if !strings.Contains(got, "ok 2 - merge feature-tap") {
		t.Errorf("expected merge test point, got: %q", got)
	}
	if !strings.Contains(got, "ok 3 - remove worktree feature-tap") {
		t.Errorf("expected remove worktree test point, got: %q", got)
	}
	if !strings.Contains(got, "1..3") {
		t.Errorf("expected plan 1..3, got: %q", got)
	}
}

func TestResolvedGitSyncTapOutput(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", root)

	gitConfigDir := filepath.Join(root, "gitconfig")
	if err := os.MkdirAll(gitConfigDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(gitConfigDir, "config"))
	t.Setenv("HOME", root)

	// Create a bare remote repo
	bareDir := filepath.Join(root, "bare.git")
	runGit(t, root, "init", "--bare", "-b", "main", bareDir)

	// Clone it to get a repo with a remote
	repoDir := filepath.Join(root, "repo")
	runGit(t, root, "clone", bareDir, repoDir)
	runGit(t, repoDir, "config", "user.email", "test@test.com")
	runGit(t, repoDir, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("initial"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoDir, "add", "file.txt")
	runGit(t, repoDir, "commit", "-m", "initial")
	runGit(t, repoDir, "push")

	// Create a worktree with a commit
	wtDir := filepath.Join(repoDir, ".worktrees")
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wtPath := filepath.Join(wtDir, "feature-sync")
	runGit(t, repoDir, "worktree", "add", "-b", "feature-sync", wtPath)
	if err := os.WriteFile(filepath.Join(wtPath, "sync.txt"), []byte("sync"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, wtPath, "add", "sync.txt")
	runGit(t, wtPath, "commit", "-m", "sync commit")

	mock := &mockExecutor{}
	var buf bytes.Buffer

	err := Resolved(mock, &buf, nil, "tap", repoDir, wtPath, "feature-sync", "main", true, false)
	if err != nil {
		t.Fatalf("Resolved() error: %v", err)
	}

	got := buf.String()

	if !strings.Contains(got, "ok 1 - rebase feature-sync") {
		t.Errorf("expected rebase test point, got: %q", got)
	}
	if !strings.Contains(got, "ok 2 - merge feature-sync") {
		t.Errorf("expected merge test point, got: %q", got)
	}
	if !strings.Contains(got, "ok 3 - remove worktree feature-sync") {
		t.Errorf("expected remove worktree test point, got: %q", got)
	}
	if !strings.Contains(got, "ok 4 - pull") {
		t.Errorf("expected pull test point, got: %q", got)
	}
	if !strings.Contains(got, "ok 5 - push") {
		t.Errorf("expected push test point, got: %q", got)
	}
	if !strings.Contains(got, "1..5") {
		t.Errorf("expected plan 1..5, got: %q", got)
	}
}

func TestResolvedRepoNotFound(t *testing.T) {
	mock := &mockExecutor{}
	var buf bytes.Buffer

	err := Resolved(mock, &buf, nil, "tap", "/nonexistent/path", "/nonexistent/wt", "feature", "main", false, false)
	if err == nil {
		t.Error("expected error for nonexistent repo, got nil")
	}
	if !strings.Contains(err.Error(), "repository not found") {
		t.Errorf("expected 'repository not found' error, got: %v", err)
	}
}

func TestResolvedDivergedBranch(t *testing.T) {
	repoDir := setupRepo(t)

	wtDir := filepath.Join(repoDir, ".worktrees")
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wtPath := filepath.Join(wtDir, "feature-diverge")
	runGit(t, repoDir, "worktree", "add", "-b", "feature-diverge", wtPath)

	// Make a commit on the worktree
	if err := os.WriteFile(filepath.Join(wtPath, "diverge.txt"), []byte("diverge"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, wtPath, "add", "diverge.txt")
	runGit(t, wtPath, "commit", "-m", "diverge commit")

	// Make a conflicting commit on main
	if err := os.WriteFile(filepath.Join(repoDir, "diverge.txt"), []byte("conflict"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoDir, "add", "diverge.txt")
	runGit(t, repoDir, "commit", "-m", "conflicting commit on main")

	mock := &mockExecutor{}
	var buf bytes.Buffer

	err := Resolved(mock, &buf, nil, "tap", repoDir, wtPath, "feature-diverge", "main", false, false)
	if err == nil {
		t.Error("expected error for conflicting rebase, got nil")
	}

	// Abort the rebase to clean up
	exec.Command("git", "-C", wtPath, "rebase", "--abort").Run()
}
