# Auto-Merge on Shop Close — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to
> implement this plan task-by-task.

**Goal:** Auto-merge clean worktrees on session close, with `--no-merge` opt-out.

**Architecture:** Extract core merge logic from `merge.Run` into
`merge.Resolved` that accepts resolved paths. Wire `closeShop` to call it when
the worktree is clean. Add `--no-merge` flag on `attach` to skip.

**Tech Stack:** Go, cobra, spinclass internals (git, tap, executor, worktree)

---

### Task 1: Extract `merge.Resolved` from `merge.Run`

**Files:**

- Modify: `packages/spinclass/internal/merge/merge.go:18-135`

**Step 1: Extract `Resolved` function**

Move lines 56–135 of `merge.Run` (everything after path resolution) into a new
exported function. `merge.Run` calls `Resolved` after resolving paths.

In `packages/spinclass/internal/merge/merge.go`, replace the current `Run`
function body with path resolution that delegates to `Resolved`:

```go
func Resolved(execr executor.Executor, format, repoPath, wtPath, branch string) error {
	if info, err := os.Stat(repoPath); err != nil || !info.IsDir() {
		return fmt.Errorf("repository not found: %s", repoPath)
	}

	defaultBranch, err := git.DefaultBranch(repoPath)
	if err != nil {
		return fmt.Errorf("could not determine default branch: %w", err)
	}

	var tw *tap.Writer
	if format == "tap" {
		tw = tap.NewWriter(os.Stdout)
	}

	if tw == nil {
		log.Info("rebasing onto "+defaultBranch, "worktree", branch)
	}

	if err := git.RunPassthroughEnv(wtPath, []string{"GIT_SEQUENCE_EDITOR=true"}, "rebase", defaultBranch, "-i"); err != nil {
		if tw != nil {
			tw.NotOk("rebase "+branch, map[string]string{
				"message":  err.Error(),
				"severity": "fail",
			})
			tw.Plan()
		} else {
			log.Error("rebase failed, not merging")
		}
		return err
	}

	if tw != nil {
		tw.Ok("rebase " + branch)
	}

	if tw == nil {
		log.Info("merging worktree", "worktree", branch)
	}

	if err := git.RunPassthrough(repoPath, "merge", "--ff-only", branch); err != nil {
		if tw != nil {
			tw.NotOk("merge "+branch, map[string]string{
				"message":  err.Error(),
				"severity": "fail",
			})
			tw.Plan()
		} else {
			log.Error("merge failed, not removing worktree")
		}
		return err
	}

	if tw != nil {
		tw.Ok("merge " + branch)
	}

	if tw == nil {
		log.Info("removing worktree", "path", wtPath)
	}

	if err := git.RunPassthrough(repoPath, "worktree", "remove", wtPath); err != nil {
		if tw != nil {
			tw.NotOk("remove worktree "+branch, map[string]string{
				"message":  err.Error(),
				"severity": "fail",
			})
			tw.Plan()
		}
		return err
	}

	if tw != nil {
		tw.Ok("remove worktree " + branch)
		tw.Plan()
	} else {
		log.Info("detaching from session")
	}

	return execr.Detach()
}
```

Then slim `Run` down to just path resolution + delegate:

```go
func Run(execr executor.Executor, format string, target string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	var repoPath, wtPath, branch string

	if worktree.IsWorktree(cwd) && target == "" {
		repoPath, err = git.CommonDir(cwd)
		if err != nil {
			return fmt.Errorf("not in a worktree directory: %s", cwd)
		}
		wtPath = cwd
		branch, err = git.BranchCurrent(cwd)
		if err != nil {
			return fmt.Errorf("could not determine current branch: %w", err)
		}
	} else {
		if worktree.IsWorktree(cwd) {
			repoPath, err = git.CommonDir(cwd)
		} else {
			repoPath, err = worktree.DetectRepo(cwd)
		}
		if err != nil {
			return fmt.Errorf("not in a git repository: %s", cwd)
		}

		if target != "" {
			wtPath, branch, err = resolveWorktree(repoPath, target)
		} else {
			wtPath, branch, err = chooseWorktree(repoPath)
		}
		if err != nil {
			return err
		}
	}

	return Resolved(execr, format, repoPath, wtPath, branch)
}
```

**Step 2: Verify existing tests still pass**

Run: `nix develop --command go test ./packages/spinclass/...`

Expected: All tests pass (this is a pure refactor, no behavior change).

**Step 3: Commit**

```
git add packages/spinclass/internal/merge/merge.go
git commit -m "refactor: extract merge.Resolved from merge.Run"
```

---

### Task 2: Add `noMerge` parameter to `shop.Attach` and wire auto-merge

**Files:**

- Modify: `packages/spinclass/internal/shop/shop.go:69-127`

**Step 1: Add `noMerge` parameter and auto-merge logic**

