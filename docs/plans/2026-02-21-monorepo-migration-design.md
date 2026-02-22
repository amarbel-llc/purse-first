# Monorepo Migration Design

## Problem

Circular dependencies between purse-first and its packages. purse-first
depends on all packages as flake inputs for marketplace generation, while
packages depend back on purse-first for go-mcp, rust-mcp, the purse
builder, and the CLI. Example: batman depends on purse-first CLI during
build, purse-first depends on batman as a flake input.

## Decision Summary

- **Scope:** All 6 packages move in (grit, get-hubbed, lux, chix, batman,
  tap-dancer)
- **Layout:** `packages/<name>/` directory
- **Go modules:** go.work workspace with per-package go.mod
- **Nix:** Single top-level flake.nix, per-package .nix build expressions
- **Git history:** Clean copy (history preserved in archived original repos)
- **Old repos:** Archive after migration
- **Approach:** Big bang — single branch, all packages at once

## Repository Layout

```
purse-first/
├── cmd/purse-first/          # CLI (unchanged)
├── internal/                 # Core Go packages (unchanged)
├── libs/
│   ├── go-mcp/              # Go MCP library (already here)
│   └── rust-mcp/            # Rust MCP library (already here)
├── purse/                    # Plugin manifest builder (unchanged)
├── packages/
│   ├── grit/                # Git MCP server (Go)
│   ├── get-hubbed/          # GitHub MCP server (Go)
│   ├── lux/                 # LSP multiplexer MCP server (Go)
│   ├── chix/                # Nix MCP+Skill (Rust)
│   ├── batman/              # BATS skill package (Shell/Nix)
│   └── tap-dancer/          # TAP-14 libraries + skill (Go/Rust/Bash)
├── skills/                  # purse-first's own skills (unchanged)
├── lib/                     # Nix library functions (unchanged)
│   └── packages/            # Per-package Nix build expressions (new)
├── templates/               # Nix templates (unchanged)
├── docs/                    # Docs (unchanged)
├── .claude-plugin/          # Bob plugin manifest (unchanged)
├── flake.nix               # Single flake builds everything
├── go.work                 # Go workspace linking all modules
├── justfile                # Updated with per-package targets
└── marketplace-config.json # Updated repo references
```

## Go Workspace

go.work at root links all Go modules:

```go
go 1.24

use (
    .
    ./libs/go-mcp
    ./packages/grit
    ./packages/get-hubbed
    ./packages/lux
    ./packages/tap-dancer/go
)
```

Each package keeps its own go.mod. The workspace resolves cross-references
locally during development. Nix builds ignore go.work (already excluded)
and build each package independently with gomod2nix.toml.

## Rust Dependencies

chix and tap-dancer/rust update their Cargo.toml to use path dependencies
for rust-mcp instead of git URLs:

```toml
mcp-server = { path = "../../libs/rust-mcp" }
```

## Nix Build Structure

### Removed flake inputs

grit, get-hubbed, lux, chix, batman, tap-dancer — no more external flake
references.

### New per-package build expressions

```
lib/packages/
├── grit.nix           # buildGoModule
├── get-hubbed.nix     # buildGoModule + makeWrapper (gh on PATH)
├── lux.nix            # buildGoModule
├── chix.nix           # buildRustPackage
├── batman.nix         # mkDerivation (shell/nix, copies libs + skills)
└── tap-dancer.nix     # Multi-output (Go + Rust + Bash)
```

The top-level flake.nix imports these and passes them to mkMarketplace:

```nix
plugins = system: [
  (import ./lib/packages/grit.nix { inherit pkgs; src = ./packages/grit; })
  (import ./lib/packages/lux.nix { inherit pkgs; src = ./packages/lux; })
  # ... etc
];
```

## Per-Package Migration Notes

### grit (Go MCP server)

- Copy cmd/, internal/, go.mod, go.sum, .claude-plugin/, BATS tests
- Add to go.work
- Create lib/packages/grit.nix (buildGoModule)

### get-hubbed (Go MCP server)

- Copy cmd/, internal/, go.mod, go.sum, .claude-plugin/
- Remove purse-first flake input (was using lib.goSrc)
- Wrap binary with gh on PATH via lib/packages/get-hubbed.nix

### lux (Go MCP server)

- Largest Go package (LSP routing, subprocess pool, etc.)
- Copy cmd/, internal/, pkg/, go.mod, go.sum, .claude-plugin/
- Add to go.work
- Create lib/packages/lux.nix (buildGoModule)

### chix (Rust MCP + Skill)

- Copy src/, Cargo.toml, Cargo.lock, skills/, .claude-plugin/ (with hooks)
- Update Cargo.toml: path dependency to ../../libs/rust-mcp
- Create lib/packages/chix.nix (buildRustPackage)

### batman (Shell/Nix Skill)

- Copy lib/, skills/, .claude-plugin/, BATS tests
- Remove purse-first flake input dependency (used CLI during build)
- Reference local CLI directly in lib/packages/batman.nix

### tap-dancer (Go + Rust + Bash)

- Copy go/, rust/, bash/, skills/, .claude-plugin/
- Add packages/tap-dancer/go to go.work
- Create lib/packages/tap-dancer.nix (multi-output derivation)

## marketplace-config.json

Update all `repo` fields to point to `amarbel-llc/purse-first`.

## Testing

- Each package keeps its own tests
- justfile gets per-package targets (build-grit, test-grit, etc.)
- test-all runs everything
- nix flake check validates the whole flake
- Existing BATS integration tests continue to work

## What Doesn't Change

- cmd/purse-first/ (CLI)
- internal/ (core Go packages)
- libs/go-mcp/ and libs/rust-mcp/ (already in monorepo)
- purse/ (manifest builder)
- skills/ (bob skills)
- lib/mkMarketplace.nix (minimal changes)
- .claude-plugin/ (bob manifest)
- Protocol specification and JSON schemas
