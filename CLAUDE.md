# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Package framework for bundling CLIs, MCP servers, and skills into composable, Nix-built packages for humans and agents like Claude Code. Three layers: Protocol (share/purse-first/ convention), CLI (purse-first binary), Libraries (go-mcp, rust-mcp).

## Build & Test Commands

```sh
just build              # Build the project (alias: build-all)
just test               # Run all tests (Go + BATS)
just fmt                # Format code (Go, shell, Nix)
nix flake check         # Nix-level validation
just build-nix          # Nix build only
just update-plugins     # Update flake inputs for packages
```

## Repository Layout

| Directory | Purpose |
|-----------|---------|
| `cmd/purse-first/` | CLI entrypoint |
| `internal/` | Go internal packages (hook, install, marketplace, mapping, config, validate, localplugin, mcp) |
| `libs/go-mcp/` | Go MCP server library |
| `libs/rust-mcp/` | Rust MCP server library |
| `purse/` | Go package for building package manifests (plugin.json) |
| `skills/` | Skill documents (plugin-mcp, context-saving, go-cli-framework) |
| `packages/grit/` | Git MCP server (Go) |
| `packages/get-hubbed/` | GitHub MCP server (Go) |
| `packages/lux/` | LSP multiplexer MCP server (Go) |
| `packages/chix/` | Nix MCP+Skill server (Rust) |
| `packages/batman/` | BATS testing skill + libraries (Shell/Nix) |
| `packages/tap-dancer/` | TAP-14 libraries + skill (Go/Rust/Bash) |
| `lib/packages/` | Per-package Nix build expressions |
| `docs/` | Protocol spec and design docs |
| `zz-tests_bats/` | BATS integration tests |
| `.claude-plugin/` | Claude Code plugin manifest for bob |
| `marketplace-config.json` | Metadata for marketplace generation |
| `templates/` | Nix templates |
| `lib/` | Nix library functions (mkMarketplace) |

## Terminology

- **Package** (not "plugin") — the user-facing term for what purse-first distributes. Three flavors:
  - **MCP package** — MCP server only (grit, get-hubbed, lux)
  - **Skill package** — Skill only (robin, tap-dancer, bob)
  - **MCP + Skill package** — Both (chix)
- **Marketplace** — aggregated JSON output listing all available packages
- **bob** — purse-first's own skill package for working with purse-first codebases

## Monorepo Structure

All packages are co-located in this repo under `packages/`. Go modules use a `go.work` workspace for local resolution. Rust packages use path dependencies to `libs/rust-mcp`. The top-level `flake.nix` builds everything from local sources via per-package expressions in `lib/packages/`.

## Key Conventions

### Stable-First Nixpkgs

Every flake uses this pattern — do not deviate:

- `nixpkgs` → stable branch (runtimes, core tools)
- `nixpkgs-master` → master/unstable (LSPs, linters, formatters, latest features)
- `utils` → `flake-utils` from FlakeHub
- Variables: `pkgs = import nixpkgs`, `pkgs-master = import nixpkgs-master`

### Code Style & Tooling

- **Nix**: Format with `nix fmt` (nixfmt-rfc-style)
- **Shell**: `set -euo pipefail`, 2-space indent, `[[ ]]` conditionals, quote all vars. Format with `shfmt -s -i=2`
- **Go**: `goimports` + `gofumpt`
- **Tests**: TAP-14 output format when reasonable, BATS for CLI integration tests

### Git

- GPG signing is required for commits. If signing fails, ask user to unlock their agent rather than skipping signatures

## Protocol Specification

See `docs/purse-first-protocol.md` for the full protocol specification.
