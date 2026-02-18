# purse-first

A package framework for bundling CLIs, MCP servers, and skills into composable, Nix-built packages for humans and agents like Claude Code.

## Architecture

purse-first has three layers:

| Layer | Description |
|-------|-------------|
| **Protocol** | A convention for Nix derivations to declare Claude Code capabilities via well-known paths under `share/purse-first/` |
| **CLI** | The `purse-first` binary — implements tool routing (hooks), marketplace generation, installation, and validation |
| **Libraries** | `go-mcp` and `rust-mcp` — building blocks for creating MCP servers that conform to the protocol |

## Package Flavors

Packages come in three flavors:

| Flavor | Description | Examples |
|--------|-------------|----------|
| **MCP package** | Ships an MCP server with optional tool mappings | grit, get-hubbed, lux |
| **Skill package** | Ships skills only | robin, tap-dancer, bob |
| **MCP + Skill package** | Ships both MCP server and skills | chix |

## Current Packages

- **grit** (MCP) — Git operations via MCP: stage, commit, push, log, blame, and more
- **get-hubbed** (MCP) — GitHub CLI wrapper via MCP: issues, pull requests, and repos
- **lux** (MCP) — LSP multiplexer via MCP: hover, definitions, references, and symbols
- **chix** (MCP + Skill) — Nix MCP server and skills for Claude Code: build, evaluate, search, and manage flakes
- **robin** (Skill) — BATS integration testing skill with bundled assertion libraries, sandcastle isolation, and justfile patterns
- **tap-dancer** (Skill) — TAP-14 writer libraries (Go + Rust) and skill for producing spec-compliant TAP output
- **bob** (Skill) — purse-first's own skill package: plugin-mcp, context-saving, and go-cli-framework skills

## Getting Started

purse-first packages are built with Nix and installed via the `purse-first` CLI. A **marketplace** is an aggregation of packages into a single Claude Code plugin (the `.claude-plugin/` output).

### Installation

```sh
# Install purse-first CLI
nix profile install github:friedenberg/purse-first

# Generate a marketplace from packages
purse-first marketplace generate --config marketplace-config.json --output ~/.claude/plugins/marketplace
```

For the complete protocol specification, see [docs/purse-first-protocol.md](docs/purse-first-protocol.md).

## Development

This is a Nix-based project. See the project-specific CLAUDE.md for build commands and architecture details.

```sh
# Build the project
just build

# Run tests
just test

# Format code
just fmt
```
