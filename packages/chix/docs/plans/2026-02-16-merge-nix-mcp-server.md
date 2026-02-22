# Merge nix-mcp-server into chix Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Consolidate nix-mcp-server (Rust MCP server) into the chix plugin, creating a single "Nix for Claude Code" plugin that ships both skills and the MCP server binary (renamed to `chix`).

**Architecture:** Copy all Rust source from `~/eng/repos/nix-mcp-server/src/` into `chix/src/`, bring `Cargo.toml`/`Cargo.lock`, rework `flake.nix` to build the Rust binary with crane, update `plugin.json` to declare both skills and MCP server, and rename all references from `nix-mcp-server` to `chix`.

**Tech Stack:** Rust (2021 edition), Nix (crane builder, rust-overlay, makeWrapper), MCP protocol (JSON-RPC 2.0)

---

### Task 1: Copy Rust source and Cargo manifests

**Files:**
- Copy: all 26 files from `~/eng/repos/nix-mcp-server/src/` to `src/`
- Copy: `~/eng/repos/nix-mcp-server/Cargo.toml` to `Cargo.toml`
- Copy: `~/eng/repos/nix-mcp-server/Cargo.lock` to `Cargo.lock`

**Step 1: Copy source tree and manifests**

```bash
cp -r ~/eng/repos/nix-mcp-server/src/ /home/sasha/eng/repos/chix/src/
cp ~/eng/repos/nix-mcp-server/Cargo.toml /home/sasha/eng/repos/chix/Cargo.toml
cp ~/eng/repos/nix-mcp-server/Cargo.lock /home/sasha/eng/repos/chix/Cargo.lock
```

**Step 2: Verify the copy**

Run: `find src/ -name '*.rs' | wc -l`
Expected: 26

---

### Task 2: Rename binary from nix-mcp-server to chix

**Files:**
- Modify: `Cargo.toml` — change `name` from `"nix-mcp-server"` to `"chix"`
- Modify: `src/main.rs` — change CLI name and about text, update install-claude to register as `"nix"` MCP server using `chix` binary
- Modify: `src/server.rs:212` — change `server_info.name` from `"nix-mcp-server"` to `"chix"`

**Step 1: Update Cargo.toml**

In `Cargo.toml`, change:
```toml
[package]
name = "nix-mcp-server"
```
to:
```toml
[package]
name = "chix"
```

**Step 2: Update main.rs CLI metadata**

In `src/main.rs`, change:
```rust
#[command(name = "nix-mcp-server")]
#[command(about = "MCP server providing nix operations as tools for Claude Code")]
```
to:
```rust
#[command(name = "chix")]
#[command(about = "Nix MCP server and skills for Claude Code")]
```

Also update the `install_claude` success message from `"nix-mcp-server"` to `"chix"`.

**Step 3: Update server.rs server info**

In `src/server.rs`, change:
```rust
name: "nix-mcp-server".to_string(),
```
to:
```rust
name: "chix".to_string(),
```

**Step 4: Verify cargo check passes**

Run: `nix develop --command cargo check`
Expected: compiles with no errors (warnings OK)

**Step 5: Commit**

```bash
git add src/ Cargo.toml Cargo.lock
git commit -m "feat: import nix-mcp-server source, rename binary to chix"
```

---

### Task 3: Rework flake.nix for Rust build

**Files:**
- Modify: `flake.nix` — replace devShell-only flake with full crane-based Rust build

**Step 1: Replace flake.nix**

The new `flake.nix` must:
- Keep existing inputs: `nixpkgs`, `nixpkgs-master`, `utils`
- Remove: `shell` input (replaced by `rust` devenv)
- Add inputs: `rust-overlay`, `crane`, `fh`, `rust` devenv (from `github:friedenberg/eng?dir=devenvs/rust`)
- Build with crane: separate `buildDepsOnly` + `buildPackage` for the `chix` binary
- Wrap with `makeWrapper` to inject `fh`, `cachix`, `nil` into PATH
- Install `plugin.json` and `format-nix` hook to `$out/share/purse-first/nix/`
- DevShell: extend `rust` devenv with `fh`, `nil`, `just`, `gum`

New `flake.nix`:

