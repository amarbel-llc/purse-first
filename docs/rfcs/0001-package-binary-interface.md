---
status: exploring
date: 2026-03-06
---

# Package Binary Interface

## Abstract

This specification defines the CLI subcommands that a purse-first package
binary MUST implement to support build-time artifact generation, runtime hook
handling, and local development workflows. The interface is language-agnostic:
Go binaries (via `command.App`) and Rust binaries (via `rust-mcp`) produce
identical behavior from the perspective of `purse-first` and the Nix build
system. Standardizing this contract enables `purse-first install-dev-mcp` to
work uniformly across all MCP packages.

## Introduction

Today, purse-first MCP packages expose build-time and runtime subcommands
inconsistently:

| Package | Generate | Hook | Notes |
|---------|----------|------|-------|
| grit | `grit generate-plugin <dir>` | `grit hook` | Positional args, raw `flag.Parse` |
| get-hubbed | `get-hubbed generate-plugin <dir>` | `get-hubbed hook` | Same pattern as grit |
| mgp | `mgp generate-plugin <dir>` | `mgp hook` | Same pattern as grit |
| lux | `lux _generate --dir <dir>` | `lux hook` | Named param via `command.App` RunCLI |
| chix | None (uses `purse-first generate-plugin`) | `chix hook` | Rust; no generate-plugin subcommand |

This inconsistency creates three problems:

1. **No uniform local dev flow.** `purse-first install-dev-mcp` cannot call a
   consistent subcommand across packages to discover MCP server metadata.
2. **Nix build files encode package-specific knowledge.** Each `.nix` file
   knows whether to call `generate-plugin <dir>` or `_generate --dir <dir>` or
   `purse-first generate-plugin --root <src>`.
3. **Rust packages lack parity.** chix has no `generate-plugin` subcommand,
   requiring the Nix build to orchestrate multiple binaries.

This RFC specifies the subcommand interface so that all package binaries
— regardless of implementation language — expose the same contract.

## Requirements Language

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD",
"SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be
interpreted as described in RFC 2119.

## Specification

### 1. Scope

This specification applies to any binary that serves as an MCP server within a
purse-first package. Skill-only packages (no MCP server) are out of scope.

### 2. Subcommands

A conformant binary MUST implement the following subcommands:

#### 2.1. `generate-plugin [<output-dir>|-]`

Writes package metadata following the purse-first protocol directory layout.

**Invocation:**

```
<binary> generate-plugin              # writes to current working directory
<binary> generate-plugin <output-dir> # writes to <output-dir>
<binary> generate-plugin -            # writes plugin.json to stdout
```

**Arguments:**

| Position | Required | Description |
|----------|----------|-------------|
| 1 | No | Output target. A directory path, `-` for stdout, or omitted for PWD. |

**Output modes:**

The argument MUST be interpreted as follows:

- **Omitted:** The binary MUST use the current working directory as the output
  directory, writing to `./share/purse-first/<name>/`.
- **A directory path:** The binary MUST write to
  `<output-dir>/share/purse-first/<name>/`.
- **`-` (hyphen):** The binary MUST write only `plugin.json` content to stdout
  as a single JSON document. No files are written to disk. Additional artifacts
  (mappings, hooks, skills) are NOT produced in this mode.

**Directory mode behavior:**

The binary MUST write at minimum:

```
<output-dir>/share/purse-first/<name>/plugin.json
```

The binary SHOULD write, when applicable:

```
<output-dir>/share/purse-first/<name>/mappings.json
<output-dir>/share/purse-first/<name>/hooks/hooks.json
<output-dir>/share/purse-first/<name>/hooks/pre-tool-use
```

The binary MAY write additional artifacts (manpages, shell completions, skills)
to standard paths under `<output-dir>`.

**Stdout mode behavior:**

When the argument is `-`, the binary MUST write a single JSON object to stdout
that conforms to the `plugin.json` schema. The binary MUST NOT write any files
to disk. Diagnostic output MUST be written to stderr, not stdout.

This mode exists for tooling that needs to inspect package metadata without
side effects (e.g., `purse-first install-dev-mcp`).

**plugin.json:** In both modes, the `plugin.json` content MUST conform to the
plugin manifest schema defined in the purse-first protocol. The `name` field
MUST match the directory name under `share/purse-first/` (in directory mode) or
the package's canonical name (in stdout mode). The `mcpServers` map MUST
contain at least one entry. The `command` field in each MCP server entry MUST
be a bare binary name (not an absolute path).

