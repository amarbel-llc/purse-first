# Purse-First Plugin Protocol

**Version:** 0.1.0
**Status:** Draft
**Authors:** friedenberg

## Abstract

This document specifies how Nix flake derivations expose Claude Code plugin
metadata, MCP server declarations, tool routing mappings, and skills through
well-known paths in the `share/` directory of the derivation output. The
protocol follows the conventions established by the XDG Base Directory
Specification, the freedesktop shared MIME database, and the Nix `share/`
output convention.

## Motivation

Claude Code plugins bundle MCP servers, tool-routing rules, and skills.
Today each aggregator must know how to extract this metadata from each
upstream project. A standard `share/` layout lets any Nix derivation
advertise its Claude Code capabilities declaratively — without coordination
between producer and consumer, without wrapper scripts, and without a
central registry.

Goals:

- A single derivation output is self-describing: consumers can discover
  everything by globbing well-known paths.
- Individual plugins compose via `symlinkJoin` without conflicts.
- The protocol is language-agnostic: a Go binary, a Rust binary, and a
  static JSON file all produce the same directory shape.
- Skills, mappings, and future extension types are opt-in and independently
  discoverable.

## Terminology

| Term | Definition |
|------|-----------|
| **derivation output** | The Nix store path produced by a derivation, conventionally referred to as `$out`. |
| **plugin** | A named unit of Claude Code functionality. One derivation produces exactly one plugin. |
| **MCP server** | A Model Context Protocol server exposed by a plugin. |
| **mapping** | A tool-routing rule that redirects a built-in Claude Code tool to an MCP tool. |
| **skill** | A Markdown document (with YAML frontmatter) that teaches Claude Code how to use a plugin. |
| **marketplace** | An aggregation of multiple plugins into a single installable derivation with a `.claude-plugin/` directory. |

## 1. Base Directory

All plugin metadata MUST reside under:

```
$out/share/purse-first/<plugin-name>/
```

Where `<plugin-name>` is the plugin identifier: lowercase ASCII, alphanumeric
with hyphens, matching the pattern `^[a-z][a-z0-9-]*$`.

This mirrors the freedesktop convention of
`$out/share/<project>/<resource-type>` and the Nix idiom of placing
read-only data under `share/`.

### 1.1. Relationship to `bin/`

Plugin binaries are installed to `$out/bin/` as usual. The `command` field
in `plugin.json` references a bare binary name (not an absolute path).
Consumers resolve it to `<root>/bin/<command>` where `<root>` is the
ancestor of the `share/` directory.

This separation means `share/purse-first/` is purely declarative data and
`bin/` is purely executable code.

## 2. Required Files

### 2.1. `plugin.json` — Plugin Manifest

```
$out/share/purse-first/<plugin-name>/plugin.json
```

This file MUST exist. It declares the plugin's identity and its MCP server
configuration.

**Schema:** `https://anthropic.com/claude-code/plugin.schema.json`

**Minimal example:**

```json
{
  "name": "grit",
  "mcpServers": {
    "grit": {
      "type": "stdio",
      "command": "grit"
    }
  }
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Plugin identifier. MUST match the directory name under `share/purse-first/`. |
| `mcpServers` | object | No | Map of server-name to MCP server config. |
| `description` | string | No | Human-readable description. |
| `version` | string | No | SemVer version. |
| `author` | object | No | `{name, email?, url?}` |
| `homepage` | string | No | Project homepage URL. |
| `repository` | string | No | Source repository URL. |
| `license` | string | No | SPDX license identifier. |
| `keywords` | array | No | Search keywords. |

**MCP Server entry:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | Transport type. Currently always `"stdio"`. |
| `command` | string | Yes | Bare binary name resolved from `$out/bin/`. |
| `args` | array | No | Arguments passed to the binary. |

**Invariant:** `plugin.name` MUST equal the directory name. Consumers
SHOULD reject manifests where the name does not match.

## 3. Optional Files

### 3.1. `mappings.json` — Tool Routing

```
$out/share/purse-first/<plugin-name>/mappings.json
```

Declares tool-routing rules that redirect built-in Claude Code tools (e.g.,
`Bash`, `Grep`, `Read`) to MCP server tools exposed by this plugin.

**Example:**

```json
{
  "server": "lux",
  "mappings": [
    {
      "replaces": "Grep",
      "extensions": [".go", ".rs", ".ts"],
      "tools": [
        {
          "name": "lsp_references",
          "use_when": "finding usages of a symbol"
        },
        {
          "name": "lsp_workspace_symbols",
          "use_when": "searching for symbol definitions by name"
        }
      ],
      "reason": "Use LSP-backed tools for semantic code search instead of text-based grep."
    }
  ]
}
```

**Fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `server` | string | Yes | MCP server name this mapping refers to. MUST match a key in the plugin's `mcpServers`. |
| `mappings` | array | Yes | List of mapping entries. |

**Mapping entry:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `replaces` | string | Yes | Name of the built-in tool to intercept. |
| `extensions` | array | No | File extensions (with leading dot, case-insensitive) that scope this rule. If absent, matches all files. |
| `command_prefixes` | array | No | Shell command prefixes that scope this rule (for `Bash` tool interception). |
| `tools` | array | Yes | Suggested MCP tools to use instead. Each has `name` and `use_when`. |
| `reason` | string | Yes | Human-readable denial message shown to the agent. |

**Scoping:** When both `extensions` and `command_prefixes` are absent, the
mapping applies unconditionally to the replaced tool. When either is
present, the mapping applies only when the invocation matches.

### 3.2. `skills/` — Skill Documents

```
$out/share/purse-first/<plugin-name>/skills/<skill-name>/SKILL.md
```

Skills are Markdown files with YAML frontmatter that teach Claude Code how
to use the plugin. Each skill lives in a named subdirectory.

**Frontmatter:**

```yaml
---
name: Plugin MCP Integration
description: >
  This skill should be used when the user asks to add purse-first support
  to an MCP server project.
