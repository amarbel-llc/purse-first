---
status: experimental
date: 2026-03-04
promotion-criteria: >
  default boundary-notify to true after at least two weeks of real-world use
  without reports of false positives that broke a legitimate workflow
---

# Worktree Boundary Enforcement

## Motivation

When Claude Code runs inside a spinclass-managed worktree, it has no inherent
awareness of the worktree boundary. It will happily read, write, or edit files
in the parent repo, sibling worktrees, or unrelated paths. This is usually
unintentional — the agent drifts when it searches upward for context.

The goal is to give Claude a lightweight signal when it reaches outside its
lane, without breaking legitimate cross-boundary operations the user explicitly
requested.

## Design Decision: Notify, Not Deny

The initial design blocked tool calls that violated the boundary (`decision:
"block"`). This was reverted in favor of notification (`permissionDecision:
"allow"` + `additionalContext`).

Rationale:

- **Fail-open is safer.** A deny on a path-detection false positive (e.g. a
  symlink, a non-file argument in a Bash command) silently kills the tool call.
  The user sees a confusing refusal with no recourse.
- **Claude can override.** The user may legitimately say "also check the parent
  repo's README." A denial forces the user to disable the hook entirely; a
  notification lets Claude honor the request while still being aware of the
  boundary.
- **Hooks are not a security boundary.** The hook runs in Claude's process.
  Blocking here provides no security guarantee. The purpose is guidance, not
  enforcement.

## Interface

### Activation

Opt-in via sweatfile `[experimental]` block:

```toml
[experimental]
boundary-notify = true
```

Default is off. The feature will be promoted to default-on once confidence in
false-positive rate is established.

### Hook Invocation

spinclass installs itself as a Claude Code PreToolUse hook. On each tool call,
the hook:

1. Detects whether the CWD is inside a worktree.
2. If not in a worktree, exits with no output (no-op).
3. If in a worktree, sets the boundary to the worktree's top-level path.
4. Checks whether the tool's target path(s) are inside the boundary or an
   allowed path.
5. If a violation is found and `boundary-notify` is enabled, outputs a JSON
   notification.

### Boundary Detection

A directory is a worktree if it contains a `.git` **file** (not a directory).
The main repo checkout has a `.git` directory; each worktree has a `.git` file
pointing to the common object store.

`detectBoundary` walks from CWD to the git top-level and returns the
top-level path if it is (or contains) a worktree.

### Path Extraction by Tool

| Tool | Path extracted from |
|------|---------------------|
| `Read`, `Write`, `Edit` | `tool_input.file_path` |
| `Glob`, `Grep` | `tool_input.path` |
| `Bash` | absolute path tokens (strings starting with `/`) in `tool_input.command` |
| `Task` | not checked (always allowed) |

Tools not in this table produce no paths and are always allowed.

The Bash extractor is intentionally conservative: it only extracts tokens that
look like absolute paths. Relative paths in Bash commands are not checked.

### Always-Allowed Paths

Regardless of the boundary, the following paths are always allowed:

- `~/.claude` — Claude's own configuration and memory directories

### Notification Format

When a violation is detected:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "allow",
    "additionalContext": "worktree boundary violation: Read /repo/.worktrees/other/file.go is outside /repo/.worktrees/current\nActivity outside the worktree should only be performed if the user explicitly requested it. Otherwise, work exclusively within the worktree."
  }
}
```

Multiple violations in a single tool call are joined with newlines before the
guidance message.

## Examples

**Read inside boundary — no output:**

```
CWD: /repo/.worktrees/my-branch
boundary: /repo/.worktrees/my-branch
tool: Read { file_path: "/repo/.worktrees/my-branch/main.go" }
→ no output
```

**Read outside boundary — notification:**

```
CWD: /repo/.worktrees/my-branch
boundary: /repo/.worktrees/my-branch
tool: Read { file_path: "/repo/main.go" }
→ allow + additionalContext: "worktree boundary violation: Read /repo/main.go is outside /repo/.worktrees/my-branch\n..."
```

**Read ~/.claude — always allowed:**

```
tool: Read { file_path: "/home/user/.claude/CLAUDE.md" }
→ no output
```

**boundary-notify disabled — always no output:**

```toml
# sweatfile has no [experimental] block
```

```
→ no output regardless of path
```

## Limitations

- **Bash path extraction is shallow.** Only absolute path tokens (space-split
  words starting with `/`) are extracted. Paths embedded in quoted strings,
  variable expansions, or heredocs are not detected.
- **No relative path checking for Bash.** `cat ../README.md` in a Bash command
  is not flagged even if it resolves outside the boundary.
- **Task subagents are unchecked.** Tool calls made by a Task agent have their
  own hook invocations, so boundary checks apply to the subagent's own tool
  calls. The Task tool itself is not checked.
- **Opt-in only.** The feature is off by default. Users who do not add the
  sweatfile config receive no boundary guidance.
- **Not a security control.** The hook is Claude-side and provides no
  enforcement guarantee. It is guidance only.

## More Information

- Implementation: `packages/spinclass/internal/hooks/hooks.go`
- Hook registration: `packages/spinclass/internal/hooks/cmd.go`
- Worktree detection: `packages/spinclass/internal/worktree/worktree.go` (`IsWorktree`)
- Sweatfile opt-in: `packages/spinclass/internal/sweatfile/sweatfile.go` (`BoundaryNotifyEnabled`)
