# Per-Package Hook Design

## Problem

Purse-first's central PreToolUse hook routes all tool calls through a single
binary that must be the marketplace build to discover mappings. The standalone
CLI build has no `share/purse-first/` directory, so hooks silently stop
intercepting. This coupling between the router and the marketplace build is
fragile and creates confusing failures.

More fundamentally, mapping logic belongs to each package, not to a central
router. Grit knows how to intercept git commands. Lux knows how to intercept
file reads for LSP-supported languages. Purse-first shouldn't need to know
about any of them.

## Design

Each package generates its own `hooks/hooks.json` at build time using Claude
Code's native plugin hook system. Claude Code auto-discovers these hooks when
the plugin is installed. No central purse-first hook exists.

### Build-Time Generation

Each package already declares tool mappings in Go source:

```go
MapsTools: []command.ToolMapping{
    {Replaces: "Bash", CommandPrefixes: []string{"git status"}, UseWhen: "checking repository status"},
}
```

The existing `generate-plugin` command (run during nix build) is extended to
also emit:

- `hooks/hooks.json` — Claude Code plugin hook configuration
- `hooks/pre-tool-use` — Shell script that delegates to the package binary

### Generated hooks/hooks.json

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "'${CLAUDE_PLUGIN_ROOT}/hooks/pre-tool-use' hook",
            "timeout": 5
          }
        ]
      }
    ]
  }
}
```

The `matcher` is derived from the unique set of `Replaces` values across all
commands in the package (e.g., `"Bash"` for grit, `"Read|Grep|Glob|Bash"` for
lux).

### Generated hooks/pre-tool-use

A thin shell wrapper with absolute nix store paths baked in (following the
existing chix pattern):

```bash
#!/nix/store/...-bash-.../bin/bash
set -euo pipefail
exec /nix/store/...-grit-0.1.0/bin/grit "$@"
```

### Package Binary Hook Subcommand

Each package's `hook` subcommand:

1. Reads HookInput JSON from stdin (tool_name, tool_input, cwd)
2. Extracts the command string or file path from tool_input
3. Matches against its own ToolMapping declarations (in memory, no file loading)
4. Returns deny with MCP tool suggestions, or allows passthrough (exit 0)

The matching logic moves from purse-first's `internal/mapping/` into go-mcp's
`command` package so each package binary can self-match.

MCP tool name format: `mcp__plugin_{server}_{server}__{tool}` (unchanged).

### Plugin Cache Layout

After installation:

```
~/.claude/plugins/cache/purse-first/grit/0.1.0/
  plugin.json
  hooks/
    hooks.json
    pre-tool-use
  skills/
    ...
```

Claude Code auto-discovers `hooks/hooks.json` at session start.

## Changes By Component

| Component | Change |
|-----------|--------|
| go-mcp (`libs/go-mcp`) | `generate-plugin` emits `hooks/` dir. Matching logic moves here from purse-first. |
| Package nix builds | `postInstall` output now includes `hooks/` alongside `plugin.json` |
| purse-first CLI | Remove `hook`, `post-hook`, `session-end` subcommands. Remove `install-local` hook registration. Remove `internal/hook/` and `internal/mapping/`. |
| Marketplace build | `symlinkJoin` collects `hooks/` subdirectory (no changes needed) |
| Plugin cache | Claude Code symlinks hooks into cache, auto-discovers at session start |

## Not Changing

- ToolMapping declarations in package source code (same API)
- plugin.json structure (no hooks field added)
- Marketplace generation
- Skill discovery

## Known Concerns

**N hook processes per tool use.** Each installed package with PreToolUse hooks
spawns a process per tool call. With 4 MCP packages (grit, lux, get-hubbed,
chix), that's 4 process spawns per tool use. Claude Code runs plugin hooks in
parallel, so latency is bounded by the slowest hook, but the overhead may be
noticeable. Mitigation options to explore later:

- Single dispatcher binary
- Matcher-based filtering (Claude Code only invokes hooks whose matcher matches)
- Prompt-based hooks for simpler cases

## Removed Functionality

- Purse-first central PreToolUse/PostToolUse/Stop hooks
- `internal/hook/` package (handler, install, uninstall)
- `internal/mapping/` package (loader, matching)
- `.purse-first/` project-local and `~/.local/state/purse-first/` global override directories
- `purse-first hook`, `post-hook`, `session-end` CLI subcommands
- `install-local` hook registration in `.claude/settings.json`