```nix
{
  description = "Nix MCP server and skills for Claude Code";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/23d72dabcb3b12469f57b37170fcbc1789bd7457";
    nixpkgs-master.url = "github:NixOS/nixpkgs/b28c4999ed71543e71552ccfd0d7e68c581ba7e9";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
    rust-overlay = {
      url = "github:oxalica/rust-overlay";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    crane.url = "github:ipetkov/crane";
    fh.url = "https://flakehub.com/f/DeterminateSystems/fh/*.tar.gz";
    rust.url = "github:friedenberg/eng?dir=devenvs/rust";
  };

  outputs =
    {
      self,
      nixpkgs,
      nixpkgs-master,
      utils,
      rust-overlay,
      crane,
      fh,
      rust,
    }:
    utils.lib.eachDefaultSystem (
      system:
      let
        overlays = [ (import rust-overlay) ];
        pkgs = import nixpkgs { inherit system overlays; };

        rustToolchain = pkgs.rust-bin.stable.latest.default;
        craneLib = (crane.mkLib pkgs).overrideToolchain rustToolchain;

        src = craneLib.cleanCargoSource ./.;

        commonArgs = {
          inherit src;
          strictDeps = true;
          buildInputs = [ ];
          nativeBuildInputs = [ ];
        };

        cargoArtifacts = craneLib.buildDepsOnly commonArgs;

        chix-unwrapped = craneLib.buildPackage (
          commonArgs
          // {
            inherit cargoArtifacts;
          }
        );

        fhPkg = fh.packages.${system}.default;

        formatNixHook = pkgs.writeShellScript "format-nix" ''
          set -euo pipefail
          input=$(cat)
          file_path=$(${pkgs.jq}/bin/jq -r '.tool_input.file_path // empty' <<< "$input")
          if [[ -n "$file_path" && "$file_path" == *.nix ]]; then
            ${pkgs.nixfmt-rfc-style}/bin/nixfmt "$file_path" 2>/dev/null || true
          fi
        '';

        chix =
          pkgs.runCommand "chix"
            {
              nativeBuildInputs = [ pkgs.makeWrapper ];
            }
            ''
              mkdir -p $out/bin
              makeWrapper ${chix-unwrapped}/bin/chix $out/bin/chix \
                --prefix PATH : ${
                  pkgs.lib.makeBinPath [
                    fhPkg
                    pkgs.cachix
                    pkgs.nil
                  ]
                }

              mkdir -p $out/share/purse-first/nix/hooks
              cp ${./.claude-plugin/plugin.json} $out/share/purse-first/nix/plugin.json
              install -m 755 ${formatNixHook} $out/share/purse-first/nix/hooks/format-nix
            '';
      in
      {
        packages = {
          default = chix;
          chix = chix;
          unwrapped = chix-unwrapped;
        };

        devShells.default = rust.devShells.${system}.default.overrideAttrs (oldAttrs: {
          nativeBuildInputs = (oldAttrs.nativeBuildInputs or [ ]) ++ [
            fhPkg
            pkgs.nil
            pkgs.just
            pkgs.gum
          ];

          shellHook = ''
            echo "chix - dev environment"
          '';
        });
      }
    );
}
```

**Step 2: Update flake.lock**

Run: `nix flake lock`
Expected: resolves all inputs, produces updated `flake.lock`

**Step 3: Verify the build**

Run: `nix build .#default --show-trace`
Expected: builds successfully, produces `result/bin/chix`

**Step 4: Verify wrapped binary has tools in PATH**

Run: `./result/bin/chix --help`
Expected: shows CLI help with name `chix`

**Step 5: Commit**

```bash
git add flake.nix flake.lock
git commit -m "feat: add crane-based Rust build for chix binary"
```

---

### Task 4: Update plugin.json to declare MCP server

**Files:**
- Modify: `.claude-plugin/plugin.json` — add `mcpServers` and `hooks` declarations

**Step 1: Update plugin.json**

Replace `.claude-plugin/plugin.json` with:

```json
{
  "name": "chix",
  "description": "Nix MCP server and skills for Claude Code: 32 tools for nix, FlakeHub, cachix, nil LSP operations plus procedural knowledge for Go/Rust Nix projects",
  "author": {
    "name": "friedenberg"
  },
  "mcpServers": {
    "nix": {
      "type": "stdio",
      "command": "chix"
    }
  },
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "${CLAUDE_PLUGIN_ROOT}/hooks/format-nix",
            "timeout": 30
          }
        ]
      }
    ]
  }
}
```

Note: the `hooks/format-nix` script is installed to `$out/share/purse-first/nix/hooks/` by the flake, but when used as a development plugin (via `.claude-plugin/`), it needs a local copy. We'll handle this in the next task.

**Step 2: Commit**

```bash
git add .claude-plugin/plugin.json
git commit -m "feat: declare MCP server and format hook in plugin manifest"
```

---

### Task 5: Add format-nix hook script for development use

**Files:**
- Create: `.claude-plugin/hooks/format-nix`

**Step 1: Create the hook script**

This is used when the plugin is loaded from the source directory (development mode). The Nix-built version installs its own copy.

