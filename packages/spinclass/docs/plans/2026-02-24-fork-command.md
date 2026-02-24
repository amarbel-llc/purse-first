# Fork Command Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `spinclass fork [<new-branch>]` that creates a new worktree branched from the current worktree's HEAD, with optional auto-generated branch name, without attaching.

**Architecture:** Reads `$SPINCLASS_SESSION` to identify the current worktree. Auto-name generation scans `.worktrees/` for `<branch>-N` collisions. New worktree creation uses `git worktree add -b` from the current worktree path (not repo root) so HEAD is inherited. Reuses existing sweatfile/trust setup from `worktree.Create`.

**Tech Stack:** Go, cobra, `internal/worktree`, `internal/shop`, `internal/git`, `internal/tap`.

---

### Task 1: Add `ForkName` to worktree package

Generates a collision-free fork branch name by scanning `.worktrees/` for existing `<source>-N` directories.

**Files:**
- Modify: `packages/spinclass/internal/worktree/worktree.go`
- Test: `packages/spinclass/internal/worktree/worktree_test.go`

**Step 1: Write the failing tests**

Add to `packages/spinclass/internal/worktree/worktree_test.go`:

```go
func TestForkName(t *testing.T) {
	dir := t.TempDir()
	wtDir := filepath.Join(dir, ".worktrees")
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/worktree/... -run TestForkName -v`
Expected: FAIL — `ForkName` undefined

**Step 3: Implement `ForkName`**

Add to `packages/spinclass/internal/worktree/worktree.go`:

```go
// ForkName returns a collision-free branch name for forking sourceBranch.
// It tries <sourceBranch>-1, <sourceBranch>-2, etc., checking for existing
// directories in <repoPath>/.worktrees/.
func ForkName(repoPath, sourceBranch string) string {
	wtDir := filepath.Join(repoPath, WorktreesDir)
	for n := 1; ; n++ {
		candidate := fmt.Sprintf("%s-%d", sourceBranch, n)
		if _, err := os.Stat(filepath.Join(wtDir, candidate)); os.IsNotExist(err) {
			return candidate
		}
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/worktree/... -run TestForkName -v`
Expected: PASS

**Step 5: Commit**

```
git add packages/spinclass/internal/worktree/worktree.go packages/spinclass/internal/worktree/worktree_test.go
git commit -m "feat(spinclass): add ForkName for auto-generating fork branch names"
```

---

### Task 2: Add `CreateFrom` to worktree package

Runs `git worktree add -b <newBranch> <newPath>` from `fromPath` (the current worktree), then applies sweatfile and trusts the workspace.

**Files:**
- Modify: `packages/spinclass/internal/worktree/worktree.go`
- Modify: `packages/spinclass/internal/git/git.go`
- Test: `packages/spinclass/internal/worktree/worktree_test.go`

**Step 1: Add `WorktreeAddFrom` to git package**

