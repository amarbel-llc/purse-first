# Auto-Merge on Shop Close

## Summary

When exiting a session (`closeShop`), if the worktree is clean, automatically
run the full merge flow (rebase + ff-only merge + remove worktree). This is the
default behavior, with a `--no-merge` flag on `attach` to opt out.

## Trigger Conditions

Auto-merge runs when:

- Worktree is clean (no uncommitted changes)
- `--no-merge` was NOT passed to `attach`

Auto-merge is skipped (falls back to status reporting) when:

- Worktree is dirty
- `--no-merge` flag was passed

## Changes

### `merge` package: extract `Resolved` function

Extract core merge logic from `merge.Run` into:

```go
func Resolved(exec executor.Executor, format, repoPath, wtPath, branch string) error
```

`merge.Run` calls `Resolved` after resolving paths from cwd. `closeShop` calls
`Resolved` directly with the already-known `ResolvedPath` fields.

### `shop.Attach`: add `noMerge` parameter

```go
func Attach(exec executor.Executor, rp worktree.ResolvedPath, format string, claudeArgs []string, noMerge bool) error
```

After session ends:

- If dirty or `noMerge` → current `closeShop` status reporting
- If clean → call `merge.Resolved(exec, format, repoPath, wtPath, branch)`

### `main.go` `attachCmd`: add `--no-merge` flag

New `bool` flag wired through to `shop.Attach`.

### TAP output

- Auto-merge path: TAP output comes from `merge.Resolved` (rebase/merge/remove
  test points)
- Skip path: TAP output comes from existing `closeShop` (create/close test
  points with status description)

## What doesn't change

- `spinclass merge` command (manual merge from any context)
- `closeShop` behavior for dirty/no-merge cases
- Executor interface
