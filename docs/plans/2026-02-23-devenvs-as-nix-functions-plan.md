# Devenvs as Plain Nix Functions — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove all `path:./devenvs/*` flake inputs from purse-first, replacing them with plain Nix function imports so transitive consumers don't break on `nix flake update`.

**Architecture:** Each devenv gets a `default.nix` (plain Nix function). The standalone `flake.nix` becomes a thin wrapper that imports `default.nix`. The top-level `flake.nix` imports `default.nix` directly — no path inputs. `gomod2nix` is hoisted to a top-level input. Devenv outputs are exposed as `devShells.<system>.{go,shell,bats,rust}` and `overlays.<system>.go`.

**Tech Stack:** Nix flakes, nixfmt-rfc-style

---

### Task 1: Create `devenvs/bats/default.nix`

Simplest devenv — no extra dependencies. Establishes the pattern.

**Files:**
- Create: `devenvs/bats/default.nix`

**Step 1: Write `default.nix`**

```nix
# devenvs/bats/default.nix
{ pkgs }:
{
  devShell = pkgs.mkShell {
    packages = with pkgs; [
      bats
      parallel
      shellcheck
      shfmt
      # TODO: add bats.libraries.bats-support and bats.libraries.bats-assert
    ];
  };
}
```

**Step 2: Verify it evaluates**

Run: `nix eval --file devenvs/bats/default.nix --apply 'f: builtins.attrNames (f { pkgs = import <nixpkgs> {}; })'`

Expected: `[ "devShell" ]`

**Step 3: Commit**

```
git add devenvs/bats/default.nix
git commit -m "refactor(devenvs/bats): extract default.nix from flake"
```

---

### Task 2: Update `devenvs/bats/flake.nix` to wrap `default.nix`

**Files:**
- Modify: `devenvs/bats/flake.nix`

**Step 1: Rewrite `flake.nix` as thin wrapper**

```nix
{
  description = "A Nix-flake-based BATS development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/6d41bc27aaf7b6a3ba6b169db3bd5d6159cfaa47";
    nixpkgs-master.url = "github:NixOS/nixpkgs/5b7e21f22978c4b740b3907f3251b470f466a9a2";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
  };

  outputs =
    {
      self,
      nixpkgs,
      utils,
      nixpkgs-master,
    }:
    (utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
        result = import ./default.nix { inherit pkgs; };
      in
      {
        inherit (result) devShell;
      }
    ));
}
```

**Step 2: Verify standalone flake still works**

Run: `nix flake show ./devenvs/bats`

Expected: Shows `devShells.<system>.default`

**Step 3: Commit**

```
git add devenvs/bats/flake.nix
git commit -m "refactor(devenvs/bats): flake.nix wraps default.nix"
```

---

### Task 3: Create `devenvs/shell/default.nix` and update wrapper

Same pattern as bats — no extra dependencies.

**Files:**
- Create: `devenvs/shell/default.nix`
- Modify: `devenvs/shell/flake.nix`

**Step 1: Write `default.nix`**

```nix
# devenvs/shell/default.nix
{ pkgs }:
{
  devShell = pkgs.mkShell {
    packages = with pkgs; [
      bats
      nodePackages.bash-language-server
      shellcheck
      shfmt
    ];
  };
}
```

**Step 2: Rewrite `flake.nix` as thin wrapper**

```nix
{
  description = "A Nix-flake-based Shell development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/6d41bc27aaf7b6a3ba6b169db3bd5d6159cfaa47";
    nixpkgs-master.url = "github:NixOS/nixpkgs/5b7e21f22978c4b740b3907f3251b470f466a9a2";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
  };

  outputs =
    {
      self,
      nixpkgs,
      utils,
      nixpkgs-master,
    }:
    (utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
        result = import ./default.nix { inherit pkgs; };
      in
      {
        inherit (result) devShell;
      }
    ));
}
```

**Step 3: Verify**

