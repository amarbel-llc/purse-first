package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/amarbel-llc/spinclass/internal/sweatfile"
)

func TestResolvePathBranchName(t *testing.T) {
	home := t.TempDir()
	repoPath := filepath.Join(home, "repos", "myrepo")

	rp, err := ResolvePath(sweatfile.Sweatfile{}, repoPath, []string{"feature-x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantAbs := filepath.Join(repoPath, WorktreesDir, "feature_x")
	if rp.AbsPath != wantAbs {
		t.Errorf("AbsPath = %q, want %q", rp.AbsPath, wantAbs)
	}
	if rp.Branch != "feature_x" {
		t.Errorf("Branch = %q, want %q", rp.Branch, "feature_x")
	}
	if rp.RepoPath != repoPath {
		t.Errorf("RepoPath = %q, want %q", rp.RepoPath, repoPath)
	}
}

func TestResolvePathSessionKey(t *testing.T) {
	home := t.TempDir()
	repoPath := filepath.Join(home, "repos", "myrepo")

	rp, err := ResolvePath(sweatfile.Sweatfile{}, repoPath, []string{"feature-x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantKey := "myrepo/feature_x"
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
	root := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", root)

	dir := filepath.Join(root, "no-repo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

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

	// Create two repos with WorktreesDir
	for _, name := range []string{"repo-a", "repo-b"} {
		repoDir := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(repoDir, WorktreesDir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Create a repo without WorktreesDir (should be excluded)
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
	want := WorktreesDir + "\n"
	if string(data) != want {
		t.Errorf("expected %q, got %q", want, string(data))
	}

	// Second call should be idempotent
	if err := excludeWorktreesDir(repoDir); err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}

	data, err = os.ReadFile(filepath.Join(repoDir, ".git", "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Errorf("expected idempotent result %q, got %q", want, string(data))
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
	want := WorktreesDir + "\n"
	if string(data) != want {
		t.Errorf("expected %q, got %q", want, string(data))
	}
}

func TestForkName(t *testing.T) {
	dir := t.TempDir()
	wtDir := filepath.Join(dir, WorktreesDir)
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// No collisions: first fork is <branch>-1
	got := ForkName(dir, "my-feature")
	if got != "my-feature-1" {
		t.Errorf("ForkName() = %q, want %q", got, "my-feature-1")
	}

	// Create my-feature-1 dir, next should be my-feature-2
	if err := os.Mkdir(filepath.Join(wtDir, "my-feature-1"), 0o755); err != nil {
		t.Fatal(err)
	}
	got = ForkName(dir, "my-feature")
	if got != "my-feature-2" {
		t.Errorf("ForkName() = %q, want %q", got, "my-feature-2")
	}

	// Create my-feature-2 as well, next should be my-feature-3
	if err := os.Mkdir(filepath.Join(wtDir, "my-feature-2"), 0o755); err != nil {
		t.Fatal(err)
	}
	got = ForkName(dir, "my-feature")
	if got != "my-feature-3" {
		t.Errorf("ForkName() = %q, want %q", got, "my-feature-3")
	}
}

func TestResolvePathMultipleArgs(t *testing.T) {
	home := t.TempDir()
	repoPath := filepath.Join(home, "repos", "myrepo")

	rp, err := ResolvePath(sweatfile.Sweatfile{}, repoPath, []string{"this", "is", "the", "branch-name"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rp.Branch != "this-is-the-branch_name" {
		t.Errorf("Branch = %q, want %q", rp.Branch, "this-is-the-branch_name")
	}
	if rp.ExistingBranch != "" {
		t.Errorf("ExistingBranch = %q, want empty for new branch", rp.ExistingBranch)
	}
}

func TestResolvePathDetectsLocalBranch(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "myrepo")
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
	runGit(repoDir, "branch", "existing_branch")

	rp, err := ResolvePath(sweatfile.Sweatfile{}, repoDir, []string{"existing-branch"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rp.ExistingBranch != "existing_branch" {
		t.Errorf("ExistingBranch = %q, want %q", rp.ExistingBranch, "existing_branch")
	}
	if rp.Branch != "existing_branch" {
		t.Errorf("Branch = %q, want %q", rp.Branch, "existing_branch")
	}
}

func TestResolvePathDetectsRemoteBranch(t *testing.T) {
	root := t.TempDir()

	// Create a bare "remote" repo
	bareDir := filepath.Join(root, "remote.git")
	if err := os.MkdirAll(bareDir, 0o755); err != nil {
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
	runGit(bareDir, "init", "--bare")

	// Create a local repo, add the bare as origin
	repoDir := filepath.Join(root, "local")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(repoDir, "init")
	runGit(repoDir, "config", "user.email", "test@test.com")
	runGit(repoDir, "config", "user.name", "Test")
	runGit(repoDir, "commit", "--allow-empty", "-m", "initial")
	runGit(repoDir, "remote", "add", "origin", bareDir)

	// Push a branch to the remote, then delete it locally
	runGit(repoDir, "branch", "remote_only_branch")
	runGit(repoDir, "push", "origin", "remote_only_branch")
	runGit(repoDir, "branch", "-d", "remote_only_branch")
	runGit(repoDir, "fetch", "origin")

	rp, err := ResolvePath(sweatfile.Sweatfile{}, repoDir, []string{"remote-only-branch"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rp.ExistingBranch != "remote_only_branch" {
		t.Errorf("ExistingBranch = %q, want %q", rp.ExistingBranch, "remote_only_branch")
	}
	if rp.Branch != "remote_only_branch" {
		t.Errorf("Branch = %q, want %q", rp.Branch, "remote_only_branch")
	}
}

func TestResolvePathPrefersUnsanitizedBranch(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "myrepo")
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

	// Create a branch with the hyphenated name (as if created outside spinclass)
	runGit(repoDir, "branch", "quiet-pecan")

	// User passes "quiet-pecan" — should find the hyphenated branch,
	// NOT normalize to "quiet_pecan" and miss it.
	rp, err := ResolvePath(sweatfile.Sweatfile{}, repoDir, []string{"quiet-pecan"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rp.ExistingBranch != "quiet-pecan" {
		t.Errorf("ExistingBranch = %q, want %q", rp.ExistingBranch, "quiet-pecan")
	}
	if rp.Branch != "quiet-pecan" {
		t.Errorf("Branch = %q, want %q", rp.Branch, "quiet-pecan")
	}
	wantAbs := filepath.Join(repoDir, WorktreesDir, "quiet-pecan")
	if rp.AbsPath != wantAbs {
		t.Errorf("AbsPath = %q, want %q", rp.AbsPath, wantAbs)
	}
}

func TestResolvePathFallsBackToSanitizedBranch(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "myrepo")
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

	// Only the sanitized form exists
	runGit(repoDir, "branch", "quiet_pecan")

	// User passes "quiet-pecan" — unsanitized won't match, should fall back
	// to sanitized "quiet_pecan".
	rp, err := ResolvePath(sweatfile.Sweatfile{}, repoDir, []string{"quiet-pecan"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rp.ExistingBranch != "quiet_pecan" {
		t.Errorf("ExistingBranch = %q, want %q", rp.ExistingBranch, "quiet_pecan")
	}
	if rp.Branch != "quiet_pecan" {
		t.Errorf("Branch = %q, want %q", rp.Branch, "quiet_pecan")
	}
}

func TestResolvePathBothFormsExistPrefersUnsanitized(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "myrepo")
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

	// Both forms exist as branches
	runGit(repoDir, "branch", "quiet-pecan")
	runGit(repoDir, "branch", "quiet_pecan")

	// Should prefer the unsanitized (user's literal input) form
	rp, err := ResolvePath(sweatfile.Sweatfile{}, repoDir, []string{"quiet-pecan"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rp.ExistingBranch != "quiet-pecan" {
		t.Errorf("ExistingBranch = %q, want %q", rp.ExistingBranch, "quiet-pecan")
	}
	if rp.Branch != "quiet-pecan" {
		t.Errorf("Branch = %q, want %q", rp.Branch, "quiet-pecan")
	}
}

func TestResolvePathRandomNameWhenNoArgs(t *testing.T) {
	home := t.TempDir()
	repoPath := filepath.Join(home, "repos", "myrepo")

	rp, err := ResolvePath(sweatfile.Sweatfile{}, repoPath, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rp.Branch == "" {
		t.Error("expected non-empty random branch name")
	}
	if rp.ExistingBranch != "" {
		t.Errorf("ExistingBranch = %q, want empty for random name", rp.ExistingBranch)
	}
}

func TestCreateFrom(t *testing.T) {
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

	// Create a source worktree to fork from
	srcPath := filepath.Join(parentDir, "src-wt")
	runGit(repoDir, "worktree", "add", "-b", "source-branch", srcPath)

	newPath := filepath.Join(parentDir, "fork-wt")
	_, err := CreateFrom(repoDir, srcPath, newPath, "fork-branch")
	if err != nil {
		t.Fatalf("CreateFrom() error: %v", err)
	}

	// New worktree directory should exist
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Errorf("expected worktree at %s, not found", newPath)
	}

	// Should be a worktree (has .git file, not dir)
	if !IsWorktree(newPath) {
		t.Errorf("expected %s to be a worktree", newPath)
	}
}