Update `Attach` signature and `closeShop` to accept `noMerge`. When clean and
`!noMerge`, call `merge.Resolved` instead of reporting status.

In `packages/spinclass/internal/shop/shop.go`:

```go
func Attach(exec executor.Executor, rp worktree.ResolvedPath, format string, claudeArgs []string, noMerge bool) error {
	if err := createWorktree(rp, false); err != nil {
		return err
	}

	if format != "tap" {
		fmt.Println(rp.AbsPath)
	}

	var command []string
	if len(claudeArgs) > 0 {
		if flake.HasDevShell(rp.AbsPath) {
			log.Info("flake.nix detected, starting claude in nix develop")
			command = append([]string{"nix", "develop", "--command", "claude"}, claudeArgs...)
		} else {
			command = append([]string{"claude"}, claudeArgs...)
		}
	} else if flake.HasDevShell(rp.AbsPath) {
		log.Info("flake.nix detected, starting session in nix develop")
		command = []string{"nix", "develop", "--command", os.Getenv("SHELL")}
	}

	if err := exec.Attach(rp.AbsPath, rp.SessionKey, command); err != nil {
		return fmt.Errorf("attach failed: %w", err)
	}

	return closeShop(exec, rp, format, noMerge)
}
```

Update `closeShop` to accept `exec` and `noMerge`, and call `merge.Resolved`
when conditions are met:

```go
func closeShop(exec executor.Executor, rp worktree.ResolvedPath, format string, noMerge bool) error {
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

	worktreeStatus := git.StatusPorcelain(rp.AbsPath)
	isClean := worktreeStatus == ""

	if isClean && !noMerge {
		return merge.Resolved(exec, format, rp.RepoPath, rp.AbsPath, rp.Branch)
	}

	commitsAhead := git.CommitsAhead(rp.AbsPath, defaultBranch, rp.Branch)
	desc := statusDescription(defaultBranch, commitsAhead, worktreeStatus)

	if format == "tap" {
		tw := tap.NewWriter(os.Stdout)
		tw.Ok("create " + rp.Branch)
		tw.Ok("close " + rp.Branch + " # " + desc)
		tw.Plan()
	} else {
		log.Info(desc, "worktree", rp.SessionKey)
	}

	return nil
}
```

Add the `merge` import to `shop.go`:

```go
import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/amarbel-llc/spinclass/internal/executor"
	"github.com/amarbel-llc/spinclass/internal/flake"
	"github.com/amarbel-llc/spinclass/internal/git"
	"github.com/amarbel-llc/spinclass/internal/merge"
	"github.com/amarbel-llc/spinclass/internal/sweatfile"
	"github.com/amarbel-llc/spinclass/internal/tap"
	"github.com/amarbel-llc/spinclass/internal/worktree"
)
```

**Step 2: Verify compilation and existing tests**

Run: `nix develop --command go build ./packages/spinclass/...`

Then: `nix develop --command go test ./packages/spinclass/...`

Expected: Compile succeeds, existing tests pass (shop_test.go only tests
`statusDescription` which is unchanged).

**Step 3: Commit**

```
git add packages/spinclass/internal/shop/shop.go
git commit -m "feat: auto-merge clean worktrees on shop close"
```

---

### Task 3: Add `--no-merge` flag to `attachCmd`

**Files:**

- Modify: `packages/spinclass/cmd/spinclass/main.go:61-97,206-219`

**Step 1: Add flag variable and wire it**

Add a package-level `var attachNoMerge bool` and register the flag. Pass it to
`shop.Attach`.

Near the top of `main.go` (after `var createVerbose bool`):

```go
var attachNoMerge bool
```

In the `attachCmd.RunE` function, change the `shop.Attach` call:

```go
return shop.Attach(exec, rp, format, claudeArgs, attachNoMerge)
```

In `init()`, add the flag registration:

```go
attachCmd.Flags().BoolVar(&attachNoMerge, "no-merge", false, "skip auto-merge on session close")
```

**Step 2: Verify it compiles**

Run: `nix develop --command go build ./packages/spinclass/...`

Expected: Clean build.

**Step 3: Commit**

```
git add packages/spinclass/cmd/spinclass/main.go
git commit -m "feat: add --no-merge flag to attach command"
```

---

### Task 4: Verify full build

**Step 1: Run Go tests for spinclass**

Run: `nix develop --command go test ./packages/spinclass/...`

Expected: All tests pass.

**Step 2: Run nix build**

Run: `nix build`

Expected: Clean build.

**Step 3: Final commit with design doc**

```
git add packages/spinclass/docs/plans/2026-02-24-auto-merge-on-close-design.md
git add packages/spinclass/docs/plans/2026-02-24-auto-merge-on-close-plan.md
git commit -m "docs: add auto-merge on close design and plan"
```
