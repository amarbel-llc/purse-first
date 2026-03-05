---
status: accepted
date: 2026-03-04
promotion-criteria: n/a — shipped as the only hook architecture; supersedes the
  abandoned central hook design
---

# Per-Package Hook Architecture

## Motivation

The initial hook design used a single central hook binary shared by all
installed packages. When a PreToolUse event fired, the central binary had to
load and consult every package's tool mappings.

This created coupling: adding a new package required updating the central hook.
It also conflicted with the purse-first protocol's per-package isolation model,
where each package in `share/purse-first/` is independently installable and
carries its own `hooks/` directory.

The pivot to per-package hooks aligned hook generation with the existing package
lifecycle and eliminated the shared coordination point.

## Design

### Build-Time Generation

When a Go MCP package is built (`generate-plugin <dir>`), `App.GenerateAll()`
calls `GenerateHooks()`. This writes two files under `<dir>/<app.Name>/hooks/`:

**`hooks.json`** — Claude Code hook configuration:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash|Read",
        "hooks": [
          {
            "type": "command",
            "command": "${CLAUDE_PLUGIN_ROOT}/hooks/pre-tool-use",
            "timeout": 5
          }
        ]
      }
    ]
  }
}
```

The `matcher` is a pipe-joined sorted list of every unique `Replaces` value
across all non-hidden commands in the app. This limits hook invocations to only
the tool names the package can actually intercept.

**`pre-tool-use`** — a shell script:

```sh
#!/bin/sh
exec '/nix/store/.../bin/<package>' hook
```

The script exec's the package binary itself with the `hook` argument. The hook
binary is the MCP server binary — no separate process is needed.

Packages with no `MapsTools` declarations produce no hook files.

### Runtime: `HandleHook`

When Claude Code fires a PreToolUse event, `pre-tool-use` runs, which calls
`App.HandleHook(os.Stdin, os.Stdout)`.

`HandleHook`:

1. Decodes the JSON hook input from stdin (`tool_name`, `tool_input`).
2. Collects all `ToolMapping` declarations from all registered commands.
3. Extracts `file_path`/`path`/`pattern` from `tool_input` (for file-based tools).
4. Extracts `command` from `tool_input` (for Bash).
5. For Bash, splits the command into simple sub-commands via `extractSimpleCommands`.
6. Iterates mappings, calling `FindToolMatch` for each.
7. On the first match, writes a deny decision to stdout and returns.
8. If no mapping matches, writes nothing (implicit allow).

Errors in decoding return `nil` — the hook always fails open.

### Match Logic: `FindToolMatch`

A `ToolMapping` matches when:

1. `Replaces` equals the incoming `tool_name`, AND
2. one of:
   - No `Extensions` and no `CommandPrefixes` → catch-all (matches any invocation of that tool)
   - `Extensions` non-empty and `file_path` matches one of the extensions (case-insensitive)
   - `CommandPrefixes` non-empty and `command` starts with one of the prefixes

### Denial Output

```json
{
  "hookSpecificOutput": {
    "permissionDecision": "deny",
    "permissionDecisionReason": "Use the MCP tool instead:\n- mcp__plugin_grit_grit__status: <UseWhen text>"
  }
}
```

The MCP tool name is derived as `mcp__plugin_<app.Name>_<app.Name>__<commandName>`.
The reason includes the `UseWhen` field from the matching `ToolMapping`.

## Interface

### Declaring Tool Mappings

```go
app.AddCommand(&command.Command{
    Name: "status",
    MapsTools: []command.ToolMapping{
        {
            Replaces:        "Bash",
            CommandPrefixes: []string{"git status"},
            UseWhen:         "running git status",
        },
    },
    // ...
})
```

| Field | Purpose |
|-------|---------|
| `Replaces` | Claude Code built-in tool name to intercept |
| `Extensions` | file extensions for file-based tools (e.g. `[".go", ".py"]`) |
| `CommandPrefixes` | bash command prefixes (e.g. `["git status", "git diff"]`) |
| `UseWhen` | human-readable description shown in the denial reason |

Either `Extensions` or `CommandPrefixes` may be empty (but not both, unless
catch-all is intended). They are checked independently; only one needs to match.

## Limitations

- **No cross-package coordination.** Each package's hook only knows its own
  mappings. If two packages both map the same Bash prefix, both hooks fire and
  both deny. Claude sees two denial reasons.
- **Static matcher.** The `hooks.json` matcher is generated at build time from
  the union of `Replaces` values. Dynamic tool name registration is not
  supported.
- **Bash splitting is shallow.** `extractSimpleCommands` splits on common shell
  operators but does not fully parse shell syntax. Complex pipelines or
  subshells may not match as expected.
- **No extension matching for Bash.** `Extensions` are only checked when
  `file_path` is present. Bash commands that reference files by extension are
  not matched via `Extensions`.

## More Information

- Hook generation: `libs/go-mcp/command/generate_hooks.go`
- Hook handler: `libs/go-mcp/command/hook.go`
- Match logic: `libs/go-mcp/command/match.go`
- Command declaration: `libs/go-mcp/command/command.go` (`ToolMapping`, `Command.MapsTools`)
- Shell parsing: `libs/go-mcp/command/shellparse.go`
