# Stop Hook Support in Sweatfile

## Summary

Add a `stop_hook` field to the sweatfile that runs a command when Claude's main
agent attempts to stop. If the command fails, the agent is blocked from stopping
and presented with the failure output. On the second invocation within the same
session, the hook approves unconditionally to avoid infinite loops.

## Sweatfile Field

```toml
stop_hook = "just test"
```

Single string. Merged like other fields: nil = inherit, empty string = clear,
non-empty = override. Parsed and merged via the existing `Sweatfile` struct and
`Merge()` function.

## Routing

`spinclass hooks` is the single entrypoint for all hook types. It reads
`hook_event_name` from the stdin JSON and dispatches:

- `"PreToolUse"` -> existing boundary enforcement (unchanged)
- `"Stop"` -> stop hook logic (new)
- anything else -> no-op (exit 0)

## Hook Registration

`ApplyClaudeSettings` writes both hook types into
`.claude/settings.local.json` when inside a worktree. The `Stop` entry is only
written when `stop_hook` is non-empty in the merged sweatfile.

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Read|Write|Edit|Glob|Grep|Bash|Task",
        "hooks": [{ "type": "command", "command": "spinclass hooks" }]
      }
    ],
    "Stop": [
      {
        "matcher": "*",
        "hooks": [{ "type": "command", "command": "spinclass hooks" }]
      }
    ]
  }
}
```

## Stop Hook Flow

1. Parse stdin JSON -> extract `session_id` and `hook_event_name`.
2. Check sentinel file at `$TMPDIR/stop-hook-<session_id>`.
   - If exists -> approve immediately (exit 0, no output).
3. Load sweatfile hierarchy from cwd to get `stop_hook` command.
   - If empty -> approve (no stop hook configured).
4. Run command via `sh -c "<stop_hook>"` with cwd as working directory.
5. Command succeeds -> approve (exit 0, no output).
6. Command fails -> write combined stdout+stderr to
   `$TMPDIR/stop-hook-<session_id>`, return block decision as JSON on stdout.

## Block Decision Output

```json
{
  "decision": "block",
  "reason": "stop_hook command failed: just test",
  "systemMessage": "Stop hook failed. Output written to /tmp/.../stop-hook-<id>. Review the output and address the failures before completing."
}
```

## Sentinel Behavior

- Written only on failure (first block).
- Keyed by `session_id` alone (one check per session).
- On second invocation, sentinel exists -> approve unconditionally.
- Sentinel lives in `$TMPDIR`, outside the worktree, no gitignore needed.

## Input Struct Changes

The existing `hookInput` struct in `hooks.go` gains two fields:

```go
type hookInput struct {
    HookEventName string         `json:"hook_event_name"`
    SessionID     string         `json:"session_id"`
    ToolName      string         `json:"tool_name"`
    ToolInput     map[string]any `json:"tool_input"`
    CWD           string         `json:"cwd"`
}
```

## Sweatfile Merge Semantics

`StopHook` is a `*string` (pointer) to distinguish unset from empty:

- `nil` -> inherit from parent
- `""` (empty string) -> clear (disable stop hook)
- non-empty -> override

## Changes Required

1. `sweatfile.go` - Add `StopHook *string` field, update `Merge()`.
2. `hooks.go` - Add `hook_event_name`/`session_id` to input struct, route on
   event name, add `runStopHook()`.
3. `apply.go` - Pass `StopHook` to `ApplyClaudeSettings`, conditionally write
   `Stop` hook entry. Sweatfile needs to be available to `ApplyClaudeSettings`.
4. `cmd.go` - No changes needed (already delegates to `Run()`).
5. Tests for each layer.
