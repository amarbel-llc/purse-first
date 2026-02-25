# Shop Exit Cleanup Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** When a user exits a spinclass session with a dirty worktree and stdin is a TTY, prompt them to discard changes, reattach, or exit without integrating.

**Architecture:** Add an interactive prompt loop in `closeShop` using `charmbracelet/huh`. TTY detection via `mattn/go-isatty`. Also migrate `merge.chooseWorktree` from shelling out to `gum` to using `huh`.

**Tech Stack:** Go, `charmbracelet/huh` (already direct dep), `mattn/go-isatty` (promote from indirect to direct)

---

### Task 1: Promote go-isatty to direct dependency

**Files:**
- Modify: `packages/spinclass/go.mod:36` (move `mattn/go-isatty` from indirect to direct)

**Step 1: Update go.mod**

Move `github.com/mattn/go-isatty v0.0.20` from the indirect require block to
the direct require block.

**Step 2: Run go mod tidy**

Run: `cd packages/spinclass && go mod tidy`
Expected: `go.mod` updated, no errors

**Step 3: Commit**

```
feat(spinclass): promote go-isatty to direct dependency
```

---

### Task 2: Add dirty action type and prompt function

**Files:**
- Create: `packages/spinclass/internal/shop/prompt.go`
- Create: `packages/spinclass/internal/shop/prompt_test.go`

**Step 1: Write the test**

```go
package shop

import "testing"

func TestDirtyActionStringValues(t *testing.T) {
	tests := []struct {
		action dirtyAction
		want   string
	}{
		{actionDiscard, "Discard changes and merge"},
		{actionReattach, "Reattach to session"},
		{actionExit, "Exit without integrating"},
	}
	for _, tt := range tests {
		if got := tt.action; string(got) != tt.want {
			t.Errorf("dirtyAction = %q, want %q", got, tt.want)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd packages/spinclass && go test ./internal/shop/ -run TestDirtyAction -v`
Expected: FAIL — `dirtyAction` type not defined

**Step 3: Write prompt.go**

```go
package shop

import (
	"github.com/charmbracelet/huh"
)

type dirtyAction string

const (
	actionDiscard  dirtyAction = "Discard changes and merge"
	actionReattach dirtyAction = "Reattach to session"
	actionExit     dirtyAction = "Exit without integrating"
)

func promptDirtyAction(branch string) (dirtyAction, error) {
	var action dirtyAction
	err := huh.NewSelect[dirtyAction]().
		Title("Worktree " + branch + " has uncommitted changes").
		Options(
			huh.NewOption(string(actionDiscard), actionDiscard),
			huh.NewOption(string(actionReattach), actionReattach),
			huh.NewOption(string(actionExit), actionExit),
		).
		Value(&action).
		Run()
	return action, err
}
```

**Step 4: Run test to verify it passes**

Run: `cd packages/spinclass && go test ./internal/shop/ -run TestDirtyAction -v`
Expected: PASS

**Step 5: Commit**

```
feat(spinclass): add dirty action type and interactive prompt
```

---

### Task 3: Add discardAll helper and test

**Files:**
- Modify: `packages/spinclass/internal/shop/shop.go`
- Modify: `packages/spinclass/internal/shop/shop_test.go`

**Step 1: Write the test**

Add to `shop_test.go`:

```go
func TestDiscardAllCleansWorktree(t *testing.T) {
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

	// Create a tracked file and commit
	if err := os.WriteFile(filepath.Join(repoDir, "tracked.txt"), []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(repoDir, "add", "tracked.txt")
	runGit(repoDir, "commit", "-m", "initial")

	// Create worktree
	wtDir := filepath.Join(repoDir, worktree.WorktreesDir)
	wtPath := filepath.Join(wtDir, "dirty-test")
	runGit(repoDir, "worktree", "add", "-b", "dirty-test", wtPath)

	// Dirty it: modify tracked file + add untracked file
	if err := os.WriteFile(filepath.Join(wtPath, "tracked.txt"), []byte("modified"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "untracked.txt"), []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Verify it's dirty
	if git.StatusPorcelain(wtPath) == "" {
		t.Fatal("expected dirty worktree")
	}

	if err := discardAll(wtPath); err != nil {
		t.Fatalf("discardAll error: %v", err)
	}

	if status := git.StatusPorcelain(wtPath); status != "" {
		t.Errorf("expected clean worktree after discard, got: %q", status)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd packages/spinclass && go test ./internal/shop/ -run TestDiscardAll -v`
Expected: FAIL — `discardAll` not defined

**Step 3: Write discardAll in shop.go**

Add to `shop.go`:

```go
func discardAll(wtPath string) error {
	if _, err := git.Run(wtPath, "checkout", "."); err != nil {
		return fmt.Errorf("git checkout: %w", err)
	}
	if _, err := git.Run(wtPath, "clean", "-fd"); err != nil {
		return fmt.Errorf("git clean: %w", err)
	}
	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd packages/spinclass && go test ./internal/shop/ -run TestDiscardAll -v`
Expected: PASS

**Step 5: Commit**