```bash
#!/usr/bin/env bash
set -euo pipefail
input=$(cat)
file_path=$(jq -r '.tool_input.file_path // empty' <<< "$input")
if [[ -n "$file_path" && "$file_path" == *.nix ]]; then
  nixfmt "$file_path" 2>/dev/null || true
fi
```

Run: `chmod +x .claude-plugin/hooks/format-nix`

**Step 2: Commit**

```bash
git add .claude-plugin/hooks/format-nix
git commit -m "feat: add format-nix hook for development mode"
```

---

### Task 6: Update justfile for Rust development

**Files:**
- Modify: `justfile` — replace with targets for both Rust and skill development

**Step 1: Write new justfile**

```just
# chix

default:
    @just --list

# Build with nix
build:
    nix build .#default --show-trace

# Development build with cargo
dev:
    cargo build

# Watch for changes and rebuild
watch:
    cargo watch -x build

# Run tests
test:
    cargo test

# Format all code
fmt:
    cargo fmt
    nix develop --command shfmt -w -i 2 -ci .claude-plugin/hooks/*.bash 2>/dev/null || true

# Check code
check:
    cargo check
    cargo clippy

# Clean build artifacts
clean:
    cargo clean
    rm -rf result
```

**Step 2: Commit**

```bash
git add justfile
git commit -m "feat: update justfile with Rust build targets"
```

---

### Task 7: Update .gitignore for Rust artifacts

**Files:**
- Modify: `.gitignore`

**Step 1: Append Rust entries**

Add to `.gitignore`:

```
# Rust
debug
target
**/*.rs.bk
*.pdb
**/mutants.out*/
```

**Step 2: Commit**

```bash
git add .gitignore
git commit -m "chore: add Rust build artifacts to gitignore"
```

---

### Task 8: Update CLAUDE.md and README

**Files:**
- Create: `CLAUDE.md` — merge development guidance from nix-mcp-server

**Step 1: Write CLAUDE.md**

```markdown
# CLAUDE.md

This file provides guidance to Claude Code when working in this repository.

## Build Commands

```bash
just build          # Build with nix (preferred)
just dev            # Fast cargo build
just test           # Run tests
just check          # Run cargo check + clippy
just fmt            # Format code
```

## Architecture

chix is a Claude Code plugin combining:
1. **MCP server** — 32 tools for nix, FlakeHub, cachix, and nil LSP operations (Rust, JSON-RPC 2.0 over stdin/stdout)
2. **Skills** — procedural knowledge for Nix-backed Go/Rust projects

### MCP Server (src/)

Request flow:
1. `main.rs` — Entry point, reads JSON-RPC requests line-by-line from stdin
2. `server.rs` — Dispatches to handlers (`initialize`, `tools/list`, `tools/call`, `resources/list`, `resources/read`)
3. `tools/mod.rs` — Tool registry with `list_tools()` returning all tool definitions
4. `tools/*.rs` — Individual tool implementations
5. `nix_runner.rs` — Executes `nix`, `fh`, `cachix` CLI commands with timeout (300s)
6. `validators.rs` — Input validation: flake refs, installables, shell metacharacter blocking

### Adding a New Tool

1. Add implementation in `src/tools/` (new file or existing)
2. Add `ToolInfo` entry in `tools/mod.rs::list_tools()` with name, description, and JSON schema
3. Add parameter struct in `tools/mod.rs`
4. Add match arm in `server.rs::call_tool()`
5. Export from `tools/mod.rs`

### Security

All inputs are validated before execution:
- `validate_installable()` / `validate_flake_ref()` — Whitelist regex for flake references
- `validate_no_shell_metacharacters()` — Blocks shell injection characters
- `validate_args()` — Validates argument arrays

Commands use `kill_on_drop(true)` to ensure cleanup on timeout.

### Skills (skills/)

Procedural knowledge for Nix workflows. See `skills/nix-codebase/SKILL.md`.
```

**Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add CLAUDE.md with development guidance"
```

---

### Task 9: Run full build and verify

**Files:** None (verification only)

**Step 1: Run nix build**

Run: `just build`
Expected: builds successfully

**Step 2: Verify binary name**

Run: `./result/bin/chix --help`
Expected: shows `chix` CLI with `install-claude` subcommand

**Step 3: Verify plugin artifacts are installed**

Run: `ls result/share/purse-first/nix/`
Expected: `plugin.json` and `hooks/format-nix`

**Step 4: Run tests**

Run: `just test`
Expected: all tests pass

**Step 5: Run check**

Run: `just check`
Expected: no errors from cargo check or clippy

---

### Task 10: Final commit — squash if desired

At this point the branch has the full migration. Review git log and decide whether to keep granular commits or squash.