**Exit code:** 0 on success, non-zero on failure. Diagnostic output SHOULD be
written to stderr.

**Idempotency:** In directory mode, the command MUST be safe to run multiple
times with the same output directory. Existing files MUST be overwritten, not
appended to.

#### 2.2. `hook`

Handles PreToolUse hook input from Claude Code.

**Invocation:**

```
<binary> hook
```

**Behavior:**

The binary MUST read JSON hook input from stdin and write a JSON hook response
to stdout. If the binary determines that the invoked built-in tool should be
denied in favor of an MCP tool, it MUST write a deny response. Otherwise, it
MUST write nothing to stdout (implicit allow) or an explicit allow response.

**Fail-open:** On any error (I/O failure, parse error, unexpected input), the
binary MUST exit 0 and MUST NOT write a deny response. Hook errors MUST NOT
block the agent.

**Exit code:** Always 0. Errors are logged to stderr but never surfaced as
non-zero exit codes.

#### 2.3. Default (no subcommand)

When invoked with no arguments (or no recognized subcommand), the binary MUST
start the MCP server.

**Invocation:**

```
<binary>
```

Additional transport flags (e.g., `--sse`, `--port`) are outside the scope of
this specification and MAY vary by implementation.

### 3. Subcommand Detection

The binary MUST detect subcommands by examining its first positional argument:

- If the first argument is `generate-plugin`, dispatch to section 2.1.
- If the first argument is `hook`, dispatch to section 2.2.
- If no arguments are provided, dispatch to section 2.3 (start MCP server).

Note that `generate-plugin` with no further arguments writes to the current
working directory (section 2.1), which is distinct from no arguments at all
(section 2.3).

Binaries MAY support additional subcommands beyond those specified here. Unknown
subcommands SHOULD produce a non-zero exit code and a diagnostic message on
stderr.

Binaries MUST NOT require flags before the subcommand name. The subcommand MUST
be the first positional argument.

### 4. Output Directory Layout

The `generate-plugin` subcommand writes to a directory structure rooted at
`<output-dir>`. The binary MUST create intermediate directories as needed.

```
<output-dir>/
  share/
    purse-first/
      <name>/
        plugin.json           # REQUIRED
        mappings.json         # CONDITIONAL: when tool mappings exist
        hooks/
          hooks.json          # CONDITIONAL: when tool mappings exist
          pre-tool-use        # CONDITIONAL: when tool mappings exist
        skills/               # OPTIONAL: when skills are bundled
          <skill-name>/
            SKILL.md
```

The `hooks/pre-tool-use` script, when generated, MUST be executable (mode
0755). It MUST invoke the package binary's `hook` subcommand, passing stdin
through and writing stdout through.

### 5. Skills Discovery

When a `--skills-dir <path>` flag is provided to `generate-plugin`, the binary
SHOULD discover skills by globbing `<path>/*/SKILL.md`, copy matching
directories into the output under `share/purse-first/<name>/skills/`, and
include them in `plugin.json` as relative paths (e.g., `"./skills/<name>"`).

The `--skills-dir` flag is OPTIONAL. Binaries that do not support skills MAY
ignore it.

**Invocation with skills:**

```
<binary> generate-plugin <output-dir> --skills-dir <path>
```

### 6. Nix Build Integration

In a Nix derivation's `postInstall` (or equivalent build phase), the binary
MUST be invokable with an explicit output directory:

```nix
postInstall = ''
  $out/bin/<binary> generate-plugin $out
'';
```

Or with skills:

```nix
postInstall = ''
  $out/bin/<binary> generate-plugin $out --skills-dir ${src}/skills
'';
```

Note: Nix builds SHOULD always pass an explicit `$out` rather than relying on
the PWD default, since the working directory during Nix builds is not the
output path.

This replaces all current per-package variations (`_generate --dir`,
`purse-first generate-plugin --root`, etc.).

### 7. Local Development Integration

`purse-first install-dev-mcp` relies on this interface to generate `.mcp.json`
for a locally-built package. The workflow is:

1. Developer builds the package (e.g., `nix build .#grit` or `cargo build`).
2. Developer runs `purse-first install-dev-mcp <path-to-binary>`.
3. `purse-first` invokes `<binary> generate-plugin -` to read the package
   metadata from stdout — no temporary directories, no filesystem side effects.
4. `purse-first` parses the JSON to extract the package name and MCP server
   configuration.
5. `purse-first` writes `.mcp.json` to the current directory with the binary
   path resolved to the absolute path of `<path-to-binary>`.

