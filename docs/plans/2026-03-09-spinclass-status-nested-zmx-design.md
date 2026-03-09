# Spinclass Status: Nested Worktrees + zmx Sessions

**Date**: 2026-03-09
**Scope**: `packages/spinclass/internal/status/`, `packages/spinclass/internal/executor/`

## Problem

The `spinclass status` command currently renders a flat table with a redundant
"Repo" column, grouping rows into Repos/Worktrees/Clean sections. Worktrees
aren't visually associated with their parent repos. There's also no indication
of which worktrees have active zmx sessions.

## Design

### Data Model

Replace flat `[]BranchStatus` return from `CollectStatus` with grouped
`[]RepoStatus`:

```go
type RepoStatus struct {
    Main      BranchStatus
    Worktrees []BranchStatus
}
```

Add `Session string` field to `BranchStatus` — empty means no active session.

### zmx Session Detection

New exported function in `internal/executor/zmx.go`:

```go
func ListSessions() (map[string]bool, error)
```

Runs `zmx -g sc list`, parses `session_name=<key>` from each line (tab-delimited
fields, `key=value` pairs). Returns set of active session keys. Called once at
the top of `CollectStatus`, passed to `CollectRepoStatus` to populate `Session`.

Session keys use existing `<repo-dirname>/<branch>` convention.

### Rendering

Tree-style text output replacing lipgloss tables:

```
purse-first  master  clean  ≡ origin/master
  ├ fresh-oak   1M     ↑3 origin/master   ● zmx
  └ ready-syca  clean  ≡ origin/master    ● zmx
finicky  main  clean  ≡ origin/main
  └ rich-magnolia  2M 1?  ↑1 origin/main  ● zmx
```

- Repo name bold, followed by main branch status on same line
- Worktrees indented with `├`/`└` tree connectors
- Columns aligned across all rows via max-width calculation
- Color via lipgloss styles: green for clean, red for dirty, yellow for
  ahead/behind, green `●` for active zmx session
- No grouping (Repos/Worktrees/Clean sections removed) — repos appear in scan
  order

### TAP Output

`RenderTap` adapts to `[]RepoStatus` — still emits one `ok` per branch.

### Tests

- Update existing render tests for new `RepoStatus` structure
- New test for `ListSessions` output parsing
- New test for tree-style render output
