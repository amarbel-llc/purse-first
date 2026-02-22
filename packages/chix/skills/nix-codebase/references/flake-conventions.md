# Flake Conventions Reference

Detailed reference for Nix flake structure, conventions, and patterns used across the codebase.

## Stable-First Nixpkgs Rationale

Two nixpkgs inputs are used in every flake:

- **`nixpkgs`** (stable): For language runtimes (`go`, `rustc`), core libraries, and anything that needs reliability. Stable packages change infrequently and are well-tested.

- **`nixpkgs-master`** (unstable/master): For development tooling (`gopls`, `golangci-lint`, `gofumpt`, `nil`, `rust-analyzer`). These benefit from being the latest version for IDE integration and linting accuracy.

Pinned SHA commits are stored in the eng monorepo and shared across all projects for consistency. Update with `just update-nixpkgs` from the eng repo root.

## Standard Flake Inputs

### Go Project

```nix
inputs = {
  nixpkgs.url = "github:NixOS/nixpkgs/<stable-sha>";
  nixpkgs-master.url = "github:NixOS/nixpkgs/<master-sha>";
  utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
  go.url = "github:friedenberg/eng?dir=devenvs/go";
  shell.url = "github:friedenberg/eng?dir=devenvs/shell";
};
```

### Rust Project (Simple)

```nix
inputs = {
  devenv-rust.url = "github:friedenberg/eng?dir=devenvs/rust";
  nixpkgs.follows = "devenv-rust/nixpkgs";
  utils.follows = "devenv-rust/utils";
};
```

### Rust Project (Complex, with crane)

```nix
inputs = {
  nixpkgs.url = "github:NixOS/nixpkgs/<stable-sha>";
  nixpkgs-master.url = "github:NixOS/nixpkgs/<master-sha>";
  utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
  rust-overlay.url = "github:oxalica/rust-overlay";
  crane.url = "github:ipetkov/crane";
  rust.url = "github:friedenberg/eng?dir=devenvs/rust";
};
```

### Shell/Skill Plugin

```nix
inputs = {
  nixpkgs.url = "github:NixOS/nixpkgs/<stable-sha>";
  nixpkgs-master.url = "github:NixOS/nixpkgs/<master-sha>";
  utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
  shell.url = "github:friedenberg/eng?dir=devenvs/shell";
};
```

## Complete Go Project Flake Template

```nix
{
  description = "Project description";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/<stable-sha>";
    nixpkgs-master.url = "github:NixOS/nixpkgs/<master-sha>";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
    go.url = "github:friedenberg/eng?dir=devenvs/go";
    shell.url = "github:friedenberg/eng?dir=devenvs/shell";
  };

  outputs =
    {
      self,
      nixpkgs,
      nixpkgs-master,
      utils,
      go,
      shell,
    }:
    utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [ go.overlays.default ];
        };

        version = "0.1.0";

        myApp = pkgs.buildGoApplication {
          pname = "my-app";
          inherit version;
          src = ./.;
          modules = ./gomod2nix.toml;
          subPackages = [ "cmd/my-app" ];

          meta = with pkgs.lib; {
            description = "My application";
            homepage = "https://github.com/amarbel-llc/my-app";
            license = licenses.mit;
          };
        };
      in
      {
        packages.default = myApp;

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            just
            gum
          ];

          inputsFrom = [
            go.devShells.${system}.default
            shell.devShells.${system}.default
          ];
        };

        apps.default = {
          type = "app";
          program = "${myApp}/bin/my-app";
        };
      }
    );
}
```

## Complete Rust Project Flake Template (Simple)

```nix
{
  description = "Rust project description";

  inputs = {
    devenv-rust.url = "github:friedenberg/eng?dir=devenvs/rust";
    nixpkgs.follows = "devenv-rust/nixpkgs";
    utils.follows = "devenv-rust/utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      utils,
      devenv-rust,
    }:
    utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        packages.default = pkgs.rustPlatform.buildRustPackage {
          pname = "my-tool";
          version = "0.1.0";
          src = ./.;
          cargoLock = {
            lockFile = ./Cargo.lock;
          };

          meta = with pkgs.lib; {
            description = "My Rust tool";
            license = licenses.mit;
          };
        };

        devShells.default = devenv-rust.devShells.${system}.default;
      }
    );
}
```

## Complete Rust Project Flake Template (Complex, with crane)

```nix
{
  description = "Complex Rust project";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/<stable-sha>";
    nixpkgs-master.url = "github:NixOS/nixpkgs/<master-sha>";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
    rust-overlay.url = "github:oxalica/rust-overlay";
    crane.url = "github:ipetkov/crane";
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
        };

        cargoArtifacts = craneLib.buildDepsOnly commonArgs;

        myTool = craneLib.buildPackage (
          commonArgs
          // {
            inherit cargoArtifacts;
          }
        );
      in
      {
        packages.default = myTool;

        devShells.default = rust.devShells.${system}.default;
      }
    );
}
```

## DevShell Composition

DevShells compose via `inputsFrom`, which merges packages and environment from multiple shells:

```nix
devShells.default = pkgs.mkShell {
  # Project-specific tools
  packages = with pkgs; [ just gum ];

  # Inherit from devenv shells
  inputsFrom = [
    go.devShells.${system}.default      # go, gopls, gofumpt, gomod2nix, etc.
    shell.devShells.${system}.default   # shellcheck, shfmt, etc.
  ];
};
```

