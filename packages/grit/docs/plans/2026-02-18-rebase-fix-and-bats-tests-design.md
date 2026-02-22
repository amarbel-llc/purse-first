# Rebase Fix and BATS Test Infrastructure

## Problem

The grit rebase tool hangs during operation. Root cause: `git.Run()` doesn't suppress interactive prompts or editors. When `git rebase --continue` runs, git opens `$EDITOR` for the commit message. Since `cmd.Stdin` is nil and no editor override is set, the process blocks indefinitely.

Secondary bug: line 135 of `rebase.go` uses `git test -d .git/rebase-merge` to detect in-progress rebases, but `git test` is not a valid git subcommand. This check always fails, making it a no-op.

## Fix

### Layer 1: Global safety in `git.Run()` (`internal/git/exec.go`)

Set environment variables on every git command:

- `GIT_TERMINAL_PROMPT=0` — prevents credential/auth prompts
- `GIT_EDITOR=true` — `true` exits 0 immediately, preventing editor hangs
- `cmd.Stdin` set to empty reader — prevents any stdin blocking

### Layer 2: Rebase-specific (`internal/tools/rebase.go`)

- Fix "rebase already in progress" check: replace `git test -d` with `os.Stat(filepath.Join(repoPath, ".git/rebase-merge"))`.

## Test Infrastructure

### BATS setup using batman

Directory structure:

```
zz-tests_bats/
├── justfile
├── common.bash
├── bin/
│   └── run-sandcastle-bats.bash
├── rebase.bats
└── rebase_mcp.bats
```

### Nix flake changes

Add `batman` and `sandcastle` as flake inputs. Add `bats-libs`, `bats`, and `sandcastle` to devShell packages.

### Test helpers (common.bash)

- Load bats-support, bats-assert, bats-assert-additions via `bats_load_library`
- `setup_test_repo()` — creates isolated git repo in `$BATS_TEST_TMPDIR` with initial commit
- `setup_test_home()` — XDG and `GIT_CONFIG_GLOBAL` isolation
- `setup_conflict_scenario()` — creates two divergent branches for rebase conflict testing
- `run_grit_mcp()` — sends JSON-RPC `tools/call` request to grit binary, captures response

### Test scenarios

**`rebase.bats`** — git-level behavior tests:

| Test | Purpose |
|------|---------|
| `clean_rebase` | Fast-forward rebase completes without hanging |
| `rebase_with_conflicts` | Returns conflict state, lists conflicted files |
| `continue_after_resolving` | Completes after conflict resolution, no editor hang |
| `abort_rebase` | Cleanly aborts in-progress rebase |
| `skip_conflicting_commit` | Skips commit and continues |
| `up_to_date_rebase` | No-op when already up to date |

**`rebase_mcp.bats`** — MCP JSON-RPC integration tests:

Same scenarios, but exercised through the grit binary via JSON-RPC `tools/call` requests. Validates JSON response structure (status field, conflicts array, branch/upstream metadata). `BATS_TEST_TIMEOUT` catches any hangs.

### Justfile integration

Root justfile gets `test-bats` and `test-bats-run` targets. `test` target runs both `test` (go) and `test-bats`.

Test justfile in `zz-tests_bats/` with `test-targets`, `test-tags`, and `test` recipes using sandcastle wrapper, TAP output, and parallel execution.
