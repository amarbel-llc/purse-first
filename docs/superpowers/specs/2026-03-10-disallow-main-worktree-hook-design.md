# Disallow Main Worktree Hook

Replaces the experimental `boundary-notify` PreToolUse hook with a narrower,
blocking check: deny tool calls that target the main worktree when operating
from a session worktree.

## Motivation

The `boundary-notify` hook flagged any path outside the worktree boundary as a
notification. This was too broad — paths outside both the worktree and the main
repo (e.g., `~/.config`, `/tmp`) are harmless. The real risk is accidentally
reading or modifying files in the main worktree, which can cause merge conflicts
or data loss.

## Sweatfile Config

New boolean in the `[hooks]` table:

```toml
[hooks]
disallow-main-worktree = true
```

Off by default (opt-in). When off, the hook allows everything (no output).

Replaces `[experimental] boundary-notify`. The `Experimental` struct is removed.

## Sweatfile Cascade

When the hook runs in a worktree, the hierarchy loads:

1. `~/.config/spinclass/sweatfile` (global)
2. Intermediate parent directories (existing walk from home to cwd)
3. Main repo root sweatfile (resolved via `git --git-common-dir`)
4. Worktree sweatfile (cwd)

Lower layers are overridden by higher layers. This lets the main repo's
sweatfile enable the flag and a worktree-local sweatfile disable it (or vice
versa).

When not running in a worktree, behavior is unchanged (no additional layer).

## Hook Behavior

The PreToolUse hook:

1. If `disallow-main-worktree` is false or unset: return nil (allow).
2. If not running in a worktree: return nil.
3. Resolve the main repo root via `git.CommonDir(cwd)`.
4. Extract paths from the tool input (Read/Write/Edit `file_path`,
   Glob/Grep `path`, Bash absolute path tokens, Task always nil).
5. For each path: resolve symlinks, check if it falls inside the main repo root.
6. If any path targets the main worktree: return `deny` with guidance to
   restrict operations to the session worktree.
7. Paths outside both the worktree and main repo: allowed (ignored).

The deny response uses the `hookSpecificOutput` envelope with
`permissionDecision: "deny"`.

## Changes

### Sweatfile struct (`internal/sweatfile/sweatfile.go`)

- Add `DisallowMainWorktree *bool` to `Hooks` struct
  (TOML key: `disallow-main-worktree`)
- Add `DisallowMainWorktreeEnabled() bool` accessor on `Sweatfile`
- Remove `Experimental` struct and `BoundaryNotifyEnabled()` accessor
- Remove `boundary-notify` field

### Sweatfile merge (`internal/sweatfile/hierarchy.go`)

- Update `Merge()` to handle `DisallowMainWorktree` (scalar override)
- Add `LoadWorktreeHierarchy(home, mainRepoRoot, worktreePath)` that inserts the
  main repo sweatfile between intermediate dirs and worktree sweatfile
- Hook command calls this variant when it detects a worktree

### Hook logic (`internal/hooks/hooks.go`)

- Replace `runPreToolUse` signature: remove `boundary`, `allowed`,
  `boundaryNotify` params; add `mainRepoRoot` and `disallowMainWorktree` params
- When denying: return `permissionDecision: "deny"` with reason
- Keep `extractPaths`, `extractAbsolutePathsFromCommand`, `resolvePath` helpers
- Remove `isInsideBoundary`, `isInsideAllowed` helpers
- Add `isInsideMainWorktree(path, mainRepoRoot) bool` helper

### Hook command (`internal/hooks/cmd.go`)

- Detect main repo root via `git.CommonDir(cwd)` when in a worktree
- Call `LoadWorktreeHierarchy` instead of `LoadHierarchy`
- Pass `mainRepoRoot` and `disallowMainWorktree` flag to `Run`
- Remove `allowed` list construction (no longer needed)

### Tests (`internal/hooks/hooks_test.go`)

- Remove boundary-notify tests
- Add: flag off allows everything
- Add: flag on + path in main worktree denies
- Add: flag on + path in worktree allows
- Add: flag on + path elsewhere allows
- Add: not in a worktree allows

### Cleanup

- Remove `Experimental` struct entirely
- Update or supersede FDR-0004
