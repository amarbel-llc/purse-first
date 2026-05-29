---
status: relocated
date: 2026-05-29
relocated-to: amarbel-llc/spinclass
promotion-criteria: n/a — shipped; silent-error behaviors are known limitations,
  not bugs to fix
---

# Merge & Close-Shop Lifecycle

> **Relocated:** `spinclass` moved out of purse-first in commit `e1d6632`
> ("slim purse-first to framework-only"). This feature now lives in
> [amarbel-llc/spinclass](https://github.com/amarbel-llc/spinclass); this FDR
> is retained for historical context. Paths in **More Information** are
> relative to the former `packages/spinclass/`.

## Motivation

A worktree session (created by `spinclass new` or `spinclass attach`) ends
when the user exits the attached terminal session. At that point, spinclass has
an opportunity to integrate the worktree's work back into the main branch and
clean up the worktree directory.

The challenge is that the worktree state at exit is unknown: it may be clean,
dirty, ahead by many commits, or already merged. The user may or may not want
an automatic merge. The environment may or may not be interactive.

`closeShop` is the function that handles this moment.

## State Machine

After the executor returns (session exit), `closeShop` runs. The full decision
tree:

```
session exits
    │
    ▼
determine defaultBranch
    ├─ error or ambiguous, non-interactive → log.Warn, return nil (silent)
    └─ ambiguous, interactive → prompt user to choose main/master
    │
    ▼
check git status (porcelain)
    │
    ├─ CLEAN + --merge-on-close
    │       → merge.Resolved() [rebase → ff-merge → worktree remove → opt. git-sync]
    │
    ├─ DIRTY + --merge-on-close + interactive
    │       → prompt loop:
    │             ┌─ Discard: git checkout . && git clean -fd → merge.Resolved()
    │             ├─ Reattach: re-enter session → check status again → if clean → merge.Resolved()
    │             └─ Exit: fall through to status report
    │
    └─ anything else (no --merge-on-close, or dirty + non-interactive)
            → emit status description and return
```

## Merge Sequence

`merge.Resolved()` runs these steps in order. Each failure stops the sequence
without rolling back prior steps:

1. **Rebase**: `git rebase <defaultBranch> -i` with `GIT_SEQUENCE_EDITOR=true`
   (non-interactive: accepts all commits without editor).
2. **Fast-forward merge**: `git merge --ff-only <branch>` in the repo root.
3. **Worktree remove**: `git worktree remove <worktreePath>`.
4. **Pull** (if `--git-sync`): `git pull` in repo root.
5. **Push** (if `--git-sync`): `git push` in repo root.
6. **Detach**: `executor.Detach()` to leave the zmx session.

Each step is reported as a TAP test point when `--format tap` is active.

## Silent Error Behaviors

Several error conditions are swallowed with `log.Warn` and `return nil`:

| Condition | Behavior |
|-----------|----------|
| Default branch undetermined | `log.Warn("could not determine default branch")`, return nil |
| Ambiguous default branch (both main + master), non-interactive | `log.Warn("both main and master branches exist, skipping rebase")`, return nil |
| Branch name undetermined | `log.Warn("could not determine current branch")`, return nil |
| Branch selection cancelled (interactive prompt) | `log.Warn("branch selection cancelled")`, return nil |

These are intentional: `closeShop` is a post-session cleanup step. Returning
an error would propagate to the user after their session has already ended. A
warning is preferable to an error that may appear to blame the user for
something they can't fix at that point.

The worktree is **not removed** on any failure. The user can manually run
`spinclass merge` to retry.

## `--merge-on-close` Flag

`spinclass new` and `spinclass attach` accept `--merge-on-close`. When set:

- Clean worktree at exit → automatic merge, no prompt.
- Dirty worktree, interactive → prompt loop (discard/reattach/exit).
- Dirty worktree, non-interactive → fall through to status report, no merge.

Without `--merge-on-close`: always fall through to status report.

## `--git-sync` Flag

`spinclass merge` accepts `--git-sync`. When set, after worktree removal:
pull and push the default branch. Intended for single-developer workflows where
the repo has a single upstream remote.

`closeShop` with `--merge-on-close` does **not** pass `--git-sync` to
`merge.Resolved` — git-sync is only available via the explicit `merge` command.

## Status Description

When closeShop falls through without merging, it emits:

```
<N> commits ahead of <defaultBranch>, clean/dirty [, (merged)]
```

Examples:
- `"1 commit ahead of main, clean"`
- `"3 commits ahead of master, dirty"`
- `"0 commits ahead of main, clean, (merged)"` — already merged

## Interface

### `spinclass new` / `spinclass attach`

```
spinclass new <branch> [--merge-on-close] [--no-attach] [-- <claude-args>...]
spinclass attach <branch> [--merge-on-close] [--no-attach] [-- <claude-args>...]
```

`--no-attach`: create the worktree but do not open a session; `closeShop` is
skipped.

### `spinclass merge`

```
spinclass merge [<target>] [--git-sync] [--format tap|table] [--verbose]
```

Without `<target>`: if in a worktree, merges current worktree; otherwise
presents an interactive selector. With `<target>`: resolves by branch name.

## Limitations

- **No rollback.** If rebase succeeds but ff-merge fails (e.g. another commit
  landed on default branch), the worktree is not removed and the rebase is not
  reverted. The user must resolve manually.
- **No dirty-worktree merge in non-interactive mode.** There is no flag to
  force-discard and merge without interaction.
- **Silent skips.** Ambiguous default branch in non-interactive mode silently
  skips the merge. This is the correct behavior for automation but can be
  surprising when running via CI or a non-TTY shell.
- **`--git-sync` is not atomic.** Pull and push are separate steps. A push
  failure after a successful pull leaves the local and remote in inconsistent
  states.

## More Information

- Core merge: `packages/spinclass/internal/merge/merge.go`
- Close-shop orchestration: `packages/spinclass/internal/shop/shop.go` (`closeShop`)
- Dirty-action prompt: `packages/spinclass/internal/shop/prompt.go`
