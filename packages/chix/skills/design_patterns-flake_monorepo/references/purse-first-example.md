# Purse-First: Path Inputs to Direct Imports

Complete walkthrough of migrating purse-first from `path:` flake inputs to
plain Nix function imports. This eliminated 4 path inputs (`go`, `shell`,
`bats`, `rust`) that broke transitive consumers on `nix flake update`.

## Before: Path Inputs

The top-level `flake.nix` declared sub-flakes as path inputs with `follows`:

```nix
inputs = {
  nixpkgs.url = "github:NixOS/nixpkgs/<sha>";
  nixpkgs-master.url = "github:NixOS/nixpkgs/<sha>";
  utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";

  go = {
    url = "path:./devenvs/go";
    inputs.nixpkgs.follows = "nixpkgs";
    inputs.nixpkgs-master.follows = "nixpkgs-master";
    inputs.utils.follows = "utils";
  };
  shell = {
    url = "path:./devenvs/shell";
    inputs.nixpkgs.follows = "nixpkgs";
    inputs.nixpkgs-master.follows = "nixpkgs-master";
    inputs.utils.follows = "utils";
  };
  bats = {
    url = "path:./devenvs/bats";
    inputs.nixpkgs.follows = "nixpkgs";
    inputs.nixpkgs-master.follows = "nixpkgs-master";
    inputs.utils.follows = "utils";
  };
  rust = {
    url = "path:./devenvs/rust";
    inputs.nixpkgs.follows = "nixpkgs";
    inputs.nixpkgs-master.follows = "nixpkgs-master";
    inputs.utils.follows = "utils";
  };

  crane.url = "github:ipetkov/crane";
  rust-overlay = {
    url = "github:oxalica/rust-overlay";
    inputs.nixpkgs.follows = "nixpkgs";
  };
  fh.url = "https://flakehub.com/f/DeterminateSystems/fh/*.tar.gz";
};
```

Sub-flakes contained all logic inline. For example, `devenvs/go/flake.nix`:

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/<sha>";
    nixpkgs-master.url = "github:NixOS/nixpkgs/<sha>";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = { self, nixpkgs, utils, gomod2nix, nixpkgs-master }:
    { overlays = gomod2nix.overlays; }
    // (utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        pkgs-master = import nixpkgs-master { inherit system; };
        packages = {
          inherit (pkgs-master) delve gofumpt golangci-lint golines
            gopls gotools govulncheck parallel;
          inherit (pkgs) go;
          gomod2nix = gomod2nix.packages.${system}.default;
        };
      in {
        inherit packages;
        devShells.default = pkgs-master.mkShell {
          packages = builtins.attrValues packages;
          env = { GOPATH = "$HOME/.cache/go"; };
        };
      }
    ));
}
```

Usage in the top-level flake:

```nix
# devShells
devShellInputsFrom = system: [
  go.devShells.${system}.default
  shell.devShells.${system}.default
  bats.devShells.${system}.default
  rust.devShells.${system}.default
];

# Overlay for Go builds
goOverlay = go.overlays.default;

# Overlay for marketplace build
overlays = [ go.overlays.default ];
```

### Problems with this approach

1. `flake.lock` contained `"type": "path"` entries for each devenv
2. Transitive consumers (`dodder -> dodder-go -> purse-first`) failed on
   `nix flake update` because Nix resolved `path:./devenvs/go` against the
   consumer's source tree (NixOS/nix#14762)
3. Each path input added 4 `follows` declarations (nixpkgs, nixpkgs-master,
   utils, and any sub-input like gomod2nix)
4. `gomod2nix` was buried inside the `go` sub-flake, not visible at the
   top level

## After: Direct Imports

### Step 1: Extract `default.nix` for each devenv

Created `default.nix` files as plain Nix functions. Examples:

**Simple (bats):**
```nix
# devenvs/bats/default.nix
{ pkgs }:
{
  devShell = pkgs.mkShell {
    packages = with pkgs; [ bats parallel shellcheck shfmt ];
  };
}
```

**Complex (go) --- needs a flake reference for overlays:**
```nix
# devenvs/go/default.nix
{ pkgs, pkgs-master, gomod2nix }:
let
  packages = {
    inherit (pkgs-master) delve gofumpt golangci-lint golines
      gopls gotools govulncheck parallel;
    inherit (pkgs) go;
    gomod2nix = gomod2nix.packages.${pkgs.system}.default;
  };
in
{
  overlay = gomod2nix.overlays.default;
  inherit packages;
  devShell = pkgs-master.mkShell {
    packages = builtins.attrValues packages;
    env = { GOPATH = "$HOME/.cache/go"; };
  };
}
```

**Complex (rust) --- needs a flake reference for overlays:**
```nix
# devenvs/rust/default.nix
{ pkgs, pkgs-master, rust-overlay }:
let
  pkgs-rust = import pkgs-master.path {
    inherit (pkgs) system;
    overlays = [
      rust-overlay.overlays.default
      (final: prev: {
        rustToolchain =
          let rust = prev.rust-bin; in
          if builtins.pathExists ./rust-toolchain.toml then
            rust.fromRustupToolchainFile ./rust-toolchain.toml
          else if builtins.pathExists ./rust-toolchain then
            rust.fromRustupToolchainFile ./rust-toolchain
          else
            rust.stable.latest.default.override {
              extensions = [ "rust-src" "rustfmt" ];
            };
      })
    ];
  };