version: 0.1.0
---
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Human-readable skill name. |
| `description` | string | Yes | When and why this skill should be invoked. |
| `version` | string | No | SemVer version. |

The body of the Markdown document is the skill content — instructions,
examples, and reference material that Claude Code uses when the skill is
activated.

Skills MAY include supporting files in the same directory:

```
skills/<skill-name>/
├── SKILL.md
├── examples/
│   ├── plugin.json
│   └── flake-go.nix
└── references/
    └── existing-integrations.md
```

### 3.3. Future Extension Points

The following paths are reserved for future use:

| Path | Purpose |
|------|---------|
| `hooks/` | Pre/post tool-use hook definitions. |
| `commands/` | Custom slash-command definitions. |
| `agents/` | Subagent type definitions. |
| `output-styles/` | Output formatting rules. |

Consumers MUST ignore unrecognized directories under
`share/purse-first/<plugin-name>/`.

## 4. Discovery

### 4.1. Single Plugin Discovery

Given a derivation output at `$out`, a consumer discovers the plugin by:

1. Glob `$out/share/purse-first/*/plugin.json`
2. For each match, parse the JSON and validate `name` matches the directory.
3. Optionally read `mappings.json` from the same directory.
4. Optionally glob `skills/*/SKILL.md` from the same directory.

### 4.2. Aggregated Discovery (symlinkJoin)

When multiple plugin derivations are composed via `symlinkJoin`:

```nix
pkgs.symlinkJoin {
  name = "my-plugins";
  paths = [ grit-pkg lux-pkg chix-pkg ];
}
```

The resulting `$out/share/purse-first/` contains a symlink for each plugin
directory. Discovery uses the same glob pattern and works transparently.

**Conflict rule:** Two derivations MUST NOT produce the same
`<plugin-name>` directory. `symlinkJoin` will fail at build time if this
invariant is violated.

### 4.3. Runtime Resolution Order

At runtime, the consumer resolves the plugin root by:

1. `$PURSE_FIRST_PLUGINS_DIR` environment variable (if set and valid).
2. `<exe-dir>/../share/purse-first/` relative to the resolved (symlink-followed) binary path.

### 4.4. Binary Resolution

MCP server commands are resolved as:

```
<plugin-root>/../../bin/<command>
```

Where `<plugin-root>` is the `share/purse-first/` directory. This means the
aggregated derivation's `bin/` directory (populated by `symlinkJoin`) is the
lookup path.

## 5. Mapping Precedence

Tool-routing mappings are loaded from three sources in increasing priority:

| Priority | Source | Path Pattern |
|----------|--------|-------------|
| 1 (lowest) | Plugin-shipped | `$out/share/purse-first/<name>/mappings.json` |
| 2 | Global user | `$XDG_STATE_HOME/purse-first/*.json` |
| 3 (highest) | Project-local | `.purse-first/*.json` |

Higher-priority sources override lower-priority ones for the same
`(replaces, server)` pair. Users can override or disable plugin-shipped
mappings without modifying the derivation.

**XDG fallback:** `$XDG_STATE_HOME` defaults to `$HOME/.local/state`.

## 6. Marketplace Generation

A marketplace aggregates multiple plugins into a single `.claude-plugin/`
directory for consumption by `claude plugin validate` and `claude plugin
install`.

**Output layout:**

