# install-local --binary Design

**Date:** 2026-02-23
**Status:** Approved

## Problem

`install-local` assumes `.claude-plugin/plugin.json` already exists. Go MCP
packages like lux generate their plugin manifest at build time via
`app.GenerateAll()`, so there's no pre-existing `plugin.json` for local dev.
Lux also lacks a `package.toml`, unlike other packages.

## Solution

Two changes:

1. Add `package.toml` to lux
2. Add `--binary` flag to `install-local` that runs the binary's `_generate`
   command before the existing install steps

## Part 1: Lux package.toml

```toml
name = "lux"
description = "LSP Multiplexer that routes requests to language servers based on file type"

[author]
name = "friedenberg"

[mcp.lux]
command = "lux"
args = ["mcp-stdio"]
```

Metadata-only. The actual plugin.json generation uses `_generate` for local dev
and `GenerateAll` during Nix builds.

## Part 2: install-local --binary

### CLI Interface

```
purse-first install-local [--root <dir>] [--binary <name>]
```

- `--root` defaults to cwd
- `--binary` names the Go binary (e.g. `lux`), triggers `_generate` step

### Behavior

When `--binary` is set:

1. Run `go run ./cmd/<binary> _generate .claude-plugin/` via `os/exec`
2. Glob `.claude-plugin/share/purse-first/*/plugin.json` to find the generated
   manifest
3. Proceed with existing steps using the found plugin.json path

When `--binary` is not set, behavior is unchanged (reads existing
`.claude-plugin/plugin.json`).

### Updated Flow

With `--binary`:
```
TAP version 14
1..4
ok 1 - generate plugin.json via _generate (lux)
ok 2 - discover and update skills in plugin.json
ok 3 - install MCP servers to .claude/settings.json (1 server)
ok 4 - install hooks to .claude/settings.json
```

Without `--binary`:
```
TAP version 14
1..3
ok 1 - discover and update skills in plugin.json
ok 2 - install MCP servers to .claude/settings.json (N servers)
ok 3 - install hooks to .claude/settings.json
```

### API Change

```go
type InstallLocalOptions struct {
    Binary string // Go binary name under cmd/, triggers _generate
}

func InstallLocal(w io.Writer, root string, opts InstallLocalOptions) error
```

### Plugin.json Discovery

After `_generate` writes to `.claude-plugin/share/purse-first/<name>/plugin.json`,
glob for the file rather than hardcoding the name. This keeps install-local
decoupled from the package name.

### MCP Server Command Rewriting

Same as existing behavior: each `mcpServers` entry gets rewritten to
`go run ./cmd/<name> ...originalArgs`.

## Files Changed

| File | Change |
|------|--------|
| `packages/lux/package.toml` (new) | Package metadata for lux |
| `cmd/purse-first/main.go` | Add `--binary` flag to `install-local` |
| `internal/localplugin/generate.go` | Update `InstallLocal` signature, add generate step |
| `internal/localplugin/install_local_test.go` | Add test for `--binary` flow |

## What Stays

- `Generate(root, pluginPath)` still used for skill discovery on existing plugin.json
- `installMCPServers` unchanged except which plugin.json path it reads
- `hook.Install` unchanged
- Nix build path (`lux.nix` calling `_generate`) unchanged