Available devenvs (from `github:friedenberg/eng?dir=devenvs/<name>`):

| Devenv | Provides |
|--------|----------|
| `go` | go, gopls, gofumpt, golangci-lint, gomod2nix, golines |
| `rust` | rustc, cargo, rust-analyzer, clippy, rustfmt |
| `shell` | shellcheck, shfmt |
| `nix` | nil, nixfmt-rfc-style, statix |
| `python` | python3, pip, black, ruff |
| `bats` | bats, shellcheck |

## MCP Server Installation Pattern

For Go MCP servers, expose an `install-mcp` app:

```nix
apps.install-mcp = {
  type = "app";
  program = toString (pkgs.writeShellScript "install-mcp" ''
    set -euo pipefail

    CLAUDE_CONFIG_DIR="''${HOME}/.claude"
    MCP_CONFIG_FILE="''${CLAUDE_CONFIG_DIR}/mcp.json"

    ${pkgs.gum}/bin/gum style --foreground 212 "Installing MCP server..."

    FLAKE_REF="github:amarbel-llc/my-project"

    NEW_SERVER=$(${pkgs.jq}/bin/jq -n \
      --arg cmd "nix" \
      --arg flake "$FLAKE_REF" \
      '{command: $cmd, args: ["run", $flake]}')

    if [[ -f "$MCP_CONFIG_FILE" ]]; then
      UPDATED=$(${pkgs.jq}/bin/jq \
        --argjson server "$NEW_SERVER" \
        '.mcpServers["my-server"] = $server' \
        "$MCP_CONFIG_FILE")
      echo "$UPDATED" > "$MCP_CONFIG_FILE"
    else
      mkdir -p "$CLAUDE_CONFIG_DIR"
      ${pkgs.jq}/bin/jq -n \
        --argjson server "$NEW_SERVER" \
        '{mcpServers: {"my-server": $server}}' > "$MCP_CONFIG_FILE"
    fi

    ${pkgs.gum}/bin/gum style --foreground 212 "Done!"
  '');
};
```

Justfile integration:

```
install:
    nix run .#install-mcp
```

## Direnv Integration

Every project uses a `.envrc` with:

```bash
source_up
use flake .
```

- `source_up`: Inherits parent `.envrc` (for monorepo context)
- `use flake .`: Loads the local flake's devShell

This ensures all devenv tools (including `gomod2nix`) are available when entering the project directory.

## Nix Formatting

Format all `.nix` files with `nixfmt-rfc-style`:

```bash
nix run github:friedenberg/eng?dir=devenvs/nix#fmt -- path/to/flake.nix
```

Or from within a nix devenv:

```bash
nix develop github:friedenberg/eng?dir=devenvs/nix --command nixfmt path/to/flake.nix
```

## Flake Lock Management

### Update all inputs

```bash
nix flake update
```

### Update a specific input

```bash
nix flake lock --update-input go
```

### Check inputs

```bash
nix flake metadata
```

## Platform-Specific Handling

Some builds need platform-specific configuration:

```nix
# Skip tests on macOS (common for network-dependent tests)
doCheck = !pkgs.stdenv.hostPlatform.isDarwin;

# Platform-specific build inputs
buildInputs = pkgs.lib.optionals pkgs.stdenv.isDarwin [
  pkgs.darwin.apple_sdk.frameworks.Security
];
```

## Aggregation with symlinkJoin

Combine multiple packages into a single output:

```nix
packages.default = pkgs.symlinkJoin {
  name = "combined";
  paths = [
    package1
    package2
    package3
  ];
};
```

Each component manages its own dependencies independently. The symlinkJoin creates a unified `bin/`, `share/`, etc.

## FlakeHub Publishing

All flakes are published to FlakeHub on every push to master. See `references/flakehub-ci.md` for the full CI reference.

### FlakeHub Input URLs

Use FlakeHub URLs for third-party flakes in `inputs`:

```nix
inputs = {
  # FlakeHub URL — preferred for third-party flakes
  utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";

  # Tarball format for specific versions
  fh.url = "https://flakehub.com/f/DeterminateSystems/fh/0.1.21.tar.gz";

  # GitHub URLs — for devenvs and pinned nixpkgs
  go.url = "github:friedenberg/eng?dir=devenvs/go";
  nixpkgs.url = "github:NixOS/nixpkgs/<stable-sha>";
};
```

### When to Use Which URL

| URL Type | Use For |
|----------|---------|
| `https://flakehub.com/f/...` | Third-party flakes on FlakeHub (`flake-utils`, `crane`, `fenix`, `fh`) |
| `github:friedenberg/eng?dir=devenvs/...` | Devenv references |
| `github:NixOS/nixpkgs/<sha>` | Pinned nixpkgs (stable and master) |
| `github:amarbel-llc/<repo>` | Unpublished repos or repos not yet on FlakeHub |

### Managing FlakeHub Inputs with `fh`

The `fh` CLI (available in `devenvs/nix`) manages FlakeHub inputs:

```bash
fh add numtide/flake-utils                        # Add input
fh add --input-name utils numtide/flake-utils     # Custom input name
fh add "NixOS/nixpkgs/0.2411.*"                   # Version constraint
```

This is used by the `just update-nix-repos` target in the eng monorepo to keep all repos' inputs in sync.
