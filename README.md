# purse-first

A package framework for bundling CLIs, MCP servers, and skills into composable, Nix-built packages for humans and agents like Claude Code.

## Architecture

purse-first has three layers:

| Layer | Description |
|-------|-------------|
| **Protocol** | A convention for Nix derivations to declare Claude Code capabilities via well-known paths under `share/purse-first/` |
| **CLI** | The `purse-first` binary — implements tool routing (hooks), marketplace generation, installation, and validation |
| **Libraries** | `go-mcp` — building blocks for creating MCP servers that conform to the protocol; `dewey` — multi-tier Go utilities, static analyzers, and the `dagnabit` export tool |

## Package Flavors

The protocol defines three package flavors. Concrete MCP packages now live in
[amarbel-llc/moxy](https://github.com/amarbel-llc/moxy) as moxins; the in-tree
skill packages are `claude-plugins` and `mcp`.

| Flavor | Description | Examples |
|--------|-------------|----------|
| **MCP package** | Ships an MCP server with optional tool mappings | grit, get-hubbed (moxy) |
| **Skill package** | Ships skills only | claude-plugins, mcp (in-tree) |
| **MCP + Skill package** | Ships both MCP server and skills | chix (moxy) |

## What's in this repo

purse-first was slimmed to the framework itself. The concrete MCP-server
packages (grit, get-hubbed, chix, …) moved to
[amarbel-llc/moxy](https://github.com/amarbel-llc/moxy) as moxins; `lux` was
pulled out and is currently **dormant** (not published in moxy or any other
active repo). What remains here:

- **`purse-first`** (CLI) — tool routing (hooks), marketplace generation, installation, and validation
- **`go-mcp`** (library) — building blocks for protocol-conforming MCP servers
- **`dewey`** (library) — multi-tier Go utilities, static analyzers, and the `dagnabit` rename/export tool
- **In-tree skills** — `claude-plugins` and `mcp`

## Getting Started

purse-first packages are built with Nix and installed via the `purse-first` CLI. A **marketplace** is an aggregation of packages into a single Claude Code plugin (the `.claude-plugin/` output).

### Installation

```sh
# Install the purse-first CLI
nix profile install github:amarbel-llc/purse-first#purse-first

# Generate a marketplace.json from discovered packages
purse-first generate-marketplace --config marketplace-config.json --output ~/.claude/plugins/marketplace

# …or install this repo's own marketplace directly
purse-first install-self
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
