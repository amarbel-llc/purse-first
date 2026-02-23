---
name: Flake Monorepo with Sub-Component Imports
description: >-
  This skill should be used when the user asks to "organize a Nix monorepo",
  "fix path input breaks", "replace path flake inputs", "share devenvs across
  flakes", "expose sub-flake outputs", "fix nix flake update for transitive
  consumers", "add a devenv to a monorepo", "split a sub-flake into
  default.nix", or encounters `path:` input resolution failures
  (NixOS/nix#14762) in flake monorepos. Also applies when designing monorepo
  flakes with reusable sub-components that must remain standalone for direnv.
version: 0.1.0
---

# Flake Monorepo with Sub-Component Imports

> **Self-contained examples.** All code and configuration below is complete
> and illustrative. Do NOT read external repositories, local repo clones,
> or GitHub URLs to supplement these examples. Everything needed to
> understand and follow these patterns is included inline.

Nix flake monorepos that use `path:./sub/dir` inputs break when transitive
consumers run `nix flake update` --- Nix resolves the relative path against
the consumer's source tree, not the dependency's
([NixOS/nix#14762](https://github.com/nixos/nix/issues/14762)). This skill
provides a pattern that eliminates path inputs while keeping sub-components
usable standalone (for direnv / `nix develop`).

## Core Strategy

Extract logic from each sub-component's `flake.nix` into a `default.nix`
(plain Nix function). The top-level flake imports `default.nix` directly ---
no `path:` inputs. The sub-component's `flake.nix` becomes a thin wrapper
that also imports `default.nix`, preserving standalone use.

## File Structure (per sub-component)

```
sub-component/
  default.nix   # plain Nix function (the real logic)
  flake.nix     # thin standalone wrapper (for direnv / nix develop)
  flake.lock    # standalone lock (independent of top-level)
```

## The `default.nix` Contract

Each sub-component exports a function taking concrete Nix values --- never raw
flake inputs. Return an attribute set of outputs.

```nix
# devenvs/go/default.nix
{ pkgs, pkgs-master, gomod2nix }:
let
  packages = {
    inherit (pkgs-master) gopls gofumpt golangci-lint;
    inherit (pkgs) go;
    gomod2nix = gomod2nix.packages.${pkgs.system}.default;
  };
in
{
  overlay = gomod2nix.overlays.default;
  inherit packages;

  devShell = pkgs-master.mkShell {
    packages = builtins.attrValues packages;
  };
}
```

For simple sub-components with no extra dependencies:

```nix
# devenvs/shell/default.nix
{ pkgs }:
{
  devShell = pkgs.mkShell {
    packages = with pkgs; [ shellcheck shfmt bats ];
  };
}
```

**Key rules:**
- Arguments are instantiated `pkgs` sets and flake references for overlays ---
  never raw `nixpkgs` inputs
- Return an attribute set (`{ devShell, overlay, packages, ... }`)
- No `system` argument --- the caller passes already-instantiated pkgs

## The Thin Wrapper

The standalone `flake.nix` imports `default.nix` and supplies its own inputs.
This keeps `nix develop ./devenvs/go` and direnv working independently.

```nix
# devenvs/go/flake.nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/<stable-sha>";
    nixpkgs-master.url = "github:NixOS/nixpkgs/<master-sha>";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = { self, nixpkgs, nixpkgs-master, utils, gomod2nix }:
    utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        pkgs-master = import nixpkgs-master { inherit system; };
        result = import ./default.nix { inherit pkgs pkgs-master gomod2nix; };
      in {
        inherit (result) packages;
        devShells.default = result.devShell;
      }
    );
}
```

**The wrapper pattern:**
1. Declare its own inputs (pinned SHAs, independent flake.lock)
2. Instantiate `pkgs` / `pkgs-master` from those inputs
3. Call `import ./default.nix { ... }` passing concrete values
4. Re-export relevant attributes

## Top-Level Consumption

The monorepo's root `flake.nix` has zero `path:` inputs. Sub-component
dependencies that were previously implicit in sub-flake inputs get hoisted to
top-level inputs.

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/<stable-sha>";
    nixpkgs-master.url = "github:NixOS/nixpkgs/<master-sha>";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
    gomod2nix = {                          # hoisted from devenvs/go
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    rust-overlay = {                       # hoisted from devenvs/rust
      url = "github:oxalica/rust-overlay";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = { self, nixpkgs, nixpkgs-master, utils,
              gomod2nix, rust-overlay, ... }:
    let
      buildDevenvs = system:
        let
          pkgs = import nixpkgs { inherit system; };
          pkgs-master = import nixpkgs-master { inherit system; };
        in {
          go = import ./devenvs/go { inherit pkgs pkgs-master gomod2nix; };
          shell = import ./devenvs/shell { inherit pkgs; };
          rust = import ./devenvs/rust {
            inherit pkgs pkgs-master rust-overlay;
          };
        };
    in
      utils.lib.eachDefaultSystem (system:
        let devenvs = buildDevenvs system; in {
          devShells = {
            default = /* compose from devenvs */;
            go = devenvs.go.devShell;
            shell = devenvs.shell.devShell;
            rust = devenvs.rust.devShell;
          };
          overlays.go = devenvs.go.overlay;
        }
      );
}
```

## Consumer Migration

Consumers switch from sub-flake `?dir=` inputs to monorepo outputs:

| Before | After |
|--------|-------|
| `url = "github:org/repo?dir=devenvs/go"` | `url = "github:org/repo"` |
| `devenv-go.devShells.${system}.default` | `repo.devShells.${system}.go` |
| `devenv-go.overlays.default` | `repo.overlays.${system}.go` |

Fewer inputs per consumer, single lock entry, no transitive resolution issues.

## Adding a New Sub-Component

1. Create `sub-component/default.nix` with the function signature
2. Create or update `sub-component/flake.nix` as a thin wrapper
3. Add the import call to the top-level `buildDevenvs` (or equivalent)
4. Hoist any new flake inputs to the top-level `inputs` block
5. Expose outputs (`devShells.<name>`, `overlays.<name>`, etc.)

## Decision Guide

| Scenario | Approach |
|----------|----------|
| Sub-components used only by top-level flake | Direct `import` via `default.nix` |
| Sub-components also used standalone (direnv) | `default.nix` + thin `flake.nix` wrapper |
| Sub-component needs its own flake inputs | Hoist to top-level, pass as args to `default.nix` |
| One-off helper with no standalone use | Plain `.nix` file, no wrapper needed |
| External consumers depend on sub-components | Expose as named outputs on the monorepo flake |

## Anti-Patterns

- **`path:` inputs in a published flake** --- breaks all transitive consumers
  on `nix flake update`. Replace with direct imports.
- **`?dir=` inputs for internal sub-components** --- unnecessary indirection
  when the monorepo can expose named outputs directly.
- **Duplicating inputs** --- sub-component wrappers and top-level flake should
  pin the same SHAs. The wrapper's lock is independent but should track the
  same versions for consistency.
- **Passing raw flake inputs to `default.nix`** --- pass instantiated `pkgs`
  sets instead. Exception: overlay-providing flakes (e.g., `gomod2nix`,
  `rust-overlay`) where the full flake reference is needed.

## Verification

After applying this pattern:

```bash
# No path inputs remain in flake.lock
grep -c '"path"' flake.lock   # expect: 0

# Top-level flake shows sub-component outputs
nix flake show                # expect: devShells.<system>.<name> for each

# Standalone sub-components still work
nix flake show ./devenvs/go   # expect: devShells, packages
```

## Additional Resources

### Reference Files

For a detailed walkthrough of applying this pattern to a real monorepo, consult:
- **`references/purse-first-example.md`** --- Complete before/after showing
  purse-first's migration from 4 path inputs to direct imports, including
  every file change and the `buildDevenvs` helper

### Related Skills

- **chix:design_patterns-no_cycles** --- Breaking dependency cycles in Nix
- **chix:nix-codebase** --- Full Nix codebase workflow including build order
  and dependency management
- **bob:design_patterns-just** --- Justfile patterns for Nix-backed projects