```
$out/
├── bin/
│   ├── purse-first
│   ├── grit
│   ├── lux
│   └── ...
├── share/
│   └── purse-first/
│       ├── grit/
│       │   ├── plugin.json
│       │   └── mappings.json
│       ├── lux/
│       │   ├── plugin.json
│       │   └── mappings.json
│       ├── nix/
│       │   └── plugin.json
│       └── purse-first/
│           ├── plugin.json
│           └── skills/
│               └── plugin-mcp/
│                   └── SKILL.md
└── .claude-plugin/
    └── marketplace.json
```

The marketplace is generated by scanning `share/purse-first/`, discovering
both MCP servers and skills per-plugin, merging the discovered data with a
`marketplace-config.json`, and writing the result to
`.claude-plugin/marketplace.json`.

**Schema:** `https://anthropic.com/claude-code/marketplace.schema.json`

## 7. Producing a Conformant Derivation

### 7.1. Pattern A: Generate at Build Time (Go, Rust with embedded metadata)

Add a hidden subcommand that writes `plugin.json` (and optionally
`mappings.json`) to a directory argument:

```
$out/bin/my-mcp generate-plugin $out/share/purse-first
```

In `flake.nix`:

```nix
postInstall = ''
  $out/bin/my-mcp generate-plugin $out/share/purse-first
'';
```

### 7.2. Pattern B: Static Files (any language)

Place `plugin.json` in the source tree and copy it during the build:

```nix
postInstall = ''
  mkdir -p $out/share/purse-first/my-mcp
  cp ${./plugin.json} $out/share/purse-first/my-mcp/plugin.json
'';
```

Or as a `runCommand` wrapper around an upstream derivation:

```nix
pkgs.runCommand "my-mcp-wrapped" {
  nativeBuildInputs = [ pkgs.makeWrapper ];
} ''
  mkdir -p $out/bin $out/share/purse-first/my-mcp
  makeWrapper ${upstream}/bin/my-mcp $out/bin/my-mcp
  cp ${./plugin.json} $out/share/purse-first/my-mcp/plugin.json
''
```

### 7.3. Validation

Producers SHOULD validate their output with:

```sh
claude plugin validate $out/.claude-plugin/
```

or by checking:

1. `plugin.json` is valid JSON matching the plugin schema.
2. `plugin.name` matches the parent directory name.
3. Every `mcpServers[*].command` has a corresponding `$out/bin/<command>`.

## 8. Complete Directory Reference

```
$out/
├── bin/
│   └── <binary>                              # executable
└── share/
    └── purse-first/
        └── <plugin-name>/                    # MUST match plugin.name
            ├── plugin.json                   # REQUIRED — plugin manifest
            ├── mappings.json                 # OPTIONAL — tool routing
            └── skills/                       # OPTIONAL — skill documents
                └── <skill-name>/
                    ├── SKILL.md              # REQUIRED per skill
                    ├── examples/             # OPTIONAL
                    └── references/           # OPTIONAL
```

## 9. Comparison with Prior Art

| Aspect | XDG Base Dirs | freedesktop shared-mime | Purse-First Protocol |
|--------|--------------|------------------------|---------------------|
| Base path | `$XDG_DATA_DIRS/...` | `$out/share/mime/` | `$out/share/purse-first/` |
| Discovery | Glob across dirs | `update-mime-database` | Glob `*/plugin.json` |
| Composition | `:` separated paths | merge-and-rebuild | `symlinkJoin` |
| Identity | directory name | MIME type | `plugin.name` = dirname |
| Schema validation | per-spec | XML DTD | JSON Schema |

## 10. Security Considerations

- **Store path immutability.** Nix store paths are read-only. Plugin
  manifests cannot be tampered with after build.
- **Binary provenance.** Because commands are resolved relative to the same
  derivation root, a plugin cannot reference binaries outside its closure.
- **Hook fail-open.** Tool routing hooks return success on I/O or parse
  errors to avoid blocking the agent. This is intentional: a broken mapping
  file should degrade gracefully, not deny all tool use.
- **User override.** Project-local mappings (`.purse-first/`) take highest
  precedence, ensuring users always have the final say over tool routing.

## 11. Versioning

This protocol uses the filename and schema `$id` for versioning. Breaking
changes require a new base directory (e.g., `share/purse-first-v2/`).
Non-breaking additions (new optional files, new `plugin.json` fields) are
backwards-compatible and do not require a version bump.

## Appendix A: JSON Schemas

- Plugin: `plugin.schema.json`
- Marketplace: `marketplace.schema.json`

Both are included in this repository and referenced by `$id` in the schema
documents.

## Appendix B: Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PURSE_FIRST_PLUGINS_DIR` | `<exe>/../share/purse-first/` | Override the plugin discovery root. |
| `XDG_STATE_HOME` | `$HOME/.local/state` | Base for user-level mapping overrides. |
| `XDG_CONFIG_HOME` | `$HOME/.config` | Base for user-level configuration. |
