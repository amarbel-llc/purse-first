# install-local Design

**Date:** 2026-02-19
**Status:** Proposed

## Problem

`generate-local-plugin` only discovers skills and updates `plugin.json`. Local
dev also requires installing MCP servers and hooks into the project-scoped
`.claude/settings.json`. These steps are currently manual or handled by
separate commands.

## Solution

Replace `generate-local-plugin` with `install-local`. The new command performs
three steps:

1. **Discover and update skills** — glob `skills/*/SKILL.md`, update
   `.claude-plugin/plugin.json` (existing `localplugin.Generate` behavior)
2. **Install MCP servers** — read `plugin.json` for `mcpServers`. For each
   declared server, write an entry to `.claude/settings.json` with command
   rewritten to `go run ./cmd/<binary>` (preserving declared args). Skip if no
   `mcpServers` declared.
3. **Install hooks** — call `hook.Install(binaryPath, project=true)` with
   `binaryPath` = `go run ./cmd/purse-first`

All output uses tap-dancer's `tap.Writer` for TAP-14 formatting.

## CLI Interface

```
purse-first install-local [--root <dir>]
```

- `--root` defaults to cwd

## MCP Server Command Rewriting

Each `mcpServers` entry in `plugin.json` has a relative command name (e.g.
`purse-first`). `install-local` rewrites this to:

```json
{
  "command": "go",
  "args": ["run", "./cmd/<name>", ...originalArgs]
}
```

Written to `.claude/settings.json` under `mcpServers`.

## TAP-14 Output

```
TAP version 14
1..3
ok 1 - discover and update skills in plugin.json
ok 2 - install MCP servers to .claude/settings.json (N servers)
ok 3 - install hooks to .claude/settings.json
```

Uses `github.com/amarbel-llc/tap-dancer/go` package.

## Files Changed

| File | Change |
|------|--------|
| `cmd/purse-first/main.go` | Replace `generate-local-plugin` with `install-local` |
| `internal/localplugin/generate.go` | Add `InstallLocal(w, root)` orchestrator |
| `internal/localplugin/mcp.go` (new) | MCP server installation to settings.json |
| `go.mod` / `gomod2nix.toml` | Add tap-dancer Go dependency |
| `justfile` | Remove `generate-local-plugin` recipe |

## What Stays

- `localplugin.Generate` remains (used by `mkMarketplace.nix` during Nix builds)
- `hook.Install` already supports `project=true`
- `install` command (marketplace install) is unaffected
