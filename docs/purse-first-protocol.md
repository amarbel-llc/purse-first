# Purse-First Package Protocol

**Version:** 0.1.0
**Status:** Draft
**Authors:** friedenberg

> **Note (#110):** Marketplace *aggregation* was removed from purse-first in
> #110. This protocol now describes a per-package convention only: each
> derivation produces one self-describing package under
> `share/purse-first/<name>/`. Aggregating those per-package outputs into a
> consumable plugin set is the job of external tools (e.g. clown), not
> purse-first.

## Abstract

This document specifies how Nix flake derivations expose Claude Code package
metadata, MCP server declarations, tool routing mappings, and skills through
well-known paths in the `share/` directory of the derivation output. One
derivation produces exactly one self-describing package; external aggregators
discover packages by globbing well-known paths. The protocol follows the
conventions established by the XDG Base Directory Specification, the
freedesktop shared MIME database, and the Nix `share/` output convention.

## Motivation

Claude Code packages bundle MCP servers, tool-routing rules, and skills.
Today each aggregator must know how to extract this metadata from each
upstream project. A standard `share/` layout lets any Nix derivation
advertise its Claude Code capabilities declaratively — without coordination
between producer and consumer, without wrapper scripts, and without a
central registry.

Goals:

- A single derivation output is self-describing: consumers can discover
  everything by globbing well-known paths.
- Individual packages compose via `symlinkJoin` without conflicts.
- The protocol is language-agnostic: a Go binary, a Rust binary, and a
  static JSON file all produce the same directory shape.
- Skills, mappings, and future extension types are opt-in and independently
  discoverable.

## Terminology

| Term | Definition |
|------|-----------|
| **derivation output** | The Nix store path produced by a derivation, conventionally referred to as `$out`. |
| **package** | A named unit of Claude Code functionality. One derivation produces exactly one package. The manifest filename `plugin.json` is retained for backwards compatibility. |
| **MCP server** | A Model Context Protocol server exposed by a package. |
| **mapping** | A tool-routing rule that redirects a built-in Claude Code tool to an MCP tool. |
| **skill** | A Markdown document (with YAML frontmatter) that teaches Claude Code how to use a package. |

## 1. Base Directory

All package metadata MUST reside under:

```
$out/share/purse-first/<package-name>/
```

Where `<package-name>` is the package identifier: lowercase ASCII, alphanumeric
with hyphens, matching the pattern `^[a-z][a-z0-9-]*$`.

This mirrors the freedesktop convention of
`$out/share/<project>/<resource-type>` and the Nix idiom of placing
read-only data under `share/`.

### 1.1. Relationship to `bin/`

Package binaries are installed to `$out/bin/` as usual. The `command` field
in `plugin.json` references a bare binary name (not an absolute path).
Consumers resolve it to `<root>/bin/<command>` where `<root>` is the
ancestor of the `share/` directory.

This separation means `share/purse-first/` is purely declarative data and
`bin/` is purely executable code.

## 2. Required Files

### 2.1. `plugin.json` — Package Manifest

```
$out/share/purse-first/<package-name>/plugin.json
```

This file MUST exist. It declares the package's identity and its MCP server
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
| `name` | string | Yes | Package identifier. MUST match the directory name under `share/purse-first/`. |
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
$out/share/purse-first/<package-name>/mappings.json
```

Declares tool-routing rules that redirect built-in Claude Code tools (e.g.,
`Bash`, `Grep`, `Read`) to MCP server tools exposed by this package.

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
| `server` | string | Yes | MCP server name this mapping refers to. MUST match a key in the package's `mcpServers`. |
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
$out/share/purse-first/<package-name>/skills/<skill-name>/SKILL.md
```

Skills are Markdown files with YAML frontmatter that teach Claude Code how
to use the package. Each skill lives in a named subdirectory.

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

### 3.3. `hooks/` — Per-Package Hooks

```
$out/share/purse-first/<package-name>/hooks/hooks.json
$out/share/purse-first/<package-name>/hooks/pre-tool-use
```

Packages that declare tool mappings ship per-package PreToolUse hooks.
The `hooks/hooks.json` follows the Claude Code hook format with a
`${CLAUDE_PLUGIN_ROOT}` reference for portable path resolution. The
`pre-tool-use` script is an executable wrapper that delegates to the
package binary's `hook` subcommand.

**hooks.json example:**

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
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

The matcher is built from the unique set of tool names replaced by the
package's mappings (e.g., `"Bash"`, `"Bash|Grep|Read"`). Hooks are
generated at build time by the package binary's `GenerateHooks` method.

There is no central hook infrastructure. Each package handles its own
tool routing independently.

### 3.4. Future Extension Points

The following paths are reserved for future use:

| Path | Purpose |
|------|---------|
| `commands/` | Custom slash-command definitions. |
| `agents/` | Subagent type definitions. |
| `output-styles/` | Output formatting rules. |

Consumers MUST ignore unrecognized directories under
`share/purse-first/<package-name>/`.

## 4. Discovery

### 4.1. Single Package Discovery

Given a derivation output at `$out`, a consumer discovers the package by:

1. Glob `$out/share/purse-first/*/plugin.json`
2. For each match, parse the JSON and validate `name` matches the directory.
3. Optionally read `mappings.json` from the same directory.
4. Optionally glob `skills/*/SKILL.md` from the same directory.

The same glob pattern works whether `$out` is a single package's derivation
output or an external aggregator's `symlinkJoin` of several packages — each
package directory is discovered transparently.

**Conflict rule:** Two derivations MUST NOT produce the same
`<package-name>` directory. When an external aggregator composes packages via
`symlinkJoin`, the build fails if this invariant is violated.

### 4.2. Runtime Resolution Order

At runtime, the consumer resolves the package root by:

1. `$PURSE_FIRST_PLUGINS_DIR` environment variable (if set and valid).
2. `<exe-dir>/../share/purse-first/` relative to the resolved (symlink-followed) binary path.

### 4.3. Binary Resolution

MCP server commands are resolved as:

```
<package-root>/../../bin/<command>
```

Where `<package-root>` is the `share/purse-first/` directory. The sibling
`bin/` directory — whether produced by the single package's build or
populated by an external aggregator's `symlinkJoin` — is the lookup path.

## 5. Tool Routing

### 5.1. Per-Package Hooks

Each package that declares tool mappings ships its own PreToolUse hook.
The hook is self-contained: the package binary reads hook input from
stdin, matches against its own `ToolMapping` declarations, and writes a
deny response when a match is found.

There is no central hook handler. Multiple packages can each have their
own hooks — they run in parallel via Claude Code's hook system.

### 5.2. Mapping Declarations

Tool mappings are declared in the package source code (via `ToolMapping`
structs on `Command` definitions) and serialized to `mappings.json` at
build time. The `mappings.json` serves as documentation and for external
tooling — the actual matching is performed by the package binary at
runtime via its `hook` subcommand.

## 6. Producing a Conformant Derivation

### 6.1. Pattern A: Generate at Build Time (Go, Rust with embedded metadata)

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

### 6.2. Pattern B: Static Files (any language)

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

### 6.3. Validation

Producers SHOULD validate their per-package output with:

```sh
purse-first validate $out/share/purse-first/<name>/.claude-plugin/plugin.json
```

or by checking:

1. `plugin.json` is valid JSON matching the plugin schema.
2. `plugin.name` matches the parent directory name.
3. Every `mcpServers[*].command` has a corresponding `$out/bin/<command>`.

## 7. Complete Directory Reference

```
$out/
├── bin/
│   └── <binary>                              # executable
└── share/
    └── purse-first/
        └── <package-name>/                   # MUST match plugin.name
            ├── plugin.json                   # REQUIRED — package manifest
            ├── mappings.json                 # OPTIONAL — tool routing
            ├── hooks/                        # OPTIONAL — per-package hooks
            │   ├── hooks.json                # PreToolUse matcher config
            │   └── pre-tool-use              # executable wrapper script
            └── skills/                       # OPTIONAL — skill documents
                └── <skill-name>/
                    ├── SKILL.md              # REQUIRED per skill
                    ├── examples/             # OPTIONAL
                    └── references/           # OPTIONAL
```

## 8. Comparison with Prior Art

| Aspect | XDG Base Dirs | freedesktop shared-mime | Purse-First Protocol |
|--------|--------------|------------------------|---------------------|
| Base path | `$XDG_DATA_DIRS/...` | `$out/share/mime/` | `$out/share/purse-first/` |
| Discovery | Glob across dirs | `update-mime-database` | Glob `*/plugin.json` |
| Composition | `:` separated paths | merge-and-rebuild | `symlinkJoin` |
| Identity | directory name | MIME type | `plugin.name` = dirname |
| Schema validation | per-spec | XML DTD | JSON Schema |

## 9. Security Considerations

- **Store path immutability.** Nix store paths are read-only. Package
  manifests cannot be tampered with after build.
- **Binary provenance.** Because commands are resolved relative to the same
  derivation root, a package cannot reference binaries outside its closure.
- **Hook fail-open.** Tool routing hooks return success on I/O or parse
  errors to avoid blocking the agent. This is intentional: a broken mapping
  file should degrade gracefully, not deny all tool use.
- **User override.** Project-local mappings (`.purse-first/`) take highest
  precedence, ensuring users always have the final say over tool routing.

## 10. Versioning

This protocol uses the filename and schema `$id` for versioning. Breaking
changes require a new base directory (e.g., `share/purse-first-v2/`).
Non-breaking additions (new optional files, new `plugin.json` fields) are
backwards-compatible and do not require a version bump.

## Appendix A: JSON Schemas

- Plugin: `plugin.schema.json`

Referenced by `$id` in the schema document.

## Appendix B: Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PURSE_FIRST_PLUGINS_DIR` | `<exe>/../share/purse-first/` | Override the package discovery root. |
| `XDG_STATE_HOME` | `$HOME/.local/state` | Base for user-level mapping overrides. |
| `XDG_CONFIG_HOME` | `$HOME/.config` | Base for user-level configuration. |
