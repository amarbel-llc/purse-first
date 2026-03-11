# grit try_commit Tool

Add a `try_commit` MCP tool to grit that combines the stage + commit + context
gathering cycle into a single tool call. Agents currently call status, diff, log,
add, and commit as separate tool invocations when creating commits — five
round-trips that waste tokens and latency. `try_commit` collapses this into one.

## Motivation

The typical agent commit flow:

1. `status` — see what's changed
2. `diff` — see staged/unstaged changes
3. `log` — match commit message style
4. `add` — stage files
5. `commit` — create the commit

Each call is a full MCP round-trip. The agent already knows what it changed (it
wrote the code), so most of this context gathering is redundant. The useful
signal is: what got committed (diff stats), and what's left (post-commit status).

## Tool Definition

**Name:** `try_commit`

**Title:** Try Commit

**Description:**

```go
command.Description{Short: "Stage, commit, and return context in a single call. Replaces the status, diff, log, add, commit multi-tool cycle. Use this instead of calling those tools individually when creating commits in independent agent loops."}
```

**Parameters:**

| Parameter   | Type   | Required | Description                      |
|-------------|--------|----------|----------------------------------|
| `repo_path` | string | yes      | Path to the git repository       |
| `message`   | string | yes      | Commit message                   |
| `paths`     | array  | yes      | File paths to stage before committing |

No `amend` parameter — agents that need amend use the existing `commit` tool.

**Annotations:**

```go
ReadOnlyHint:    protocol.BoolPtr(false)
DestructiveHint: protocol.BoolPtr(false)
IdempotentHint:  protocol.BoolPtr(false)
OpenWorldHint:   protocol.BoolPtr(false)
```

**Tool Mappings:**

```go
MapsTools: []command.ToolMapping{
    {Replaces: "Bash", CommandPrefixes: []string{"git commit"}, UseWhen: "creating a new commit"},
}
```

Overlaps with the existing `commit` tool's mapping intentionally. Both intercept
`git commit` bash calls.

## Execution Flow

1. Run `git add -- <paths>` to stage the specified files.
2. Run `git diff --numstat --cached` to capture stat-only diff of what's staged.
3. Run `git commit -m <message>` to create the commit.
4. Run `git status --porcelain=v2 --branch` to capture post-commit working tree state.
5. Call `git.DetectInProgressState` to attach any in-progress state (rebase, merge, etc.) to the status result, matching the existing `status` tool's behavior.
6. Return composite result.

Steps are sequential. If step 1 fails, return error immediately. If step 3
fails, return the error alongside the status so the agent sees what went wrong
without a follow-up call.

## Response Type

```go
type TryCommitResult struct {
    Commit CommitResult  `json:"commit"`
    Staged []DiffStat    `json:"staged"`
    Status StatusResult  `json:"status"`
}
```

- `commit` — the full `CommitResult` struct (status, branch, hash, subject).
  See `git/types.go` for the canonical type.
- `staged` — numstat of what was committed (file paths + additions/deletions).
  Stat-only, no patch content and no `DiffSummary` — the agent wrote the code
  and doesn't need the full diff or aggregate counts echoed back.
- `status` — full `StatusResult` after the commit, including `State` from
  `DetectInProgressState`. Shows any remaining unstaged or untracked files so
  the agent sees if it missed something.

## Error Handling

Both error cases use `command.TextErrorResult` (matching existing tool patterns):

- `git add` failure: return `TextErrorResult`, do not attempt commit.
- `git commit` failure (nothing staged, GPG error, hook rejection): run status
  after the failure, then return a `TryCommitResult` with an empty `Commit`
  field and populated `Staged`/`Status` fields so the agent can diagnose
  without a follow-up call. Return this as `command.JSONResult` (not an error)
  so the structured data is preserved.

## What Does Not Change

All existing tools (`commit`, `status`, `diff`, `log`, `add`, `show`, `blame`,
etc.) remain unchanged. `try_commit` is additive. Agents can adopt it gradually
or continue using the individual tools when they need fine-grained control.

## Implementation

One new file: `packages/grit/internal/tools/try_commit.go`.

- Define `registerTryCommitCommands(app)` following existing pattern.
- Add `TryCommitResult` to `packages/grit/internal/git/types.go`.
- Register in `registry.go` via `registerTryCommitCommands(app)`.
- Reuse existing `git.Run`, `git.ParseStatus`, `git.ParseDiffNumstat`,
  `git.ParseCommit` functions — no new git parsing code needed.

## Testing

- Unit test the handler with a temp git repo: stage + commit + verify composite
  result structure.
- Test error case: paths that don't exist, nothing to commit after add.
- Existing tests for `commit`, `status`, `diff`, `add` remain unchanged.
