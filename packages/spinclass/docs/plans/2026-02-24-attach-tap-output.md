# Attach TAP-14 Output Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make `spinclass attach` produce a single TAP-14 stream with two test points: one for create (worktree setup) and one for close (worktree status after session exits).

**Architecture:** `Attach()` owns the TAP stream. It creates one `*tap.Writer`, emits the version line and plan (`1..2`), then passes the writer to `Create()` and `CloseShop()` for their test points. When those functions are called standalone (not from attach), they create their own writer as today.

**Tech Stack:** Go 1.24, Cobra CLI, internal `tap` package

---

### Task 1: Add `*tap.Writer` parameter to `Create()` and `CloseShop()`

**Files:**
- Modify: `internal/shop/shop.go:19-46` (Create)
- Modify: `internal/shop/shop.go:88-116` (CloseShop)
- Modify: `internal/shop/shop_test.go` (update callers)

**Step 1: Write the failing test for Create with shared writer**

Add to `internal/shop/shop_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first && nix develop ./packages/spinclass#devShell --command go test ./packages/spinclass/internal/shop/... -run TestCreateSharedWriter -v`
Expected: compile error — `Create` doesn't accept `*tap.Writer`

**Step 3: Update `Create()` signature and logic**

In `internal/shop/shop.go`, replace the `Create` function:

```go
func Create(w io.Writer, rp worktree.ResolvedPath, verbose bool, format string, tw *tap.Writer) error {
	existed := true

	if _, err := os.Stat(rp.AbsPath); os.IsNotExist(err) {
		existed = false
		result, err := worktree.Create(rp.RepoPath, rp.AbsPath)
		if err != nil {
			return err
		}
		if verbose {
			logSweatfileResult(result)
		}
	}

	if format == "tap" {
		if tw == nil {
			tw = tap.NewWriter(w)
			tw.PlanAhead(1)
		}
		if existed {
			tw.Skip("create "+rp.Branch, "already exists "+rp.AbsPath)
		} else {
			tw.Ok("create " + rp.Branch + " " + rp.AbsPath)
		}
		return nil
	}

	fmt.Fprintln(w, rp.AbsPath)
	return nil
}
```

**Step 4: Update `CloseShop()` signature and logic**

In `internal/shop/shop.go`, replace the `CloseShop` function:

```go
func CloseShop(w io.Writer, rp worktree.ResolvedPath, format string, tw *tap.Writer) error {
	if rp.Branch == "" {
		if err := rp.FillBranchFromGit(); err != nil {
			log.Warn("could not determine current branch")
			return nil
		}
	}

	defaultBranch, err := git.BranchCurrent(rp.RepoPath)
	if err != nil || defaultBranch == "" {
		log.Warn("could not determine default branch")
		return nil
	}

	commitsAhead := git.CommitsAhead(rp.AbsPath, defaultBranch, rp.Branch)
	worktreeStatus := git.StatusPorcelain(rp.AbsPath)

	desc := statusDescription(defaultBranch, commitsAhead, worktreeStatus)

	if format == "tap" {
		if tw == nil {
			tw = tap.NewWriter(w)
			tw.PlanAhead(1)
		}
		tw.Ok("close " + rp.Branch + " # " + desc)
	} else {
		log.Info(desc, "worktree", rp.SessionKey)
	}

	return nil
}
```

**Step 5: Fix existing callers that pass `nil`**

Update the `Attach` function's internal calls (will be refactored in Task 2, but must compile now):

```go
func Attach(exec executor.Executor, rp worktree.ResolvedPath, format string, claudeArgs []string) error {
	if err := Create(os.Stdout, rp, false, "", nil); err != nil {
		return err
	}

	var command []string
	if len(claudeArgs) > 0 {
		command = append([]string{"claude"}, claudeArgs...)
	}

	if err := exec.Attach(rp.AbsPath, rp.SessionKey, command); err != nil {
		return fmt.Errorf("attach failed: %w", err)
	}

	return CloseShop(os.Stdout, rp, format, nil)
}
```

Update `Fork` (line 154):

