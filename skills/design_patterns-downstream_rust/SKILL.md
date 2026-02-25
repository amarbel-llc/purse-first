---
name: "Downstream Rust: Consuming purse-first Libraries"
description: >-
  This skill should be used when the user asks to "add mcp-server dependency",
  "depend on rust-mcp", "consume rust-lib-mcp", "build a Rust MCP server
  outside purse-first", "migrate from amarbel-llc/rust-lib-mcp", or is building
  a Rust project that depends on purse-first libraries (like mcp-server) from a
  separate repository. Also applies when encountering dependency resolution
  questions about git deps vs path deps vs flake inputs for Rust crates across
  repo boundaries, or when a Rust MCP server needs to work both with and without
  Nix.
version: 0.1.0
---

# Downstream Rust: Consuming purse-first Libraries

> **Self-contained examples.** All code and configuration below is complete
> and illustrative. Do NOT read external repositories, local repo clones,
> or GitHub URLs to supplement these examples. Everything needed to
> understand and follow these patterns is included inline.

Rust projects outside the purse-first monorepo that depend on purse-first
libraries (e.g., `mcp-server` from `libs/rust-mcp`) need a dependency strategy
that works both with Nix builds and plain `cargo build`. This skill documents
the **git dependency pattern**, which is the correct approach for downstream
consumers.

## The Problem

Three ways to reference a Rust crate across repo boundaries:

1. **Path dependency** --- `mcp-server = { path = "../../libs/rust-mcp" }`
2. **Git dependency** --- `mcp-server = { git = "https://github.com/..." }`
3. **Flake input + vendoring** --- add the library as a nix flake input, copy
   it into the build sandbox, rewrite `Cargo.toml` paths

Each has trade-offs when a project must build in both nix and non-nix
environments.

## Decision Guide

| Scenario | Approach |
|----------|----------|
| Crate lives in the **same repo** (e.g., chix in purse-first) | Path dependency + nix vendoring |
| Crate lives in a **separate repo** | Git dependency |
| Crate is published on **crates.io** | Registry dependency |

**For downstream consumers, always use git dependencies.**

## Why Git Dependencies Win for Downstream

| | Git dep | Path dep | Flake input + vendoring |
|---|---|---|---|
| `cargo build` (no nix) | works | broken (path doesn't exist) | broken (path doesn't exist) |
| `nix build` (crane) | works (cargo fetches) | broken (cross-repo) | works (vendored) |
| Version pinning | `Cargo.lock` | N/A | `flake.lock` |
| Sources of truth | 1 | 1 (same-repo only) | 2 (Cargo.toml + flake.nix) |
| Maintenance | update Cargo.lock | none | update lock + keep sed in sync |

**Key insight:** Crane handles cargo git dependencies natively during
`buildDepsOnly`. No extra nix machinery is needed --- cargo fetches the git dep
as part of its normal dependency resolution inside the sandbox.

### Why Flake Input + Vendoring Is Wrong Here

The purse-first monorepo uses flake input vendoring for **chix** (its internal
Rust MCP server) because chix's `Cargo.toml` has a path dep
(`path = "../../libs/rust-mcp"`) that resolves within the monorepo. The nix
build copies rust-mcp source into the sandbox and rewrites the path:

```nix
# This pattern is ONLY for in-monorepo consumers (like chix)
chixSrc = pkgs.runCommand "chix-src" { } ''
  cp -r ${src} $out
  chmod -R u+w $out
  mkdir -p $out/vendor-libs
  cp -r ${rustMcpSrc} $out/vendor-libs/rust-mcp
  sed -i 's|path = "../../libs/rust-mcp"|path = "vendor-libs/rust-mcp"|' $out/Cargo.toml
'';
```

For a downstream project, this creates two problems:

1. **`cargo build` breaks** --- the path dep has nowhere to point outside the
   monorepo
2. **Dual source of truth** --- the git rev is pinned in both `Cargo.lock` (for
   non-nix) and `flake.lock` (for nix), which can drift

## The Pattern

### 1. Add the git dependency

```toml
# Cargo.toml
[dependencies]
mcp-server = { git = "https://github.com/amarbel-llc/rust-lib-mcp", features = ["tools"] }
```

Select features based on what the server needs:

