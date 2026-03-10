# Grit Hard Reset Command Design

## Problem

The grit MCP server's `reset` command is explicitly soft-only (unstage files or
`git reset --soft`). When a user tries `git reset --hard` via Bash, the hook
denies it in favor of the grit `reset` tool, which cannot perform the operation.

## Solution

Add a separate `hard_reset` command that runs `git reset --hard <ref>`, with
main/master branch protection consistent with push and rebase.

## Design

### Command: `hard_reset`

- **File**: `packages/grit/internal/tools/hard_reset.go`
- **Registration**: `registerHardResetCommands(app)` in `registry.go`

### Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `repo_path` | string | yes | Path to the git repository |
| `ref` | string | yes | Target ref (HEAD, origin/main, HEAD~3, SHA) |

### Safety

- Blocked on `main`/`master` branches (same pattern as push force and rebase)
- `DestructiveHint: true` in tool annotations

### Hook Mapping

`MapsTools: {Replaces: "Bash", CommandPrefixes: ["git reset --hard"]}`

The existing `reset` command maps `git reset` broadly. The new command maps
`git reset --hard` specifically. More-specific prefixes match first.

### Return Type

Uses existing `MutationResult` with `Status: "hard_reset"` and `Ref` field.

### Server Instructions

Update the instructions string in `main.go` to include "hard reset" in the
capabilities list.

## Files Changed

1. `packages/grit/internal/tools/hard_reset.go` (new)
2. `packages/grit/internal/tools/registry.go` (add registration call)
3. `packages/grit/cmd/grit/main.go` (update instructions string)
