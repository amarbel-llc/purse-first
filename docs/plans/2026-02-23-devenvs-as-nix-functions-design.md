# Devenvs as Plain Nix Functions

## Problem

Purse-first's `flake.nix` declares devenv sub-flakes as relative path inputs:

```nix
go = { url = "path:./devenvs/go"; ... };
bats = { url = "path:./devenvs/bats"; ... };
```

When consumers depend on purse-first transitively (e.g., dodder -> dodder-go ->
purse-first), Nix resolves these relative paths against the wrong source tree
([NixOS/nix#14762](https://github.com/nixos/nix/issues/14762)). This breaks
`nix flake update` for all transitive consumers.

## Design

Convert devenvs from flake inputs to plain Nix functions. The top-level
`flake.nix` imports them directly; no `path:` inputs remain.

### File Structure (per devenv)

```
devenvs/go/
  default.nix   # plain Nix function
  flake.nix     # thin standalone wrapper (for direnv / nix develop)
  flake.lock
```

### `default.nix` Contract

Each devenv exports a function taking concrete Nix values (not flake inputs):

```nix
# devenvs/go/default.nix
{ pkgs, pkgs-master, gomod2nix }:
{
  devShell = pkgs-master.mkShell { ... };
  overlay = gomod2nix.overlays.default;
  packages = { go = pkgs.go; gopls = pkgs-master.gopls; ... };
}
```

Arguments per devenv:

| Devenv | Args beyond `pkgs, pkgs-master` |
|--------|-------------------------------|
| bats   | none                          |
| shell  | none                          |
| go     | `gomod2nix`                   |
| rust   | `rust-overlay`                |

### `flake.nix` Wrapper

The standalone `flake.nix` imports `default.nix` and supplies its own inputs.
This keeps direnv and `nix develop ./devenvs/go` working:

```nix
# devenvs/go/flake.nix
{
  inputs = { nixpkgs, nixpkgs-master, utils, gomod2nix };
  outputs = { self, nixpkgs, nixpkgs-master, utils, gomod2nix }:
    utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        pkgs-master = import nixpkgs-master { inherit system; };
        result = import ./default.nix { inherit pkgs pkgs-master gomod2nix; };
      in {
        inherit (result) packages;
        overlays = { default = result.overlay; };
        devShells.default = result.devShell;
      }
    );
}
```

### Top-Level Consumption

Purse-first's `flake.nix` removes all `path:./devenvs/*` inputs and imports
directly:

```nix
outputs = { self, nixpkgs, nixpkgs-master, utils, gomod2nix, rust-overlay, ... }:
  let
    buildDevenvs = system:
      let
        pkgs = import nixpkgs { inherit system; };
        pkgs-master = import nixpkgs-master { inherit system; };
      in {
        go = import ./devenvs/go { inherit pkgs pkgs-master gomod2nix; };
        shell = import ./devenvs/shell { inherit pkgs pkgs-master; };
        bats = import ./devenvs/bats { inherit pkgs pkgs-master; };
        rust = import ./devenvs/rust { inherit pkgs pkgs-master rust-overlay; };
      };
  in
    utils.lib.eachDefaultSystem (system:
      let devenvs = buildDevenvs system; in {
        devShells = {
          go = devenvs.go.devShell;
          shell = devenvs.shell.devShell;
          bats = devenvs.bats.devShell;
          rust = devenvs.rust.devShell;
        };
        overlays.go = devenvs.go.overlay;
      }
    );
```

### Consumer Migration

Consumers switch from sub-flake `?dir=` inputs to purse-first outputs:

Before:
```nix
devenv-go.url = "github:amarbel-llc/purse-first?dir=devenvs/go";
inputsFrom = [ devenv-go.devShells.${system}.default ];
goOverlay = devenv-go.overlays.default;
```

After:
```nix
purse-first.url = "github:amarbel-llc/purse-first";
inputsFrom = [ purse-first.devShells.${system}.go ];
goOverlay = purse-first.overlays.${system}.go;
```

### Hoisted Inputs

`gomod2nix` becomes a new top-level input (justified: purse-first already
depends on it for Go package builds). `rust-overlay` is already top-level.

## Eng Follow-Up

The same pattern applies to all 26 devenvs in `amarbel-llc/eng`. Consumer repos
(~17) migrate from `eng?dir=devenvs/go` to `eng.devShells.${system}.go`. This
is a separate effort.

## Agent Ergonomics

- Modifying a devenv: edit `default.nix` (plain function, explicit args)
- Adding a dependency: add function parameter to `default.nix`, update call
  sites in same-directory `flake.nix` and top-level `flake.nix`
- Missing arg: Nix fails immediately with
  `function called without required argument 'foo'`