```
feat(spinclass): add discardAll helper for worktree cleanup
```

---

### Task 4: Wire interactive prompt into closeShop

**Files:**
- Modify: `packages/spinclass/internal/shop/shop.go` — `closeShop` and `New` signatures

**Step 1: Update closeShop signature**

Change `closeShop` to accept `exec executor.Executor`, `interactive bool`,
`command []string`, `noAttach bool`, and `sessionKey string`:

```go
func closeShop(w io.Writer, exec executor.Executor, rp worktree.ResolvedPath, format string, noMerge bool, verbose bool, tw *tap.Writer, interactive bool, command []string, noAttach bool) error {
```

**Step 2: Add the interactive dirty-worktree loop**

Replace the existing dirty-worktree branch (the `else` after `if isClean &&
!noMerge`) with:

```go
	if interactive && !noMerge {
		for {
			action, err := promptDirtyAction(rp.Branch)
			if err != nil {
				break // prompt cancelled/failed, fall through to exit
			}

			switch action {
			case actionDiscard:
				if err := discardAll(rp.AbsPath); err != nil {
					if tw != nil {
						tw.NotOk("discard "+rp.Branch, map[string]string{
							"severity": "fail",
							"message":  err.Error(),
						})
						tw.Plan()
					}
					return err
				}
				err := merge.Resolved(exec, w, tw, format, rp.RepoPath, rp.AbsPath, rp.Branch, verbose)
				if tw != nil {
					tw.Plan()
				}
				return err

			case actionReattach:
				tp := tap.TestPoint{
					Description: "reattach " + rp.Branch,
					Ok:          true,
				}
				if err := exec.Attach(rp.AbsPath, rp.SessionKey, command, noAttach, &tp); err != nil {
					return fmt.Errorf("reattach failed: %w", err)
				}
				// Re-check status after reattach
				worktreeStatus = git.StatusPorcelain(rp.AbsPath)
				isClean = worktreeStatus == ""
				if isClean {
					err := merge.Resolved(exec, w, tw, format, rp.RepoPath, rp.AbsPath, rp.Branch, verbose)
					if tw != nil {
						tw.Plan()
					}
					return err
				}
				continue // loop back to prompt

			case actionExit:
				break // fall through to status log
			}
			break
		}
	}
```

**Step 3: Update New to pass interactive and plumbing args**

In `New`, add TTY detection and pass through:

```go
import "github.com/mattn/go-isatty"

// In New(), before calling closeShop:
interactive := isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())

return closeShop(w, exec, rp, format, noMerge, verbose, tw, interactive, command, noAttach)
```

Note: `New` already has `exec`, `command` (as `claudeArgs`), and `noAttach`
available. The `command` variable here is the same slice built from
`claudeArgs` (with `"claude"` prepended if non-empty).

**Step 4: Run all shop tests**

Run: `cd packages/spinclass && go test ./internal/shop/ -v`
Expected: All existing tests still pass (they use `mockExecutor` and `noMerge=true` or clean worktrees)

**Step 5: Commit**

```
feat(spinclass): prompt on dirty worktree exit when interactive
```

---

### Task 5: Migrate merge.chooseWorktree from gum to huh

**Files:**
- Modify: `packages/spinclass/internal/merge/merge.go:179-212` — `chooseWorktree` function

**Step 1: Replace gum with huh**

Replace the `chooseWorktree` function:

```go
import "github.com/charmbracelet/huh"

func chooseWorktree(repoPath string) (wtPath, branch string, err error) {
	paths := worktree.ListWorktrees(repoPath)
	if len(paths) == 0 {
		return "", "", fmt.Errorf("no worktrees found in %s", repoPath)
	}

	branches := make([]string, len(paths))
	for i, p := range paths {
		branches[i] = filepath.Base(p)
	}

	var selected string
	options := make([]huh.Option[string], len(branches))
	for i, b := range branches {
		options[i] = huh.NewOption(b, b)
	}

	err = huh.NewSelect[string]().
		Title("Select worktree to merge").
		Options(options...).
		Value(&selected).
		Run()
	if err != nil {
		return "", "", fmt.Errorf("worktree selection cancelled")
	}

	for i, b := range branches {
		if b == selected {
			return paths[i], b, nil
		}
	}

	return "", "", fmt.Errorf("selected worktree not found: %s", selected)
}
```

**Step 2: Remove unused imports**

Remove `"os/exec"` from merge.go imports (only needed for gum). Keep `"os"`
since it's still used for `os.Stdout` etc.

**Step 3: Run merge tests (if any) and build**

Run: `cd packages/spinclass && go build ./...`
Expected: Compiles without errors

**Step 4: Commit**

```
refactor(spinclass): migrate merge worktree selection from gum to huh
```

---

### Task 6: Run full test suite and verify

**Step 1: Run all Go tests**

Run: `cd packages/spinclass && go test ./... -v`
Expected: All tests pass

**Step 2: Build with nix**

Run: `just build` (from `packages/spinclass/`)
Expected: Clean build

**Step 3: Commit any fixups if needed**