| Feature | Purpose | Default |
|---------|---------|---------|
| `tools` | Tool trait and registry | yes |
| `resources` | Resource trait and registry | yes |
| `prompts` | Prompt trait and registry | yes |
| `sampling` | Server-initiated LLM requests | no |
| `completions` | Parameter completion (V1 only) | no |
| `command` | CLI command types | no |
| `command-executor` | Async subprocess execution | no |
| `http-transport` | HTTP server transport | no |

### 2. Build with Crane (nix)

A standard crane-based `flake.nix` handles git dependencies without any special
configuration:

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/<stable-sha>";
    nixpkgs-master.url = "github:NixOS/nixpkgs/<master-sha>";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
    rust-overlay = {
      url = "github:oxalica/rust-overlay";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    crane.url = "github:ipetkov/crane";
    rust.url = "github:amarbel-llc/eng?dir=devenvs/rust";
  };

  outputs = { self, nixpkgs, nixpkgs-master, utils, rust-overlay, crane, rust }:
    utils.lib.eachDefaultSystem (system:
      let
        overlays = [ (import rust-overlay) ];
        pkgs = import nixpkgs { inherit system overlays; };
        rustToolchain = pkgs.rust-bin.stable.latest.default;
        craneLib = (crane.mkLib pkgs).overrideToolchain rustToolchain;

        src = craneLib.cleanCargoSource ./.;
        commonArgs = { inherit src; strictDeps = true; };
        cargoArtifacts = craneLib.buildDepsOnly commonArgs;

        myPackage = craneLib.buildPackage (commonArgs // {
          inherit cargoArtifacts;
        });
      in {
        packages.default = myPackage;
        devShells.default = rust.devShells.${system}.default;
      }
    );
}
```

**No flake input for rust-mcp.** Crane calls cargo, cargo reads `Cargo.lock`,
cargo fetches the pinned git rev. The nix sandbox allows network access during
the fixed-output `buildDepsOnly` phase.

### 3. Pin and update

`Cargo.lock` pins the exact git rev. To update:

```bash
cargo update -p mcp-server
```

Or to pin a specific rev:

```toml
mcp-server = { git = "https://github.com/amarbel-llc/rust-lib-mcp", rev = "abc1234", features = ["tools"] }
```

## Implementing Tools

The `mcp-server` crate uses a trait-based pattern. Each tool is a struct
implementing the `Tool` trait:

```rust
use mcp_server::{McpServer, Tool, ToolResult, ToolError, Context};
use serde_json::{json, Value};

struct MyTool;

#[async_trait::async_trait]
impl Tool for MyTool {
    fn name(&self) -> &str { "my_tool" }
    fn description(&self) -> &str { "Does something useful" }

    fn input_schema(&self) -> Value {
        json!({
            "type": "object",
            "properties": {
                "param": { "type": "string", "description": "A parameter" }
            },
            "required": ["param"]
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context)
        -> Result<ToolResult, ToolError>
    {
        let param = arguments.get("param")
            .and_then(|v| v.as_str())
            .ok_or_else(|| ToolError::InvalidArguments("Missing param".into()))?;

        Ok(ToolResult::text(format!("Result: {}", param)))
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let server = McpServer::builder("my-server", env!("CARGO_PKG_VERSION"))
        .with_tool(MyTool)
        .build();

    server.run_stdio().await?;
    Ok(())
}
```

## Anti-Patterns

- **Flake input for downstream Rust deps** --- creates dual source of truth
  between `Cargo.lock` and `flake.lock`, requires `sed` rewriting in nix build,
  breaks plain `cargo build`
- **Path deps across repo boundaries** --- only works if the referenced path
  exists on the local filesystem; breaks CI and other developers' machines
- **`vendorHash` for Rust** --- that is a Go concept (`buildGoModule`). Crane
  handles Rust dependency vendoring automatically via `buildDepsOnly`
- **Skipping `Cargo.lock` from version control** --- always commit
  `Cargo.lock` for binaries so nix and cargo builds are reproducible

## Related Skills

- **chix:design_patterns-go_nix_monorepo** --- The Go equivalent, which uses
  workspace vendor builds with a shared `vendorHash`
- **bob:creating-packages** --- Packaging an MCP server for distribution via
  purse-first
- **bob:go-cli-framework** --- Building Go MCP servers with go-mcp (the Go
  counterpart to rust-mcp)
