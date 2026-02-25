package shop

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tap "github.com/amarbel-llc/tap-dancer/go"
	"github.com/amarbel-llc/spinclass/internal/worktree"
)

func TestStatusDescription(t *testing.T) {
	tests := []struct {
		name          string
		defaultBranch string
		commitsAhead  int
		porcelain     string
		want          string
	}{
		{
			name:          "ahead and clean",
			defaultBranch: "master",
			commitsAhead:  3,
			porcelain:     "",
			want:          "3 commits ahead of master, clean",
		},
		{
			name:          "one commit ahead",
			defaultBranch: "master",
			commitsAhead:  1,
			porcelain:     "",
			want:          "1 commit ahead of master, clean",
		},
		{
			name:          "ahead and dirty",
			defaultBranch: "main",
			commitsAhead:  2,
			porcelain:     "M file.go\n",
			want:          "2 commits ahead of main, dirty",
		},
		{
			name:          "merged",
			defaultBranch: "master",
			commitsAhead:  0,
			porcelain:     "",
			want:          "0 commits ahead of master, clean, (merged)",
		},
		{
			name:          "zero ahead but dirty",
			defaultBranch: "master",
			commitsAhead:  0,
			porcelain:     "?? untracked\n",
			want:          "0 commits ahead of master, dirty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := statusDescription(tt.defaultBranch, tt.commitsAhead, tt.porcelain)
			if got != tt.want {
				t.Errorf("statusDescription() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCreateTapNewWorktreeErrorPath(t *testing.T) {
	dir := t.TempDir()
	worktreePath := filepath.Join(dir, "new-worktree")

	rp := worktree.ResolvedPath{
		AbsPath:  worktreePath,
		RepoPath: dir,
		Branch:   "feature-x",
	}

	var buf bytes.Buffer
	err := Create(&buf, rp, false, "tap", nil)
	if err == nil {
		t.Error("expected error when creating worktree in non-git dir, got nil")
	}
}

func TestCreateTapSkipExisting(t *testing.T) {
	dir := t.TempDir()

	rp := worktree.ResolvedPath{
		AbsPath:  dir,
		RepoPath: dir,
		Branch:   "feature-x",
	}

	var buf bytes.Buffer
	if err := Create(&buf, rp, false, "tap", nil); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "# SKIP already exists") {
		t.Errorf("expected SKIP line, got: %q", got)
	}
}

func TestCreateTapNewWorktree(t *testing.T) {
	parentDir := t.TempDir()
	repoDir := filepath.Join(parentDir, "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	runGit("init")
	runGit("config", "user.email", "test@test.com")
	runGit("config", "user.name", "Test")
	runGit("commit", "--allow-empty", "-m", "initial")

	worktreePath := filepath.Join(parentDir, "spinclass-test-wt")

	rp := worktree.ResolvedPath{
		AbsPath:  worktreePath,
		RepoPath: repoDir,
		Branch:   "feature-wt",
	}

	var buf bytes.Buffer
	if err := Create(&buf, rp, false, "tap", nil); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "ok 1 - create") {
		t.Errorf("expected ok line, got: %q", got)
	}
	if !strings.Contains(got, "1..1") {
		t.Errorf("expected plan line 1..1, got: %q", got)
	}
}

func TestCreateSharedWriter(t *testing.T) {
	dir := t.TempDir()

	rp := worktree.ResolvedPath{
		AbsPath:  dir,
		RepoPath: dir,
		Branch:   "feature-x",
	}

	var buf bytes.Buffer
	tw := tap.NewWriter(&buf)
	tw.PlanAhead(2)

	if err := Create(&buf, rp, false, "tap", tw); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	got := buf.String()
	// Should use the shared writer's counter (test point 1)
	if !strings.Contains(got, "ok 1 - create feature-x # SKIP") {
		t.Errorf("expected ok 1 with SKIP, got: %q", got)
	}
	// Should NOT contain a second "TAP version 14" line
	if strings.Count(got, "TAP version 14") != 1 {
		t.Errorf("expected exactly one TAP version line, got: %q", got)
	}
}

type mockExecutor struct {
	attachCalled bool
}

func (m *mockExecutor) Attach(dir string, key string, command []string, dryRun bool, tp *tap.TestPoint) error {
	m.attachCalled = true
	if dryRun {
		tp.Skip = "dry run"
		tp.Diagnostics = &tap.Diagnostics{
			Extras: map[string]any{"command": "mock-command"},
		}
	}
	return nil
}

func (m *mockExecutor) Detach() error {
	return nil
}

func TestNewTapExistingWorktree(t *testing.T) {
	parentDir := t.TempDir()
	repoDir := filepath.Join(parentDir, "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	runGit := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	runGit(repoDir, "init")
	runGit(repoDir, "config", "user.email", "test@test.com")
	runGit(repoDir, "config", "user.name", "Test")
	runGit(repoDir, "commit", "--allow-empty", "-m", "initial")

	// Create worktree so attach finds it existing
	wtDir := filepath.Join(repoDir, ".worktrees")
	wtPath := filepath.Join(wtDir, "feature-tap")
	runGit(repoDir, "worktree", "add", "-b", "feature-tap", wtPath)

	rp := worktree.ResolvedPath{
		AbsPath:    wtPath,
		RepoPath:   repoDir,
		Branch:     "feature-tap",
		SessionKey: "repo/feature-tap",
	}

	mock := &mockExecutor{}
	var buf bytes.Buffer
	err := New(&buf, mock, rp, "tap", nil, true, false, false)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if !mock.attachCalled {
		t.Error("expected executor.Attach to be called")
	}

	got := buf.String()

	// Single TAP version line
	if strings.Count(got, "TAP version 14") != 1 {
		t.Errorf("expected exactly one TAP version line, got: %q", got)
	}

	// Plan
	if !strings.Contains(got, "1..2") {
		t.Errorf("expected plan 1..2, got: %q", got)
	}

	// Create test point (existing worktree -> SKIP)
	if !strings.Contains(got, "ok 1 - create feature-tap # SKIP") {
		t.Errorf("expected create SKIP test point, got: %q", got)
	}

	// Close test point
	if !strings.Contains(got, "ok 2 - close feature-tap") {
		t.Errorf("expected close test point, got: %q", got)
	}
}

func TestNewNoAttach(t *testing.T) {
	parentDir := t.TempDir()
	repoDir := filepath.Join(parentDir, "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	runGit := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	runGit(repoDir, "init")
	runGit(repoDir, "config", "user.email", "test@test.com")
	runGit(repoDir, "config", "user.name", "Test")
	runGit(repoDir, "commit", "--allow-empty", "-m", "initial")

	wtDir := filepath.Join(repoDir, ".worktrees")
	wtPath := filepath.Join(wtDir, "feature-dry")
	runGit(repoDir, "worktree", "add", "-b", "feature-dry", wtPath)

	rp := worktree.ResolvedPath{
		AbsPath:    wtPath,
		RepoPath:   repoDir,
		Branch:     "feature-dry",
		SessionKey: "repo/feature-dry",
	}

	mock := &mockExecutor{}
	var buf bytes.Buffer
	err := New(&buf, mock, rp, "tap", nil, false, true, false)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	got := buf.String()

	// Single TAP version line
	if strings.Count(got, "TAP version 14") != 1 {
		t.Errorf("expected exactly one TAP version line, got: %q", got)
	}

	// Create test point (existing worktree -> SKIP)
	if !strings.Contains(got, "ok 1 - create feature-dry # SKIP") {
		t.Errorf("expected create SKIP test point, got: %q", got)
	}

	// Attach test point (dry run -> SKIP with command diagnostic)
	if !strings.Contains(got, "ok 2 - attach feature-dry # SKIP dry run") {
		t.Errorf("expected attach SKIP test point, got: %q", got)
	}
	if !strings.Contains(got, "command: mock-command") {
		t.Errorf("expected command diagnostic, got: %q", got)
	}

	// Trailing plan
	if !strings.Contains(got, "1..2") {
		t.Errorf("expected plan 1..2, got: %q", got)
	}
}

func TestForkAutoName(t *testing.T) {
	parentDir := t.TempDir()
	repoDir := filepath.Join(parentDir, "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit(repoDir, "init")
	runGit(repoDir, "config", "user.email", "test@test.com")
	runGit(repoDir, "config", "user.name", "Test")
	runGit(repoDir, "commit", "--allow-empty", "-m", "initial")

	// Create the source worktree inside .worktrees/
	wtDir := filepath.Join(repoDir, ".worktrees")
	srcPath := filepath.Join(wtDir, "source-branch")
	runGit(repoDir, "worktree", "add", "-b", "source-branch", srcPath)

	rp := worktree.ResolvedPath{
		AbsPath:  srcPath,
		RepoPath: repoDir,
		Branch:   "source-branch",
	}

	var buf bytes.Buffer
	err := Fork(&buf, rp, "", "tap")

	if err != nil {
		t.Fatalf("Fork() error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "ok 1 - fork source-branch-1") {
		t.Errorf("expected ok line with fork name, got: %q", got)
	}

	// Forked worktree should exist
	forkedPath := filepath.Join(wtDir, "source-branch-1")
	if _, err := os.Stat(forkedPath); os.IsNotExist(err) {
		t.Errorf("expected forked worktree at %s", forkedPath)
	}
}
