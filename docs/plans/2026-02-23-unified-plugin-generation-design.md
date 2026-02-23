# Unified Plugin.json Generation

## Problem

Plugin.json files are maintained in two places that can drift:

1. Manual `.claude-plugin/plugin.json` in source tree
2. Auto-generated `share/purse-first/{name}/plugin.json` during nix build

The lux MCP server was broken because `.claude-plugin/plugin.json` had
`["mcp", "stdio"]` while the actual subcommand is `mcp-stdio`. The binary knew
the right answer (`MCPArgs` on `command.App`), but a separate manual file
duplicated it incorrectly.

## Design

Eliminate all manual `plugin.json` files. Two generators produce them from
single sources of truth:

### Go MCP packages (grit, get-hubbed, lux)

**Source of truth:** `command.App` struct in Go code.

Extend `command.App` with:

```go
type App struct {
    // existing fields...
    PluginDescription string   // "description" in plugin.json
    PluginAuthor      string   // "author.name" in plugin.json
}
```

Extend `GeneratePlugin()` to include these fields in the output manifest.

Extend `GenerateAll()` to accept an optional skills source directory. When
provided, it copies skills into the output and adds `"skills"` entries to
plugin.json.

The `_generate` / `generate-plugin` commands gain an optional `--skills-dir`
flag passed by nix `postInstall` when skills exist alongside the source.

### Non-Go packages (chix, robin, tap-dancer, sandcastle)

**Source of truth:** `package.toml` at the package root.

```toml
name = "chix"
description = "Nix MCP server and skills for Claude Code"

[author]
name = "friedenberg"

[mcp.chix]
command = "chix"

[[hooks.PostToolUse]]
matcher = "Edit|Write"
command = "${CLAUDE_PLUGIN_ROOT}/hooks/format-nix"
timeout = 30
```

Skill-only example (robin):

```toml
name = "robin"
description = "Expert skill for BATS integration tests..."

[author]
name = "friedenberg"
```

Skills are auto-discovered from `{skills-dir}/*/SKILL.md`, never listed
manually.

### `purse-first generate-plugin` command

Replaces the deprecated `generate-local-plugin`. Reads `package.toml`,
auto-discovers skills, produces `share/purse-first/{name}/plugin.json`.

Flags:
- `--root` — package root containing `package.toml`
- `--output` — output directory (typically `$out`)
- `--skills-dir` — directory containing skills to discover and copy

### Nix build changes

Each package's nix expression calls the appropriate generator in `postInstall`:

| Package | Generator |
|---------|-----------|
| grit | `$out/bin/grit generate-plugin $out` (unchanged) |
| get-hubbed | `$out/bin/get-hubbed generate-plugin $out` (unchanged) |
| lux | `$out/bin/lux _generate $out` (unchanged) |
| chix | `purse-first generate-plugin --root ${src} --output $out --skills-dir ${src}/skills` |
| robin | `purse-first generate-plugin --root ${src} --output $out --skills-dir ${src}/skills` |
| tap-dancer | `purse-first generate-plugin --root ${src} --output $out --skills-dir ${src}/skills` |
| sandcastle | `purse-first generate-plugin --root ${src} --output $out` |

## Deletions

- All `packages/*/.claude-plugin/plugin.json` files
- `generate-local-plugin` command (replaced by `generate-plugin`)
- `localplugin.Generate()` function that reads from `.claude-plugin/plugin.json`

## Source-of-truth summary

| Package | Source | Generator | Output |
|---------|--------|-----------|--------|
| grit | `command.App` in Go | binary | `share/purse-first/grit/plugin.json` |
| get-hubbed | `command.App` in Go | binary | `share/purse-first/get-hubbed/plugin.json` |
| lux | `command.App` in Go | binary | `share/purse-first/lux/plugin.json` |
| chix | `package.toml` | `purse-first` | `share/purse-first/chix/plugin.json` |
| robin | `package.toml` | `purse-first` | `share/purse-first/robin/plugin.json` |
| tap-dancer | `package.toml` | `purse-first` | `share/purse-first/tap-dancer/plugin.json` |
| sandcastle | `package.toml` | `purse-first` | `share/purse-first/sandcastle/plugin.json` |