Run: `nix flake show ./devenvs/shell`

**Step 4: Commit**

```
git add devenvs/shell/default.nix devenvs/shell/flake.nix
git commit -m "refactor(devenvs/shell): extract default.nix, flake wraps it"
```

---

### Task 4: Create `devenvs/go/default.nix` and update wrapper

Go is more complex — it exports an overlay and packages, and needs `gomod2nix`.

**Files:**
- Create: `devenvs/go/default.nix`
- Modify: `devenvs/go/flake.nix`

**Step 1: Write `default.nix`**

```nix
# devenvs/go/default.nix
#
# Args:
#   pkgs        — stable nixpkgs
#   pkgs-master — unstable nixpkgs (for latest tooling)
#   gomod2nix   — the gomod2nix flake (for overlay + CLI package)
#
{ pkgs, pkgs-master, gomod2nix }:
let
  packages = {
    inherit (pkgs-master)
      delve
      gofumpt
      golangci-lint
      golines
      gopls
      gotools
      govulncheck
      parallel
      ;

    inherit (pkgs)
      go
      ;

    gomod2nix = gomod2nix.packages.${pkgs.system}.default;
  };
in
{
  overlay = gomod2nix.overlays.default;
  inherit packages;

  devShell = pkgs-master.mkShell {
    packages = builtins.attrValues packages;

    env = {
      GOPATH = "$HOME/.cache/go";
    };
  };
}
```

**Step 2: Rewrite `flake.nix` as thin wrapper**

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/6d41bc27aaf7b6a3ba6b169db3bd5d6159cfaa47";
    nixpkgs-master.url = "github:NixOS/nixpkgs/5b7e21f22978c4b740b3907f3251b470f466a9a2";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";

    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      utils,
      gomod2nix,
      nixpkgs-master,
    }:
    {
      overlays = gomod2nix.overlays;
    }
    // (utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
        pkgs-master = import nixpkgs-master { inherit system; };
        result = import ./default.nix { inherit pkgs pkgs-master gomod2nix; };
      in
      {
        inherit (result) packages;
        devShells.default = result.devShell;
      }
    ));
}
```

Note: The standalone flake preserves `overlays = gomod2nix.overlays` at the
top level (outside `eachDefaultSystem`) so that existing `?dir=devenvs/go`
consumers still see `overlays.default`. The `default.nix` also exposes
`overlay` for the top-level flake to use.

**Step 3: Verify**

Run: `nix flake show ./devenvs/go`

Expected: Shows `overlays.default`, `packages.<system>.*`, `devShells.<system>.default`

**Step 4: Commit**

```
git add devenvs/go/default.nix devenvs/go/flake.nix
git commit -m "refactor(devenvs/go): extract default.nix, flake wraps it"
```

---

### Task 5: Create `devenvs/rust/default.nix` and update wrapper

Rust needs `rust-overlay` and has the `rust-toolchain.toml` detection logic.

**Files:**
- Create: `devenvs/rust/default.nix`
- Modify: `devenvs/rust/flake.nix`

**Step 1: Write `default.nix`**

```nix
# devenvs/rust/default.nix
#
# Args:
#   pkgs          — stable nixpkgs
#   pkgs-master   — unstable nixpkgs
#   rust-overlay  — the rust-overlay flake
#
{ pkgs, pkgs-master, rust-overlay }:
let
  pkgs-rust = import pkgs-master.path {
    inherit (pkgs) system;
    overlays = [
      rust-overlay.overlays.default
      (final: prev: {
        rustToolchain =
          let
            rust = prev.rust-bin;
          in
          if builtins.pathExists ./rust-toolchain.toml then
            rust.fromRustupToolchainFile ./rust-toolchain.toml
          else if builtins.pathExists ./rust-toolchain then
            rust.fromRustupToolchainFile ./rust-toolchain
          else
            rust.stable.latest.default.override {
              extensions = [
                "rust-src"
                "rustfmt"
              ];
            };
      })
    ];
  };
