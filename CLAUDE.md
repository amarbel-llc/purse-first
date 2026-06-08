# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Package framework for producing per-package Claude Code plugins (CLIs, MCP servers, and skills) as composable, Nix-built derivations for humans and agents like Claude Code. Each package conforms to the `share/purse-first/<name>/` convention; external tools aggregate the per-package outputs. Three layers: Protocol (`share/purse-first/` convention), CLI (`purse-first` binary), Libraries (`go-mcp`, `dewey`).

Note: marketplace *aggregation* was removed in #110 — purse-first is now a per-package plugin *producer* toolkit plus libraries, not a marketplace assembler/installer.

Most concrete MCP-server packages (grit, get-hubbed, chix, etc.) have moved out of this repo — see [amarbel-llc/moxy](https://github.com/amarbel-llc/moxy) for those as moxins. (`lux` was also pulled out of this repo, but is currently **dormant** in its entirety — it is not published in moxy or any other active repo.) This repo now contains the framework itself plus a few co-located libraries and tools (purse-first CLI, go-mcp, dewey, dagnabit).

## Build & Test Commands

The justfile follows eng-design_patterns-justfile(7): bare-verb recipes
are aggregates (no body, only deps); leaves are verb-noun. `just`
(default) is the CI gate.

```sh
just                    # default = validate lint build test (CI gate; also runs in merge-this-session)
just validate           # nix flake check + plugin manifest validation
just lint               # go vet + lint-dewey_pkgs_drift (facade drift) + lint-conformist (read-only format+lint gate) + lint-dewey-self (dewey analyzers over dewey's own source)
just build              # build-nix-gomod2nix + nix build (default = purse-first CLI)
just test               # Run ALL tests (Go + BATS integration)
just codemod-fmt        # Repo-wide formatter: conformist pass (codemod-fmt-conformist) + Go-only go fmt (codemod-fmt-go). There is no `nix fmt` — conformist must not be a flake input (cycle; see flake.nix inputs comment), it resolves from the eng devshell's PATH
just codemod-fmt-go     # `go fmt ./...` only — for Go-only quick reformat
just build-nix-gomod2nix # Sync go.work and regenerate gomod2nix.toml
just update-go           # alias for build-nix-gomod2nix
just lint-dewey_pkgs_drift # dagnabit export --check: fail on libs/dewey/pkgs/ facade drift (no tree mutation)
```

### Running Individual Tests

```sh
# Per-library Go tests (via justfile):
just test-go            # all Go tests (./...)
just test-go-mcp        # libs/go-mcp/... (verbose)
just test-dewey         # libs/dewey/... (with -tags test)

# Single Go test function (bypass justfile):
nix develop --command go test -run TestFunctionName ./internal/...

# Single BATS file:
nix develop --command bats --tap zz-tests_bats/validate_documents.bats

# Integration tests (requires nix build first):
just test-integration   # validate_documents + validate_mcp
just test-validate      # validate_documents.bats only
just test-validate-mcp  # validate_mcp.bats only
just test-dagnabit-rust # dagnabit_rust.bats (cargo fixture workspaces; skips without cargo/ast-grep)
```

### Building Individual Packages

The flake exposes these `packages.<system>` outputs (see `nix flake show`):

```sh
nix build .#purse-first              # the purse-first CLI
nix build .#dagnabit                 # libs/dewey rename/export tool
nix build .#actx                     # dewey static analyzer
nix build .#defererr                 # dewey static analyzer
nix build .#paramobj                 # dewey static analyzer
nix build .#repool                   # dewey static analyzer
nix build .#seqerror                 # dewey static analyzer
nix build .#testui                   # dewey static analyzer
nix build .#reflexive-interface-generator  # dewey codegen tool
nix build .#golangci-lint-dewey      # golangci-lint with the dewey module plugin linked in
nix build .#manpages                 # go-mcp manpage tree
nix build .#go-pkgs                  # RFC 0001 published Go workspace source
nix build .#go-pkgs-test             # RFC 0001 test-only Go workspace source
```

The default `nix build` (no attribute) builds the `purse-first` CLI.

## Terminology

- **Package** (not "plugin") — the user-facing term. Three flavors:
  - **MCP package** — MCP server only
  - **Skill package** — Skill only
  - **MCP + Skill package** — Both
- **moxin** — a `purse-first`-protocol package consumed by a separate aggregator (`moxy`). The concrete MCP packages (grit, get-hubbed, chix, etc.) now live in [amarbel-llc/moxy](https://github.com/amarbel-llc/moxy) as moxins; this repo only defines the protocol and ships the framework, libraries, and a small set of in-tree skills. (`lux` is currently dormant — see the Overview.)

## Architecture

### Go Workspace

All Go packages share a single `go.work` workspace. Modules (see `go.work`): root (`.`), `libs/dewey`, `libs/go-mcp`, `libs/go-mcp/command/huh`.

In Nix, every Go binary is built via `mkGoModule` (defined in `gomod.nix`), a thin wrapper over `pkgs.buildGoApplication` from the gomod2nix overlay. `gomod.nix` also publishes RFC 0001 dual outputs `packages.${system}.go-pkgs` and `packages.${system}.go-pkgs-test` via `pkgs.mkGoPkgs`; self-consumption uses `go-pkgs-test` so each binary's `checkPhase` exercises the same artifact downstream consumers receive. The shared `gomod2nix.toml` lockfile at the workspace root pins external modules — local code changes never invalidate it. The lockfile is generated by [amarbel-llc/gomod2nix](https://github.com/amarbel-llc/gomod2nix) (a fork of `nix-community/gomod2nix` with `go.work` support). Run `just build-nix-gomod2nix` after any `go.mod`/`go.sum`/`go.work` change; CI fails on drift via `git diff --exit-code -- gomod2nix.toml`.

### Package Lifecycle (Three-Mode Main)

The `libs/go-mcp` `command.App` abstraction expects every consuming Go MCP package's `main.go` to dispatch on its first argument:

1. **`generate-plugin <dir>`** — build-time: `app.GenerateAll(dir)` writes `plugin.json`, `mappings.json`, and `hooks/` to the output directory
2. **`hook`** — Claude Code PreToolUse handler: `app.HandleHook(stdin, stdout)` reads hook input and denies built-in tools when an MCP tool should be used instead
3. **no args** — runtime: starts the MCP server via `server.New(...).Run(ctx)`

The downstream packages that live this lifecycle (grit, get-hubbed, chix, etc.) now ship out of [amarbel-llc/moxy](https://github.com/amarbel-llc/moxy); this repo defines and tests the framework that supports them.

### command.App Pattern (libs/go-mcp)

The `command` package is the primary abstraction for building MCP servers. Define commands once, get MCP tools + CLI subcommands + plugin manifests + hook generation:

```go
app := command.NewApp("name", "description")
app.AddCommand(&command.Command{
    Name:   "my-tool",
    Params: []command.Param{{Name: "path", Type: command.String, Required: true}},
    MapsTools: []command.ToolMapping{
        {Replaces: "Bash", CommandPrefixes: []string{"git status"}},
    },
    Run: handleMyTool, // func(ctx, json.RawMessage, Prompter) (*Result, error)
})
```

`MapsTools` declares which built-in Claude Code tools this command replaces — used by `GenerateAll` to produce hooks that deny the built-in tool in favor of the MCP tool. Hooks follow a fail-open model: errors return success, never deny.

For the lower-level server API, use `server.NewToolRegistryV1()` directly.

### Skill Documents

Skills live in `skills/<name>/SKILL.md` with YAML frontmatter:

```yaml
---
name: Human-readable name
description: Trigger description with specific keyword phrases
version: 0.1.0
---
```

Skills MAY have `references/` and `examples/` subdirectories. The SKILL.md body should be 1,500–2,000 words; put detailed content in `references/`. Discovery is automatic — any `skills/*/SKILL.md` is a skill.

### package.toml

Non-Go packages (and the repo itself) use `package.toml` at the package root instead of Go's `generate-plugin` pattern. `purse-first generate-plugin` reads it to produce `plugin.json`.

### purse-first CLI

The CLI is a slim per-package producer toolkit — it generates and validates
individual package manifests. Aggregation of per-package outputs is done by
external tools (e.g. clown), not by purse-first.

| Command | Purpose |
|---------|---------|
| `generate-plugin` | Generate `plugin.json` from `package.toml` |
| `validate [path]` | Validate plugin.json or mappings (auto-detects type; `--type mcp` for MCP probe) |
| `validate-mcp <binary>` | Probe a binary as an MCP server (initialize + tools/list + resources) |

### Protocol Key Rules

- `plugin.name` MUST equal the directory name under `share/purse-first/`
- Binaries resolve as `<package-root>/../../bin/<command>` (two levels up from share dir)
- Two packages MUST NOT produce the same name when an external tool composes them via `symlinkJoin`
- Reserved paths: `commands/`, `agents/`, `output-styles/`

## Repository Layout

| Directory | Purpose |
|-----------|---------|
| `cmd/purse-first/` | CLI entrypoint |
| `cmd/dagnabit/` | dewey-aware rename/export tool for Go packages and cargo workspace crates (rust mode); used by `dewey-*` justfile recipes |
| `cmd/go-mcp-docs/` | Generates the `go-mcp` manpage tree (built via `gomod.nix`) |
| `cmd/golangci-lint-dewey/` | Standalone Go module (own go.mod + gomod2nix.toml, NOT in go.work): golangci-lint with dewey's `gclplugin` linked in. Regen lockfiles with `just build-nix-gomod2nix-gcl` after changing its go.mod |
| `internal/` | Go internal packages (`validate`, `packagetoml`) |
| `libs/go-mcp/` | Go MCP server library (`command`, `server`, `transport`, `output`, `purse`, `jsonrpc`, `executor`, `operation`, `protocol`) |
| `libs/dewey/` | Multi-tier Go utility library (NATO-level dependency ordering; `internal/`, `pkgs/` stable facades) |
| `skills/` | In-tree skill documents (currently: claude-plugins, mcp) |
| `lib/` | Nix build expressions (`mkGoWorkspaceModule.nix`) |
| `gomod.nix` | Per-system Nix interface to the Go workspace: builds every Go binary plus the RFC 0001 `go-pkgs` / `go-pkgs-test` source derivations |
| `devenvs/` | Per-language dev shells composed into the default shell (`go`, `bats`, `shell`, `rust`) |
| `dev/lux-nvim/` | Auxiliary dev tooling (Neovim client for the now-dormant `lux`) |
| `zz-tests_bats/` | BATS integration tests |
| `package.toml` | This repo's own package manifest source (consumed by `generate-plugin`) |
| `docs/` | Protocol spec (`purse-first-protocol.md`), decisions/, features/, rfcs/, plans/, superpowers/ |

## Key Conventions

### Stable-First Nixpkgs

Every flake uses this pattern — do not deviate:

- `nixpkgs` → stable branch (runtimes, core tools)
- `nixpkgs-master` → master/unstable (LSPs, linters, formatters, latest features)
- `utils` → `flake-utils` from FlakeHub
- Variables: `pkgs = import nixpkgs`, `pkgs-master = import nixpkgs-master`

### Build Artifacts

Nix builds output to `result`/`result-*` symlinks (managed by nix, already gitignored). All other toolchain builds (go, etc.) must output to the `build/` directory. Never place binaries in the repo root or source directories.

### Code Style

- **Nix**: Format with `just codemod-fmt` (conformist → nixfmt)
- **Shell**: `set -euo pipefail`, 2-space indent, `[[ ]]` conditionals, quote all vars. Format with `shfmt -s -i=2`
- **Go**: `goimports` + `gofumpt`
- **Tests**: TAP-14 output format when reasonable, BATS for CLI integration tests

### macOS Path Resolution

On macOS, `/var` → `/private/var` and `/tmp` → `/private/tmp`.
`filepath.EvalSymlinks` fails on non-existent paths, returning the unresolved
form. Comparing that against a resolved path produces false mismatches.

When resolving paths that may not exist (e.g., for prefix-matching or
containment checks), walk up the directory tree to find an existing ancestor,
resolve symlinks there, then re-append the non-existent suffix.

Never use raw `filepath.EvalSymlinks` for path comparison when the target
might not exist. The reference implementation used to live in
`packages/spinclass/`; that package has moved out of this repo, but the
same convention applies anywhere this code is reintroduced.

### Git

- GPG signing is required for commits. If signing fails, ask user to unlock their agent rather than skipping signatures

## Protocol Specification

See `docs/purse-first-protocol.md` for the full protocol specification.

## Debugging Hooks

When Claude Code reports a hook error (e.g., "PreToolUse:Bash hook error"):

1. **Check `~/.claude/debug/<session-id>.txt` first** — it contains the full
   validation error, the expected schema, and the actual hook output. This is
   the fastest path to root cause.
2. Claude Code caches `hooks.json` at session startup. Edits to files in
   `~/.claude/plugins/cache/` have no effect on the running session's hook
   config. You must reinstall the plugin (via whatever aggregator installed it)
   and restart the session.
3. The pre-tool-use *script* is re-resolved at exec time (symlinks followed),
   so replacing the script file works for diagnostics captured by other sessions.
4. Hook output MUST include `hookEventName` inside `hookSpecificOutput` — see
   RFC-0001 section 2.2 for the required schema.

## External Integrations (verify before committing)

| Integration | How to verify |
|-------------|---------------|
| direnv (`prepareDirenv`) | Integration test suite or manual: create worktree, `direnv allow` |
| MCP JSON-RPC | `purse-first validate-mcp <binary>`, verify initialize + tools/list + resources |
| Plugin manifest | `purse-first validate <path>`, verify plugin.json / mappings conform |

## Versioning

Packages use semantic versioning (MAJOR.MINOR.PATCH):

- **MAJOR** --- breaking changes to MCP tool interfaces, skill behavior, or
  protocol (corresponds to RFC changes)
- **MINOR** --- new features, new tools, new skills (corresponds to FDR reaching
  `accepted` status)
- **PATCH** --- bug fixes, documentation, dependency updates

Per eng-versioning(7) MULTI-ARTIFACT RELEASE, a single version covers every
published artifact in this repo, sourced from one root `version.env`:

- `version.env` (repo root) — `PURSE_FIRST_VERSION` is the sole source of
  truth for the purse-first CLI, the `libs/dewey` library and its binaries,
  and the `libs/go-mcp` library. It is read by `gomod.nix` at build time
  (CLI version + dewey buildinfo ldflag) and by `just build-dewey`.

The `maintenance` group exposes one recipe triple:

- `just bump-version <sem>` — rewrites `PURSE_FIRST_VERSION` (pure mutation).
- `just tag <message>` — creates the full signed tag set at that version:
  `v<sem>` (primary), `libs/dewey/v<sem>`, and `libs/go-mcp/v<sem>`. The
  path-prefixed tags are required by the Go module proxy to resolve the
  sub-directory modules (eng-versioning(7) → TAG NAMING).
- `just release <sem>` — master-only: repo-wide changelog → bump → commit →
  tag set → `gh release create` against the primary `v<sem>` tag, with notes
  enumerating the sibling sub-module tags.

Pre-1.0: MINOR bumps may include breaking changes. Post-1.0: semver is strict.
