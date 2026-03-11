# Grit Interactive Rebase

Two new MCP tools for grit that enable programmatic interactive rebasing without
a terminal. Uses a two-step workflow: plan (read commit list), then execute
(apply a structured todo list).

## Tools

### interactive_rebase_plan

Read-only. Returns the commits between upstream and HEAD as structured JSON.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| repo_path | string | yes | Path to the git repository |
| upstream | string | yes | Ref to rebase onto (branch, tag, commit) |

**Response:**

```json
{
  "status": "plan",
  "branch": "feature",
  "upstream": "main",
  "commits": [
    {"hash": "abc1234", "subject": "Add login page"},
    {"hash": "def5678", "subject": "Fix typo in login"}
  ]
}
```

If upstream..HEAD is empty, returns `{"status": "up_to_date", "commits": []}`.

**Annotations:** ReadOnly=true, Destructive=false, Idempotent=true. No
MapsTools.

### interactive_rebase_execute

Destructive. Executes an interactive rebase using a caller-provided todo list.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| repo_path | string | yes | Path to the git repository |
| upstream | string | yes | Ref to rebase onto |
| todo | array | yes | Ordered list of todo entries |
| autostash | bool | no | Automatically stash/unstash uncommitted changes |

Each todo entry:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| action | string | yes | One of: pick, reword, squash, fixup, drop |
| hash | string | yes | Commit hash |
| message | string | conditional | Required for reword, ignored otherwise |

Array order determines commit order. Commits not listed are implicitly dropped.

**Response:** Same structure as existing rebase tool — status is `completed`,
`conflict`, or `up_to_date`. Conflict responses include `conflicts` array.

**Annotations:** ReadOnly=false, Destructive=true, Idempotent=false.
MapsTools: replaces Bash with prefixes `git rebase -i`, `git rebase --interactive`.

## Mechanism

Uses `GIT_SEQUENCE_EDITOR` to programmatically control `git rebase -i`:

1. `interactive_rebase_execute` writes a temp shell script that, when invoked,
   overwrites the git-rebase-todo file with the caller's todo list.
2. Sets `GIT_SEQUENCE_EDITOR=/path/to/temp-script` and runs
   `git rebase -i <upstream>`.
3. For `reword` actions, a second temp script handles `GIT_EDITOR` to write the
   specified commit message.
4. Temp files are cleaned up via `defer os.Remove`.

Requires a new `RunWithEnv` function in `internal/git/exec.go` that accepts
extra environment variables. The existing `Run` delegates to it.

## Validation

- `action` must be one of the supported values
- `reword` must include non-empty `message`
- `squash`/`fixup` cannot be the first entry
- All hashes validated via `git rev-parse` before executing
- Blocked on main/master branches
- Rejects if a rebase is already in progress

## Conflict Handling

When `interactive_rebase_execute` encounters conflicts, it returns status
`conflict` with the list of conflicted files. The caller then uses the existing
`rebase` tool with `continue`, `abort`, or `skip` to manage the in-progress
rebase.

## File Changes

| File | Change |
|------|--------|
| `packages/grit/internal/tools/interactive_rebase.go` | New — both tool registrations and handlers |
| `packages/grit/internal/git/exec.go` | Add `RunWithEnv` function |
| `packages/grit/internal/git/types.go` | Add `InteractiveRebasePlan` and `TodoEntry` types |
| `packages/grit/internal/tools/registry.go` | Register new tools |
| `packages/grit/zz-tests_bats/interactive_rebase_mcp.bats` | New — integration tests |