```go
	if format == "tap" {
		tw := tap.NewWriter(w)
		tw.PlanAhead(1)
		tw.Ok("fork " + newBranch + " " + newPath)
		return nil
	}
```

Fork doesn't call Create or CloseShop, so no change needed there.

**Step 6: Fix existing test callers**

In `internal/shop/shop_test.go`, update all `Create()` calls to pass `nil` as the last argument:

- `TestCreateTapNewWorktreeErrorPath`: `Create(&buf, rp, false, "tap", nil)`
- `TestCreateTapSkipExisting`: `Create(&buf, rp, false, "tap", nil)`
- `TestCreateTapNewWorktree`: `Create(&buf, rp, false, "tap", nil)`

**Step 7: Run all tests**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first && nix develop ./packages/spinclass#devShell --command go test ./packages/spinclass/internal/shop/... -v`
Expected: all pass

**Step 8: Update `createCmd` caller in main.go**

In `cmd/spinclass/main.go:58`, update:

```go
return shop.Create(os.Stdout, rp, createVerbose, format, nil)
```

**Step 9: Build**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first && nix develop ./packages/spinclass#devShell --command go build ./packages/spinclass/...`
Expected: compiles

**Step 10: Commit**

```
refactor(spinclass): add shared tap.Writer parameter to Create and CloseShop

When a *tap.Writer is passed, the function uses it instead of creating
its own. This enables Attach to own a single TAP stream across both
functions. Standalone callers pass nil for unchanged behavior.
```

---

### Task 2: Wire `Attach()` to produce TAP-14 output

**Files:**
- Modify: `internal/shop/shop.go:69-86` (Attach)
- Modify: `cmd/spinclass/main.go:68-97` (attachCmd)

**Step 1: Write the failing test for Attach TAP output**

Add to `internal/shop/shop_test.go`:

```go
type mockExecutor struct {
	attachCalled bool
}

func (m *mockExecutor) Attach(dir string, key string, command []string) error {
	m.attachCalled = true
	return nil
}

func (m *mockExecutor) Detach() error {
	return nil
}

func TestAttachTapExistingWorktree(t *testing.T) {
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
	err := Attach(&buf, mock, rp, "tap", nil)
	if err != nil {
		t.Fatalf("Attach returned error: %v", err)
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
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first && nix develop ./packages/spinclass#devShell --command go test ./packages/spinclass/internal/shop/... -run TestAttachTapExistingWorktree -v`
Expected: compile error — `Attach` doesn't accept `io.Writer`

**Step 3: Update `Attach()` to produce TAP output**

In `internal/shop/shop.go`, replace the `Attach` function:

```go
func Attach(w io.Writer, exec executor.Executor, rp worktree.ResolvedPath, format string, claudeArgs []string) error {
	var tw *tap.Writer
	if format == "tap" {
		tw = tap.NewWriter(w)
		tw.PlanAhead(2)
	}

	if err := Create(w, rp, false, format, tw); err != nil {
		return err
	}

	var command []string
	if len(claudeArgs) > 0 {
		command = append([]string{"claude"}, claudeArgs...)
	}

	if err := exec.Attach(rp.AbsPath, rp.SessionKey, command); err != nil {
		return fmt.Errorf("attach failed: %w", err)
	}

	return CloseShop(w, rp, format, tw)
}
```

**Step 4: Update `attachCmd` in main.go**

In `cmd/spinclass/main.go`, update the `attachCmd.RunE` to pass `os.Stdout`:

```go
return shop.Attach(os.Stdout, exec, rp, format, claudeArgs)
```

**Step 5: Run all tests**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first && nix develop ./packages/spinclass#devShell --command go test ./packages/spinclass/internal/shop/... -v`
Expected: all pass

**Step 6: Commit**

```
feat(spinclass): add TAP-14 output to attach command

Attach now produces a single TAP stream with two test points:
create (ok/skip) and close (ok with worktree status).
```

---

### Task 3: Build and verify

**Step 1: Build with Nix**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first && nix build .#spinclass`
Expected: build succeeds

**Step 2: Commit** (if any build fixes were needed; otherwise skip)