in
{
  devShell = pkgs-rust.mkShell {
    packages = [
      pkgs-rust.rustToolchain
      pkgs.openssl pkgs.pkg-config
      pkgs-rust.cargo-deny pkgs-rust.cargo-edit
      pkgs-rust.cargo-watch pkgs-rust.rust-analyzer
    ];
  };
}
```

### Step 2: Convert sub-flake to thin wrapper

Each `flake.nix` became a 3-line body that delegates to `default.nix`:

```nix
# devenvs/go/flake.nix (after)
{
  inputs = { /* same as before */ };

  outputs = { self, nixpkgs, utils, gomod2nix, nixpkgs-master }:
    { overlays = gomod2nix.overlays; }
    // (utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        pkgs-master = import nixpkgs-master { inherit system; };
        result = import ./default.nix { inherit pkgs pkgs-master gomod2nix; };
      in {
        inherit (result) packages;
        devShells.default = result.devShell;
      }
    ));
}
```

The standalone wrapper preserves backward compatibility for existing `?dir=`
consumers and direnv users.

### Step 3: Update top-level flake

**Inputs:** Removed 4 path inputs, hoisted `gomod2nix` to top-level:

```nix
inputs = {
  nixpkgs.url = "github:NixOS/nixpkgs/<sha>";
  nixpkgs-master.url = "github:NixOS/nixpkgs/<sha>";
  utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";

  gomod2nix = {                          # NEW: hoisted from devenvs/go
    url = "github:nix-community/gomod2nix";
    inputs.nixpkgs.follows = "nixpkgs";
  };
  crane.url = "github:ipetkov/crane";
  rust-overlay = {
    url = "github:oxalica/rust-overlay";
    inputs.nixpkgs.follows = "nixpkgs";
  };
  fh.url = "https://flakehub.com/f/DeterminateSystems/fh/*.tar.gz";
};
```

**Builder helper:** Added `buildDevenvs` to centralize imports:

```nix
buildDevenvs = system:
  let
    pkgs = import nixpkgs { inherit system; };
    pkgs-master = import nixpkgs-master { inherit system; };
  in {
    go = import ./devenvs/go { inherit pkgs pkgs-master gomod2nix; };
    shell = import ./devenvs/shell { inherit pkgs; };
    bats = import ./devenvs/bats { inherit pkgs; };
    rust = import ./devenvs/rust { inherit pkgs pkgs-master rust-overlay; };
  };
```

**Usage sites updated:**

```nix
# devShellInputsFrom (was: go.devShells.${system}.default)
devShellInputsFrom = system:
  let devenvs = buildDevenvs system; in [
    devenvs.go.devShell
    devenvs.shell.devShell
    devenvs.bats.devShell
    devenvs.rust.devShell
  ];
```

Note: The `goOverlay` (gomod2nix overlay for `buildGoApplication`) was later
removed entirely when Go packages migrated to the workspace build pattern
(`buildGoModule` + `go work vendor`). See `design_patterns-go_nix_monorepo`
for that pattern.

**New outputs:** Exposed devShells for consumers:

```nix
devShells = {
  default = marketplaceOutputs.devShells.${system}.default;
  go = devenvs.go.devShell;
  shell = devenvs.shell.devShell;
  bats = devenvs.bats.devShell;
  rust = devenvs.rust.devShell;
};
```

### Step 4: Regenerate flake.lock

Running `nix flake lock` removed all `"type": "path"` entries and added the
`gomod2nix` top-level entry. The lock file shrank by 115 lines.

## Results

| Metric | Before | After |
|--------|--------|-------|
| Path inputs in flake.lock | 4 | 0 |
| `follows` declarations for devenvs | 16 | 0 |
| Top-level inputs | 8 (4 path + 4 GitHub) | 7 (all GitHub/FlakeHub) |
| Transitive `nix flake update` | Broken | Works |
| Standalone `nix develop ./devenvs/go` | Works | Works |
| Named devShell outputs | None | 4 (`go`, `shell`, `bats`, `rust`) |

## Consumer Migration Path

Existing consumers migrate from `?dir=` sub-flake inputs to named devShell outputs:

```nix
# Before
devenv-go.url = "github:amarbel-llc/purse-first?dir=devenvs/go";
inputsFrom = [ devenv-go.devShells.${system}.default ];

# After
purse-first.url = "github:amarbel-llc/purse-first";
inputsFrom = [ purse-first.devShells.${system}.go ];
```

Benefits: fewer inputs per consumer, single flake.lock entry for purse-first,
no transitive path resolution issues.
