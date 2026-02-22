# Fix create and attach commands — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix five bugs in spinclass's `create` and `attach` commands so they behave correctly.

**Architecture:** Targeted fixes across `git.go`, `worktree.go`, `shop.go`, and `apply.go`. Each fix is independent and gets its own commit.

**Tech Stack:** Go 1.23, Cobra CLI, git

---

### Task 1: Fix DefaultBranch to use symbolic-ref

**Files:**
- Modify: `internal/git/git.go:118-120`

**Step 1: Write the new DefaultBranch implementation**

Replace the current `DefaultBranch` function:

```go
func DefaultBranch(repoPath string) (string, error) {
	out, err := Run(repoPath, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil {
		branch := strings.TrimPrefix(out, "refs/remotes/origin/")
		if branch != "" {
			return branch, nil
		}
	}
	return BranchCurrent(repoPath)
}
```

**Step 2: Run tests**

Run: `cd /home/sasha/eng/repos/purse-first && nix develop --command go test ./packages/spinclass/...`
Expected: all pass (no existing tests for DefaultBranch, callers are unaffected)

**Step 3: Commit**

```
fix(spinclass): detect default branch via symbolic-ref

Falls back to BranchCurrent when no remote is configured.
```

---

### Task 2: Remove os.MkdirAll before git worktree add

**Files:**
- Modify: `internal/worktree/worktree.go:79-81`

**Step 1: Remove the MkdirAll call**

Delete these lines from `Create()`:

```go
if err := os.MkdirAll(worktreePath, 0o755); err != nil {
    return sweatfile.LoadResult{}, fmt.Errorf("creating worktree directory: %w", err)
}
```

**Step 2: Run tests**

Run: `cd /home/sasha/eng/repos/purse-first && nix develop --command go test ./packages/spinclass/...`
Expected: all pass

**Step 3: Smoke test**

```bash
cd /tmp/spinclass-test-repo && spinclass create task2-test
```

Expected: worktree created without error

**Step 4: Commit**

```
fix(spinclass): remove redundant MkdirAll before git worktree add

git worktree add creates its own directory.
```

---

### Task 3: Replace os.Chdir with stdout path output

**Files:**
- Modify: `internal/shop/shop.go:18-30`

**Step 1: Update shop.Create to print path and return**

Replace:

```go
func Create(rp worktree.ResolvedPath, verbose bool) error {
	if _, err := os.Stat(rp.AbsPath); os.IsNotExist(err) {
		result, err := worktree.Create(rp.RepoPath, rp.AbsPath)
		if err != nil {
			return err
		}
		if verbose {
			logSweatfileResult(result)
		}
	}

	return os.Chdir(rp.AbsPath)
}
```

With:

```go
func Create(rp worktree.ResolvedPath, verbose bool) error {
	if _, err := os.Stat(rp.AbsPath); os.IsNotExist(err) {
		result, err := worktree.Create(rp.RepoPath, rp.AbsPath)
		if err != nil {
			return err
		}
		if verbose {
			logSweatfileResult(result)
		}
	}

	fmt.Println(rp.AbsPath)
	return nil
}
```

**Step 2: Run tests**

Run: `cd /home/sasha/eng/repos/purse-first && nix develop --command go test ./packages/spinclass/...`
Expected: all pass

**Step 3: Commit**

```
fix(spinclass): print worktree path to stdout instead of os.Chdir

os.Chdir only affected the spinclass process. Now users can
compose with cd $(spinclass create feature-x).
```

---

### Task 4: Fix Claude permission path format

**Files:**
- Modify: `internal/sweatfile/apply.go:82-83`
- Modify: `internal/sweatfile/apply_test.go:72-73`

**Step 1: Update apply.go permission format**

In `ApplyClaudeSettings`, change:

```go
	allRules = append(allRules,
		"Edit(//"+worktreePath+"/**)",
		"Write(//"+worktreePath+"/**)",
	)
```

To:

```go
	allRules = append(allRules,
		"Edit("+worktreePath+"/*)",
		"Write("+worktreePath+"/*)",
	)
```

**Step 2: Update the test expectations**

In `apply_test.go`, change:

```go
	wantEdit := "Edit(//" + dir + "/**)"
	wantWrite := "Write(//" + dir + "/**)"
```

To:

```go
	wantEdit := "Edit(" + dir + "/*)"
	wantWrite := "Write(" + dir + "/*)"
```

**Step 3: Run tests**

Run: `cd /home/sasha/eng/repos/purse-first && nix develop --command go test ./packages/spinclass/...`
Expected: all pass

**Step 4: Commit**

```
fix(spinclass): correct Claude permission path format

Remove erroneous // prefix and use /* glob to match perms
package matching logic.
```