The stdout mode (`-`) makes this lightweight: no intermediate directories to
create or clean up, and the binary's working directory is irrelevant. The
`command` field in the output is a bare name that `purse-first` replaces with
the absolute binary path.

## Security Considerations

**Arbitrary binary execution.** `purse-first install-dev-mcp` executes a
user-provided binary with the `generate-plugin` subcommand. This is equivalent
to the trust model of `nix build` — the user trusts the binary they built.
`purse-first` SHOULD NOT execute binaries from untrusted sources.

**Hook fail-open.** The fail-open requirement in section 2.2 is a deliberate
security trade-off: a broken hook degrades to allowing all tools rather than
blocking the agent. This prevents a malicious or buggy hook from denying
legitimate tool use.

**No network access.** The `generate-plugin` subcommand SHOULD NOT make network
requests. All metadata MUST be derivable from the binary's compiled-in state.

## Conformance Testing

Conformance tests for this specification live in `zz-tests_bats/rfc-0001/`.

Tests use binary injection via `bats-emo`:

    require_bin PACKAGE_BIN

### Covered Requirements

| Requirement | Test File | Description |
|-------------|-----------|-------------|
| 2.1, directory mode MUST write plugin.json | `rfc-0001/generate_plugin_interface.bats` | Runs `generate-plugin <tmpdir>` and checks plugin.json exists |
| 2.1, directory mode MUST match name to dirname | `rfc-0001/generate_plugin_interface.bats` | Validates plugin.name equals directory name |
| 2.1, MUST have mcpServers | `rfc-0001/generate_plugin_interface.bats` | Validates at least one MCP server entry |
| 2.1, MUST use bare command name | `rfc-0001/generate_plugin_interface.bats` | Validates command field contains no `/` |
| 2.1, directory mode MUST be idempotent | `rfc-0001/generate_plugin_interface.bats` | Runs generate-plugin twice, checks identical output |
| 2.1, PWD default MUST write to cwd | `rfc-0001/generate_plugin_interface.bats` | Runs `generate-plugin` with no args from a tmpdir, checks output there |
| 2.1, stdout mode MUST output valid JSON | `rfc-0001/generate_plugin_interface.bats` | Runs `generate-plugin -`, parses stdout as JSON |
| 2.1, stdout mode MUST NOT write files | `rfc-0001/generate_plugin_interface.bats` | Runs `generate-plugin -` from an empty tmpdir, checks no files created |
| 2.2, MUST exit 0 on error | `rfc-0001/hook_interface.bats` | Sends malformed stdin, checks exit code 0 |
| 2.2, MUST NOT deny on error | `rfc-0001/hook_interface.bats` | Sends malformed stdin, checks empty or allow stdout |
| 3, MUST detect subcommand as first arg | `rfc-0001/generate_plugin_interface.bats` | Verifies `<binary> generate-plugin <dir>` works |

## Compatibility

### Migration from Current Interfaces

**Go packages (grit, get-hubbed, mgp):** Already conformant. Their
`generate-plugin <dir>` positional dispatch matches section 2.1 exactly.

**Go packages via command.App RunCLI (lux):** The `_generate --dir <dir>` command
MUST be replaced with `generate-plugin <dir>`. The `command.App` framework
SHOULD register `generate-plugin` as a hidden command that delegates to
`GenerateAll`.

**Rust packages (chix):** MUST add a `generate-plugin <dir>` subcommand. This
replaces the current split where `purse-first generate-plugin` reads
`package.toml` and `chix generate-hooks` writes hooks separately. The chix
binary SHOULD handle both plugin manifest generation and hook generation in a
single `generate-plugin` invocation.

### package.toml

The `package.toml` format remains valid for packages that do not have a binary
(skill-only packages). For MCP packages, `package.toml` becomes OPTIONAL once
the binary implements `generate-plugin`. The `purse-first generate-plugin`
command continues to exist for skill-only packages.

### Versioning

This is the first version of the package binary interface specification. Future
changes to required subcommands or argument formats constitute breaking changes
and MUST be specified in a new RFC.

## References

### Normative

- [Purse-First Package Protocol](../purse-first-protocol.md) — Directory
  layout and file format specification
- [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119) — Requirement keywords

### Informative

- [ADR-0008: Unified Plugin Generation](../decisions/0008-unified-plugin-generation.md)
- [ADR-0011: Per-Package Hooks](../decisions/0011-per-package-hooks.md)
- [ADR-0012: Unify Tool Interception with MapsTools](../decisions/0012-unify-tool-interception-with-mapstools.md)