in
{
  devShell = pkgs-rust.mkShell {
    packages = [
      pkgs-rust.rustToolchain
      pkgs.openssl
      pkgs.pkg-config
      pkgs-rust.cargo-deny
      pkgs-rust.cargo-edit
      pkgs-rust.cargo-watch
      pkgs-rust.rust-analyzer
    ];
  };
}
```

**Step 2: Rewrite `flake.nix` as thin wrapper**

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/6d41bc27aaf7b6a3ba6b169db3bd5d6159cfaa47";
    nixpkgs-master.url = "github:NixOS/nixpkgs/5b7e21f22978c4b740b3907f3251b470f466a9a2";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
    rust-overlay = {
      url = "github:oxalica/rust-overlay";
      inputs.nixpkgs.follows = "nixpkgs-master";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      utils,
      rust-overlay,
      nixpkgs-master,
    }:
    (utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
        pkgs-master = import nixpkgs-master { inherit system; };
        result = import ./default.nix { inherit pkgs pkgs-master rust-overlay; };
      in
      {
        inherit (result) devShell;
      }
    ));
}
```

**Step 3: Verify**

Run: `nix flake show ./devenvs/rust`

Expected: Shows `devShells.<system>.default`

**Step 4: Commit**

```
git add devenvs/rust/default.nix devenvs/rust/flake.nix
git commit -m "refactor(devenvs/rust): extract default.nix, flake wraps it"
```

---

### Task 6: Update top-level `flake.nix` — remove path inputs, add `gomod2nix`, import devenvs

This is the core change. Remove 4 path inputs, add 1 GitHub input, rewire all
usage sites.

**Files:**
- Modify: `flake.nix`

**Step 1: Replace inputs block**

Remove `go`, `shell`, `bats`, `rust` path inputs. Add `gomod2nix`:

```nix
inputs = {
  nixpkgs.url = "github:NixOS/nixpkgs/6d41bc27aaf7b6a3ba6b169db3bd5d6159cfaa47";
  nixpkgs-master.url = "github:NixOS/nixpkgs/5b7e21f22978c4b740b3907f3251b470f466a9a2";
  utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";

  gomod2nix = {
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

**Step 2: Update outputs function signature**

Remove `go`, `shell`, `bats`, `rust` from the outputs function params.
Add `gomod2nix`:

```nix
outputs =
  {
    self,
    nixpkgs,
    nixpkgs-master,
    utils,
    gomod2nix,
    crane,
    rust-overlay,
    fh,
  }:
```

**Step 3: Add devenv imports inside `buildPackages`**

After `let pkgs = import nixpkgs { inherit system; };` in `buildPackages`,
add:

```nix
pkgs-master = import nixpkgs-master { inherit system; };

goDevenv = import ./devenvs/go {
  inherit pkgs pkgs-master gomod2nix;
};
```

Then replace `goOverlay = go.overlays.default;` with:

```nix
goOverlay = goDevenv.overlay;
```

**Step 4: Add `buildDevenvs` helper and update `devShellInputsFrom`**

In the top-level `let` block (before `marketplaceOutputs`), add:

```nix
buildDevenvs = system:
  let
    pkgs = import nixpkgs { inherit system; };
    pkgs-master = import nixpkgs-master { inherit system; };
  in
  {
    go = import ./devenvs/go { inherit pkgs pkgs-master gomod2nix; };
    shell = import ./devenvs/shell { inherit pkgs; };
    bats = import ./devenvs/bats { inherit pkgs; };
    rust = import ./devenvs/rust { inherit pkgs pkgs-master rust-overlay; };
  };
```

Then update `devShellInputsFrom` in `marketplaceOutputs`:

```nix
devShellInputsFrom = system:
  let devenvs = buildDevenvs system;
  in [
    devenvs.go.devShell
    devenvs.shell.devShell
    devenvs.bats.devShell
    devenvs.rust.devShell
  ];
