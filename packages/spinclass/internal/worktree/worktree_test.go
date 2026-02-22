package worktree

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePathBranchName(t *testing.T) {
	home := t.TempDir()
	repoPath := filepath.Join(home, "repos", "myrepo")

	rp, err := ResolvePath(repoPath, "feature-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantAbs := filepath.Join(repoPath, ".worktrees", "feature-x")
	if rp.AbsPath != wantAbs {
		t.Errorf("AbsPath = %q, want %q", rp.AbsPath, wantAbs)
	}
	if rp.Branch != "feature-x" {
		t.Errorf("Branch = %q, want %q", rp.Branch, "feature-x")
	}
	if rp.RepoPath != repoPath {
		t.Errorf("RepoPath = %q, want %q", rp.RepoPath, repoPath)
	}
}

func TestResolvePathRelativePath(t *testing.T) {
	home := t.TempDir()
	repoPath := filepath.Join(home, "repos", "myrepo")

	rp, err := ResolvePath(repoPath, ".worktrees/feature-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantAbs := filepath.Join(repoPath, ".worktrees", "feature-x")
	if rp.AbsPath != wantAbs {
		t.Errorf("AbsPath = %q, want %q", rp.AbsPath, wantAbs)
	}
	if rp.Branch != "feature-x" {
		t.Errorf("Branch = %q, want %q", rp.Branch, "feature-x")
	}
}

func TestResolvePathAbsolutePath(t *testing.T) {
	home := t.TempDir()
	repoPath := filepath.Join(home, "repos", "myrepo")
	absTarget := "/tmp/my-custom-worktree"

	rp, err := ResolvePath(repoPath, absTarget)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rp.AbsPath != absTarget {
		t.Errorf("AbsPath = %q, want %q", rp.AbsPath, absTarget)
	}
	if rp.Branch != "my-custom-worktree" {
		t.Errorf("Branch = %q, want %q", rp.Branch, "my-custom-worktree")
	}
	if rp.RepoPath != repoPath {
		t.Errorf("RepoPath = %q, want %q", rp.RepoPath, repoPath)
	}
}

func TestResolvePathSessionKey(t *testing.T) {
	home := t.TempDir()
	repoPath := filepath.Join(home, "repos", "myrepo")

	rp, err := ResolvePath(repoPath, "feature-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantKey := "myrepo/feature-x"
	if rp.SessionKey != wantKey {
		t.Errorf("SessionKey = %q, want %q", rp.SessionKey, wantKey)
	}
}

func TestResolvePathSessionKeyAbsolutePath(t *testing.T) {
	home := t.TempDir()
	repoPath := filepath.Join(home, "repos", "dodder")

	rp, err := ResolvePath(repoPath, "/tmp/custom-wt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantKey := "dodder/custom-wt"
	if rp.SessionKey != wantKey {
		t.Errorf("SessionKey = %q, want %q", rp.SessionKey, wantKey)
	}
}

func TestDetectRepoFindsGitDir(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "myrepo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	subDir := filepath.Join(repoDir, "src", "pkg")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := DetectRepo(subDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != repoDir {
		t.Errorf("DetectRepo() = %q, want %q", got, repoDir)
	}
}

func TestDetectRepoSkipsGitFile(t *testing.T) {
	root := t.TempDir()
	// Create a parent repo with a .git directory
	parentRepo := filepath.Join(root, "parent")
	if err := os.MkdirAll(filepath.Join(parentRepo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Create a worktree-like child with a .git file (not directory)
	child := filepath.Join(parentRepo, "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, ".git"), []byte("gitdir: ../parent/.git/worktrees/child"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := DetectRepo(child)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != parentRepo {
		t.Errorf("DetectRepo() = %q, want %q (should skip .git file and find parent)", got, parentRepo)
	}
}

func TestDetectRepoFailsOutsideRepo(t *testing.T) {
	dir := t.TempDir()

	_, err := DetectRepo(dir)
	if err == nil {
		t.Error("expected error when no git repo found, got nil")
	}
}

func TestScanReposFromRepo(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "myrepo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, WorktreesDir), 0o755); err != nil {
		t.Fatal(err)
	}

	repos := ScanRepos(repoDir)
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}
	if repos[0] != repoDir {
		t.Errorf("repos[0] = %q, want %q", repos[0], repoDir)
	}
}

func TestScanReposFromParent(t *testing.T) {
	root := t.TempDir()

	// Create two repos with .worktrees
	for _, name := range []string{"repo-a", "repo-b"} {
		repoDir := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(repoDir, WorktreesDir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Create a repo without .worktrees (should be excluded)
	noWtRepo := filepath.Join(root, "repo-c")
	if err := os.MkdirAll(filepath.Join(noWtRepo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	repos := ScanRepos(root)
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d: %v", len(repos), repos)
	}

	found := make(map[string]bool)
	for _, r := range repos {
		found[filepath.Base(r)] = true
	}
	if !found["repo-a"] || !found["repo-b"] {
		t.Errorf("expected repo-a and repo-b, got %v", repos)
	}
}

func TestScanReposEmpty(t *testing.T) {
	root := t.TempDir()

	repos := ScanRepos(root)
	if len(repos) != 0 {
		t.Errorf("expected 0 repos, got %d: %v", len(repos), repos)
	}
}

func TestListWorktrees(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "myrepo")
	wtDir := filepath.Join(repoDir, WorktreesDir)

	branches := []string{"feature-a", "feature-b", "bugfix-1"}
	for _, b := range branches {
		branchDir := filepath.Join(wtDir, b)
		if err := os.MkdirAll(branchDir, 0o755); err != nil {
			t.Fatal(err)
		}
		// Create .git file to mark as worktree
		if err := os.WriteFile(filepath.Join(branchDir, ".git"), []byte("gitdir: ../../../.git/worktrees/"+b+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Also create a file (should be excluded)
	if err := os.WriteFile(filepath.Join(wtDir, "not-a-dir"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a plain directory (not a worktree — no .git file)
	if err := os.MkdirAll(filepath.Join(wtDir, "not-a-worktree"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := ListWorktrees(repoDir)
	if len(got) != 3 {
		t.Fatalf("expected 3 worktrees, got %d: %v", len(got), got)
	}

	found := make(map[string]bool)
	for _, wt := range got {
		found[filepath.Base(wt)] = true
		if !filepath.IsAbs(wt) {
			t.Errorf("expected absolute path, got %q", wt)
		}
	}
	for _, b := range branches {
		if !found[b] {
			t.Errorf("missing worktree %q in results %v", b, got)
		}
	}
}

func TestListWorktreesEmpty(t *testing.T) {
	root := t.TempDir()

	got := ListWorktrees(root)
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestIsWorktreeWithGitFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: somewhere"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !IsWorktree(dir) {
		t.Error("expected IsWorktree=true for .git file")
	}
}

func TestIsWorktreeWithGitDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	if IsWorktree(dir) {
		t.Error("expected IsWorktree=false for .git directory")
	}
}

func TestIsWorktreeNoGit(t *testing.T) {
	dir := t.TempDir()

	if IsWorktree(dir) {
		t.Error("expected IsWorktree=false for directory without .git")
	}
}

func TestExcludeWorktreesDir(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "myrepo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git", "info"), 0o755); err != nil {
		t.Fatal(err)
	}

	// First call should add the entry
	if err := excludeWorktreesDir(repoDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repoDir, ".git", "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != ".worktrees\n" {
		t.Errorf("expected '.worktrees\\n', got %q", string(data))
	}

	// Second call should be idempotent
	if err := excludeWorktreesDir(repoDir); err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}

	data, err = os.ReadFile(filepath.Join(repoDir, ".git", "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != ".worktrees\n" {
		t.Errorf("expected idempotent result '.worktrees\\n', got %q", string(data))
	}
}

func TestExcludeWorktreesDirCreatesInfoDir(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "myrepo")
	// Only create .git, not .git/info
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := excludeWorktreesDir(repoDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repoDir, ".git", "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != ".worktrees\n" {
		t.Errorf("expected '.worktrees\\n', got %q", string(data))
	}
}