The existing `git.RunPassthrough` takes a `-C <repoPath>` prefix. For fork we need to run `git worktree add -b` from the *worktree* directory (so HEAD points to that worktree's commit), not the repo root.

Add to `packages/spinclass/internal/git/git.go`:

```go
// WorktreeAddFrom runs `git -C fromPath worktree add -b newBranch newPath`
// so the new worktree branches from fromPath's current HEAD.
func WorktreeAddFrom(fromPath, newBranch, newPath string) error {
	return RunPassthrough(fromPath, "worktree", "add", "-b", newBranch, newPath)
}
```

**Step 2: Write the failing test for `CreateFrom`**

Add to `packages/spinclass/internal/worktree/worktree_test.go`:

```go
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
```

Note: `worktree_test.go` will need `"os/exec"` added to its imports if not already present.

**Step 3: Run test to verify it fails**

Run: `go test ./internal/worktree/... -run TestCreateFrom -v`
Expected: FAIL — `CreateFrom` undefined

**Step 4: Implement `CreateFrom`**

Add to `packages/spinclass/internal/worktree/worktree.go`:

```go
// CreateFrom creates a new worktree branched from fromPath's current HEAD.
// It runs git worktree add -b from fromPath, then applies sweatfile and
// trusts the workspace, same as Create.
func CreateFrom(repoPath, fromPath, newPath, newBranch string) (sweatfile.LoadResult, error) {
	if err := git.WorktreeAddFrom(fromPath, newBranch, newPath); err != nil {
		return sweatfile.LoadResult{}, fmt.Errorf("git worktree add: %w", err)
	}
	if err := excludeWorktreesDir(repoPath); err != nil {
		return sweatfile.LoadResult{}, fmt.Errorf("excluding .worktrees from git: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return sweatfile.LoadResult{}, fmt.Errorf("getting home directory: %w", err)
	}

	result, err := sweatfile.LoadHierarchy(home, repoPath)
	if err != nil {
		return sweatfile.LoadResult{}, fmt.Errorf("loading sweatfile: %w", err)
	}
	if err := sweatfile.Apply(newPath, result.Merged); err != nil {
		return sweatfile.LoadResult{}, err
	}

	claudeJSONPath := filepath.Join(home, ".claude.json")
	if err := claude.TrustWorkspace(claudeJSONPath, newPath); err != nil {
		return sweatfile.LoadResult{}, fmt.Errorf("trusting workspace in claude: %w", err)
	}

	return result, nil
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/worktree/... -run TestCreateFrom -v`
Expected: PASS

**Step 6: Run all worktree tests**

Run: `go test ./internal/worktree/... -v`
Expected: all PASS

**Step 7: Commit**

```
git add packages/spinclass/internal/git/git.go packages/spinclass/internal/worktree/worktree.go packages/spinclass/internal/worktree/worktree_test.go
git commit -m "feat(spinclass): add CreateFrom for forking worktrees from current HEAD"
```

---

### Task 3: Add `Fork` to shop package

Reads `$SPINCLASS_SESSION`, resolves current worktree, auto-generates branch name if needed, calls `CreateFrom`, and outputs TAP or plain path.

**Files:**
- Modify: `packages/spinclass/internal/shop/shop.go`
- Test: `packages/spinclass/internal/shop/shop_test.go`

**Step 1: Write the failing test**

Add to `packages/spinclass/internal/shop/shop_test.go`:

```go
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

	// Create the source worktree
	wtDir := filepath.Join(repoDir, ".worktrees")
	srcPath := filepath.Join(wtDir, "source-branch")
	runGit(repoDir, "worktree", "add", "-b", "source-branch", srcPath)

	rp := worktree.ResolvedPath{
		AbsPath:  srcPath,
		RepoPath: repoDir,
		Branch:   "source-branch",
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := Fork(rp, "", "tap")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)

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
```

Note: `shop_test.go` already imports `bytes`, `io`, `os`, `os/exec`, `path/filepath`, `strings`, `testing`, `worktree`. Add `"io"` if missing.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/shop/... -run TestForkAutoName -v`
Expected: FAIL — `Fork` undefined

**Step 3: Implement `Fork` in shop**

Add to `packages/spinclass/internal/shop/shop.go`:

```go
// Fork creates a new worktree branched from rp's current HEAD.
// If newBranch is empty, a name is auto-generated as <rp.Branch>-N.
// Does not attach to the new session.
func Fork(rp worktree.ResolvedPath, newBranch string, format string) error {
	if newBranch == "" {
		newBranch = worktree.ForkName(rp.RepoPath, rp.Branch)
	}

	newPath := filepath.Join(rp.RepoPath, worktree.WorktreesDir, newBranch)

	if _, err := worktree.CreateFrom(rp.RepoPath, rp.AbsPath, newPath, newBranch); err != nil {
		return err
	}

	if format == "tap" {
		tw := tap.NewWriter(os.Stdout)
		tw.PlanAhead(1)
		tw.Ok("fork " + newBranch + " " + newPath)
		return nil
	}

	fmt.Println(newPath)
	return nil
}
```

Also add `"path/filepath"` to shop.go's imports if not already present (it is not — check imports at top of file and add it).

**Step 4: Run test to verify it passes**

Run: `go test ./internal/shop/... -run TestForkAutoName -v`
Expected: PASS

**Step 5: Run all shop tests**

Run: `go test ./internal/shop/... -v`
Expected: all PASS

**Step 6: Commit**

```
git add packages/spinclass/internal/shop/shop.go packages/spinclass/internal/shop/shop_test.go
git commit -m "feat(spinclass): add Fork to shop package"
```

---

### Task 4: Add `fork` command to CLI

Wire up the cobra command in `main.go`. Reads `$SPINCLASS_SESSION`, resolves cwd to repo path, constructs `ResolvedPath` for the current worktree, calls `shop.Fork`.

**Files:**
- Modify: `packages/spinclass/cmd/spinclass/main.go`

**Step 1: Add the fork command**

Add to `packages/spinclass/cmd/spinclass/main.go` (before the `init()` function):

```go
var forkCmd = &cobra.Command{
	Use:   "fork [<new-branch>]",
	Short: "Fork current worktree into a new branch",
	Long:  `Create a new worktree branched from the current worktree's HEAD. If new-branch is omitted, a name is auto-generated as <current-branch>-N. Must be run from inside a spinclass session (SPINCLASS_SESSION must be set). Does not attach to the new session.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		session := os.Getenv("SPINCLASS_SESSION")
		if session == "" {
			return fmt.Errorf("SPINCLASS_SESSION is not set: are you inside a spinclass session?")
		}

		// session is "<repo-dirname>/<branch>"; extract branch as everything after first "/"
		slashIdx := strings.Index(session, "/")
		if slashIdx < 0 {
			return fmt.Errorf("invalid SPINCLASS_SESSION format: %q (expected <repo>/<branch>)", session)
		}
		currentBranch := session[slashIdx+1:]

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		repoPath, err := worktree.DetectRepo(cwd)
		if err != nil {
			return err
		}

		currentPath := filepath.Join(repoPath, worktree.WorktreesDir, currentBranch)

		rp := worktree.ResolvedPath{
			AbsPath:    currentPath,
			RepoPath:   repoPath,
			Branch:     currentBranch,
			SessionKey: session,
		}

		var newBranch string
		if len(args) == 1 {
			newBranch = args[0]
		}

		format := outputFormat
		if format == "" {
			format = "tap"
		}

		return shop.Fork(rp, newBranch, format)
	},
}
```

Also add `"path/filepath"` and `"strings"` to the imports in `main.go` if not already present. Check the existing import block — `filepath` is already imported, `strings` is not. Add `"strings"`.

**Step 2: Register the command in `init()`**

Add to the `init()` function in `main.go`, alongside the other `rootCmd.AddCommand(...)` calls:

```go
rootCmd.AddCommand(forkCmd)
```

**Step 3: Build to verify it compiles**

Run: `go build ./cmd/spinclass/...`
Expected: exits 0, no errors

**Step 4: Smoke test the error path**

Run: `SPINCLASS_SESSION="" go run ./cmd/spinclass fork`
Expected: error message containing `SPINCLASS_SESSION is not set`

**Step 5: Run all tests**

Run: `go test ./...`
Expected: all PASS

**Step 6: Commit**

```
git add packages/spinclass/cmd/spinclass/main.go
git commit -m "feat(spinclass): add fork command"
```