```

**Step 5: Update `purse-first-build.overlays`**

Change line 204 from `overlays = [ go.overlays.default ];` to:

```nix
overlays = [ gomod2nix.overlays.default ];
```

**Step 6: Expose devenv outputs**

In the `eachDefaultSystem` block at the bottom, add devShells and overlays
after the existing `devShells.default`:

```nix
devShells =
  let devenvs = buildDevenvs system;
  in {
    default = marketplaceOutputs.devShells.${system}.default;
    go = devenvs.go.devShell;
    shell = devenvs.shell.devShell;
    bats = devenvs.bats.devShell;
    rust = devenvs.rust.devShell;
  };

overlays =
  let devenvs = buildDevenvs system;
  in {
    go = devenvs.go.overlay;
  };
```

Replace the existing `devShells.default = ...;` line with this block.

**Step 7: Format**

Run: `nix run github:amarbel-llc/eng?dir=devenvs/nix#fmt -- flake.nix`

**Step 8: Commit**

```
git add flake.nix
git commit -m "refactor: remove path inputs, import devenvs as plain Nix functions

Eliminates path:./devenvs/* flake inputs that break transitive consumers
on nix flake update (NixOS/nix#14762). Devenvs are now plain Nix function
imports. gomod2nix hoisted to top-level input.

Exposes devShells.{go,shell,bats,rust} and overlays.go as outputs so
consumers can use purse-first outputs instead of ?dir= sub-flake inputs."
```

---

### Task 7: Update `flake.lock` — remove stale path entries

**Files:**
- Modify: `flake.lock`

**Step 1: Lock the flake**

Run: `nix flake lock`

This regenerates `flake.lock` without the removed path inputs and with the
new `gomod2nix` input.

**Step 2: Verify lock is clean**

Run: `grep -c '"path"' flake.lock`

Expected: `0` (no remaining path-type entries)

**Step 3: Commit**

```
git add flake.lock
git commit -m "chore: regenerate flake.lock after removing path inputs"
```

---

### Task 8: Build and verify

**Step 1: Build the default package**

Run: `nix build --show-trace`

Expected: Builds successfully, `./result` contains purse-first marketplace.

**Step 2: Check flake outputs**

Run: `nix flake show`

Expected output includes:
- `devShells.<system>.default`
- `devShells.<system>.go`
- `devShells.<system>.shell`
- `devShells.<system>.bats`
- `devShells.<system>.rust`
- `overlays.<system>.go`
- `packages.<system>.default`
- all other existing outputs

**Step 3: Verify devShell enters**

Run: `nix develop -c echo ok`

Expected: Prints `ok` after entering shell.

**Step 4: Verify standalone devenvs still work**

Run: `nix flake show ./devenvs/go && nix flake show ./devenvs/bats && nix flake show ./devenvs/shell && nix flake show ./devenvs/rust`

Expected: Each shows its outputs (devShells, packages, overlays as appropriate).

**Step 5: Run tests**

Run: `just test`

Expected: All tests pass.

---

### Task 9: Format all changed Nix files

**Files:**
- Modify: `devenvs/bats/default.nix`, `devenvs/shell/default.nix`, `devenvs/go/default.nix`, `devenvs/rust/default.nix`
- Modify: `devenvs/bats/flake.nix`, `devenvs/shell/flake.nix`, `devenvs/go/flake.nix`, `devenvs/rust/flake.nix`
- Modify: `flake.nix`

**Step 1: Format**

Run: `nix run github:amarbel-llc/eng?dir=devenvs/nix#fmt -- devenvs/bats/default.nix devenvs/shell/default.nix devenvs/go/default.nix devenvs/rust/default.nix devenvs/bats/flake.nix devenvs/shell/flake.nix devenvs/go/flake.nix devenvs/rust/flake.nix flake.nix`

**Step 2: Commit if changed**

```
git add -u
git commit -m "style: format nix files"
```
