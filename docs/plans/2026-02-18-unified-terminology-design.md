# Unified Terminology Design

## Problem

purse-first describes itself inconsistently across the codebase. The same
concepts are called different things in different places:

- "MCP-first tool routing for Claude Code" (CLI short description)
- "MCP servers and tool routing for Claude Code, built with Nix" (marketplace-config.json)
- "Plugin integration skills" (marketplace-config.json purse-first entry)
- "plugin" / "MCP server" / "tool" used interchangeably for adopters

The project has no root README.md or CLAUDE.md, so there is no canonical
description of what purse-first is.

## Decisions

### Canonical one-liner

> purse-first is a package framework for bundling CLIs, MCP servers, and skills
> into composable, Nix-built packages for humans and agents like Claude Code.

### Core terminology

| Term | Definition |
|------|-----------|
| **package** | A Nix derivation that conforms to the Purse-First Protocol. Replaces the overloaded word "plugin." |
| **MCP package** | A package that ships an MCP server and optional tool mappings (grit, get-hubbed, lux). |
| **Skill package** | A package that ships skills only, no MCP server (robin, tap-dancer, bob). |
| **MCP + Skill package** | A package that ships both (chix). |
| **marketplace** | An aggregation of packages into a single Claude Code plugin — the `.claude-plugin/` output. |
| **bob** | purse-first's own skill package — ships the plugin-mcp, context-saving, and go-cli-framework skills. |

### Three layers of purse-first

| Layer | Description |
|-------|-------------|
| **Protocol** | A convention for Nix derivations to declare Claude Code capabilities via well-known paths under `share/purse-first/`. |
| **CLI** | The `purse-first` binary — implements tool routing (hooks), marketplace generation, installation, and validation. |
| **Libraries** | `go-mcp` and `rust-mcp` — building blocks for creating MCP servers that conform to the protocol. |

### Reserved uses of "plugin"

The word "plugin" is retained only when referring to:

- Claude Code's own `.claude-plugin/` directory and `claude plugin` CLI (their terminology, not ours)
- The filename `plugin.json` (deferred breaking change)
- The `mcp__plugin_` tool name prefix (Claude Code convention)
- Go/Rust type and function names (deferred breaking change, TODO-marked)

## Scope

### Docs to create

- `README.md` — canonical project description
- `CLAUDE.md` — project instructions for Claude Code

### Docs to rewrite (plugin → package)

- `docs/purse-first-protocol.md` — full rewrite of terminology table and all
  prose; "plugin" → "package" throughout
- `skills/plugin-mcp/SKILL.md` — "plugin" → "package" in prose and section
  headers; skill name and directory stay as-is (identifiers)
- `marketplace-config.json` — update top-level and per-package descriptions
- `.claude-plugin/plugin.json` — update description string

### CLI strings to update (cmd/purse-first/main.go)

| Location | Current | Updated |
|----------|---------|---------|
| root `Short` | "MCP-first tool routing for Claude Code" | Uses canonical one-liner (short form) |
| `install` `Short` | "Install purse-first marketplace and plugins into Claude Code" | "...marketplace and packages..." |
| `generate-marketplace` `Short` | "...from discovered plugins" | "...from discovered packages" |
| `generate-marketplace` stderr | "(%d plugins)" | "(%d packages)" |
| `post-hook` `Short` | "fires plugin notifications" | "fires package notifications" |
| `session-end` `Short` | "fires plugin stop notifications" | "fires package stop notifications" |
| `validate` `Short` | "Validate plugin, mapping, or marketplace documents" | "Validate package, mapping, or marketplace documents" |
| `validate` `Long` | "Validate Claude Code plugin documents" | "Validate purse-first package documents" |
| `--plugins-dir` help | "directory containing plugin.json files" | "directory containing package manifest files" |
| Error messages | "discovering plugins", "generating local plugin" | "discovering packages", "generating local package manifest" |

The `--type plugin` flag value stays (matches the `plugin.json` filename).

### TODO markers for deferred breaking changes

Add `// TODO(terminology): rename X → Y when breaking change lands` to:

**Go types and functions:**
- `internal/marketplace/types.go`: `Plugin`, `PluginMeta`, `DiscoveredPlugin`
- `internal/marketplace/generate.go`: `DiscoverPlugins`
- `internal/mcp/marketplace.go`: `DiscoverPlugins`, `discoverFromPluginDir`
- `internal/localplugin/`: package name
- `purse/writer.go`: `Plugin` type, `WritePlugin`
- `libs/go-mcp/purse/`: `Plugin` type, `WritePlugin`

**Filenames (protocol-level breaking change):**
- `plugin.json` → `package.json` (or `package.toml`)
- `share/purse-first/<name>/plugin.json` path convention

**Library names (breaking change):**
- `libs/go-mcp/` → TBD (e.g., `go-purse`)
- `libs/rust-mcp/` → TBD
- Go module path `code.linenisgreat.com/purse-first/libs/go-mcp`

### Not changing

- `plugin.json` filename
- Go/Rust type names and function signatures (TODO-marked only)
- Skill directory names (`plugin-mcp/`)
- The word "plugin" when it refers to Claude Code's `.claude-plugin/` concept
- `mcp__plugin_` prefix in MCP tool names
- Library directory names (TODO-marked only)
