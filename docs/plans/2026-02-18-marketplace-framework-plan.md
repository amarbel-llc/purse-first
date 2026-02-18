# Marketplace Framework Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Extract marketplace assembly logic into `lib/mkMarketplace.nix`, refactor purse-first to dogfood it, and provide a `nix flake init` template for bootstrapping new marketplaces.

**Architecture:** A single Nix function (`mkMarketplace`) encapsulates all marketplace assembly: symlinkJoin of plugins, meta plugin creation, CLI wrapping, marketplace.json generation, and no-hooks variant. Purse-first's `flake.nix` calls this function and also exports it + a flake template for downstream consumers.

**Tech Stack:** Nix (flake outputs, symlinkJoin, makeWrapper, runCommand), BATS (integration tests), Go (unchanged CLI)

**Reference:** Design doc at `docs/plans/2026-02-18-marketplace-framework-design.md`

---

### Task 1: Create `lib/mkMarketplace.nix` — Core Function

**Files:**
- Create: `lib/mkMarketplace.nix`

This is the main framework function. It accepts marketplace configuration and returns flake outputs. The logic is extracted from the current `flake.nix` lines 66-265.

**Step 1: Create the file with the full function**

```nix
# lib/mkMarketplace.nix
#
# mkMarketplace — build a Claude plugin marketplace from a set of plugins.
#
# Called by purse-first's own flake.nix and available to downstream consumers
# via `purse-first.lib.mkMarketplace`.
{
  # Required — Nix infrastructure
  nixpkgs,
  nixpkgs-master,
  utils,

  # Required — marketplace identity
  name,
  owner,

  # Required — plugin set
  # function: system → list of plugin derivations
  plugins,

  # Optional — how to obtain the purse-first CLI.
  # When purse-first builds itself, it passes its own source + build config.
  # Downstream consumers pass the purse-first package from the flake input.
  purse-first-cli ? null,
  purse-first-build ? null,

  # Optional — metadata
  description ? "${name} — Claude plugin marketplace",
  repo ? null,

  # Optional — customization
  pluginConfig ? null,
  skills ? null,
  pluginBaseJson ? null,

  # Optional — extra devShell configuration
  devShellPackages ? (_system: _pkgs: _pkgs-master: []),
  devShellInputsFrom ? (_system: []),
  devShellHook ? ''echo "${name} - dev environment"'',
}:

utils.lib.eachDefaultSystem (
  system:
  let
    pkgs = import nixpkgs { inherit system; };
    pkgs-master = import nixpkgs-master {
      inherit system;
      config.allowUnfree = true;
    };

    # Resolve the purse-first CLI package.
    # Self-build: purse-first-build is an attrset with src, modules, overlays, version.
    # Downstream: purse-first-cli is a package from the flake input.
    cli =
      if purse-first-cli != null then
        purse-first-cli.packages.${system}.purse-first
      else if purse-first-build != null then
        let
          pkgs-go = import nixpkgs {
            inherit system;
            overlays = purse-first-build.overlays or [ ];
          };
        in
        pkgs-go.buildGoApplication {
          pname = "purse-first";
          version = purse-first-build.version or "0.0.0";
          src = purse-first-build.src;
          modules = purse-first-build.modules;
          subPackages = [ "cmd/purse-first" ];
          CGO_ENABLED = "0";
          ldflags = [
            "-s"
            "-w"
          ];
          meta = with pkgs.lib; {
            description = "MCP-first tool routing for Claude Code";
            license = licenses.mit;
          };
        }
      else
        throw "mkMarketplace: must provide either purse-first-cli or purse-first-build";

    # Resolve plugin packages for this system.
    pluginPkgs = plugins system;

    # Build the meta plugin (skills carrier) if skills are provided.
    metaPlugin =
      if skills != null then
        pkgs.runCommand "${name}-meta"
          {
            nativeBuildInputs = [ cli ];
          }
          ''
            mkdir -p $out/share/purse-first/${name}/skills
            cp -r ${skills}/* $out/share/purse-first/${name}/skills/

            staging=$(mktemp -d)
            ln -s $out/share/purse-first/${name}/skills $staging/skills
            mkdir -p $staging/.claude-plugin
            ${
              if pluginBaseJson != null then
                "cp ${pluginBaseJson} $staging/.claude-plugin/plugin.json"
              else
                ''
                  cat > $staging/.claude-plugin/plugin.json <<PLUGIN_EOF
                  {
                    "name": "${name}",
                    "author": { "name": "${owner.name}" },
                    "description": "${description}"
                  }
                  PLUGIN_EOF
                ''
            }
            chmod u+w $staging/.claude-plugin/plugin.json
            purse-first generate-local-plugin --root $staging
            cp $staging/.claude-plugin/plugin.json $out/share/purse-first/${name}/plugin.json
          ''
      else
        null;

    # Write marketplace-config.json if pluginConfig is provided.
    configFile =
      if pluginConfig != null then
        pkgs.writeText "${name}-marketplace-config.json" (builtins.toJSON (
          {
            inherit name description;
            inherit owner;
          }
          // (if repo != null then { inherit repo; } else { })
          // (if pluginConfig ? plugins then { inherit (pluginConfig) plugins; } else { })
        ))
      else
        null;

    # All packages to join: plugins + meta plugin (if present).
    allPaths = pluginPkgs ++ (if metaPlugin != null then [ metaPlugin ] else [ ]);

    # Main marketplace derivation.
    marketplace = pkgs.symlinkJoin {
      name = "${name}-marketplace";
      paths = allPaths;
      nativeBuildInputs = [ pkgs.makeWrapper ];
      postBuild = ''
        makeWrapper ${cli}/bin/purse-first $out/bin/purse-first \
          --set PURSE_FIRST_PLUGINS_DIR "$out/share/purse-first"

        $out/bin/purse-first generate-marketplace \
          --plugins-dir "$out/share/purse-first" \
          ${if configFile != null then "--config ${configFile}" else ""} \
          --output "$out/.claude-plugin/marketplace.json"
      '';
    };

    # No-hooks variant.
    marketplace-no-hooks = pkgs.symlinkJoin {
      name = "${name}-marketplace-no-hooks";
      paths = allPaths;
      nativeBuildInputs = [
        pkgs.makeWrapper
        pkgs.jq
      ];
      postBuild = ''
        # Replace plugin.json symlinks with hook-stripped copies.
        for pj in $out/share/purse-first/*/plugin.json; do
          ${pkgs.jq}/bin/jq 'del(.hooks)' "$pj" > "$pj.tmp"
          rm "$pj"
          mv "$pj.tmp" "$pj"
        done

        # Remove hook script directories.
        for d in $out/share/purse-first/*/hooks; do
          [ -e "$d" ] && rm -rf "$d"
        done

        makeWrapper ${cli}/bin/purse-first $out/bin/purse-first \
          --set PURSE_FIRST_PLUGINS_DIR "$out/share/purse-first"

        $out/bin/purse-first generate-marketplace \
          --no-hooks \
          --plugins-dir "$out/share/purse-first" \
          ${if configFile != null then "--config ${configFile}" else ""} \
          --output "$out/.claude-plugin/marketplace.json"
      '';
    };
  in
  {
    packages = {
      default = marketplace;
      inherit marketplace-no-hooks;
    } // (if purse-first-build != null then { purse-first = cli; } else { });

    apps.default = {
      type = "app";
      program = "${marketplace}/bin/purse-first";
    };

    devShells.default = pkgs.mkShell {
      packages = [
        pkgs.just
      ] ++ (devShellPackages system pkgs pkgs-master);

      inputsFrom = devShellInputsFrom system;

      shellHook = devShellHook;
    };
  }
)
```

**Step 2: Verify the file parses**

Run: `nix-instantiate --parse lib/mkMarketplace.nix` (from repo root)
Expected: Successful parse, prints the AST

**Step 3: Commit**

```bash
git add lib/mkMarketplace.nix
git commit -m "Add lib/mkMarketplace.nix framework function"
```

---

### Task 2: Refactor `flake.nix` to Call `mkMarketplace`

**Files:**
- Modify: `flake.nix`

Replace the inline marketplace assembly with a call to `lib/mkMarketplace.nix`. Keep the same inputs, add framework exports (`lib`, `templates`). The `mcp-all` convenience package and per-plugin packages are kept as extra outputs alongside what `mkMarketplace` returns.

**Step 1: Write the refactored `flake.nix`**

The new `flake.nix` should:
1. Import `lib/mkMarketplace.nix` as `mkMarketplace`
2. Call `mkMarketplace` with purse-first's own configuration
3. Merge the mkMarketplace outputs with purse-first-specific extras (per-plugin packages, dev deps, `lib` export, `templates` export)
4. Keep the `go` overlay for `buildGoApplication` via `purse-first-build`

Key structure:

```nix
{
  description = "Claude Plugin Marketplace: MCP servers and tool routing for Claude Code";

  inputs = {
    # ... same inputs as before ...
  };

  outputs = { self, nixpkgs, nixpkgs-master, utils, go, shell, bats, sandcastle,
              grit, get-hubbed, lux, chix, batman, tap-dancer }:
    let
      mkMarketplace = import ./lib/mkMarketplace.nix;

      # Source filtering for Go build (exclude go.work and libs/)
      purse-first-src = nixpkgs.lib.cleanSourceWith {
        src = ./.;
        filter = path: type:
          let baseName = builtins.baseNameOf path;
          in baseName != "go.work" && baseName != "go.work.sum"
             && !nixpkgs.lib.hasPrefix (toString ./libs) path;
      };

      # get-hubbed needs gh on PATH — pass as a wrapped plugin
      wrapGetHubbed = system:
        let
          pkgs = import nixpkgs { inherit system; };
          upstream = get-hubbed.packages.${system}.default;
        in
        pkgs.runCommand "get-hubbed" { nativeBuildInputs = [ pkgs.makeWrapper ]; } ''
          mkdir -p $out/bin
          makeWrapper ${upstream}/bin/get-hubbed $out/bin/get-hubbed \
            --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.gh ]}
          if [ -d "${upstream}/share" ]; then
            cp -r ${upstream}/share $out/share
          fi
        '';

      # Call mkMarketplace for the purse-first marketplace itself
      marketplaceOutputs = mkMarketplace {
        inherit nixpkgs nixpkgs-master utils;

        name = "purse-first";
        owner = { name = "friedenberg"; email = "sasha@friedenberg.me"; };
        description = "MCP servers and tool routing for Claude Code, built with Nix";
        repo = "amarbel-llc/purse-first";

        purse-first-build = {
          src = purse-first-src;
          modules = ./gomod2nix.toml;
          version = "0.1.0";
          overlays = [ go.overlays.default ];
        };

        plugins = system: [
          grit.packages.${system}.default
          (wrapGetHubbed system)
          lux.packages.${system}.default
          chix.packages.${system}.default
          batman.packages.${system}.robin
          tap-dancer.packages.${system}.default
        ];

        skills = ./skills;
        pluginBaseJson = ./.claude-plugin/plugin.json;
        pluginConfig = builtins.fromJSON (builtins.readFile ./marketplace-config.json);

        devShellPackages = _system: pkgs: pkgs-master: [
          pkgs-master.claude-code
          pkgs.bats
          pkgs.bats.libraries.bats-support
          pkgs.bats.libraries.bats-assert
          (sandcastle.packages.${pkgs.system}.default)
        ];

        devShellInputsFrom = system: [
          go.devShells.${system}.default
          shell.devShells.${system}.default
          bats.devShells.${system}.default
        ];

        devShellHook = ''echo "purse-first - dev environment"'';
      };
    in
    # Merge marketplace outputs with framework exports + per-plugin extras
    marketplaceOutputs
    // {
      # Framework exports for downstream consumers
      lib.mkMarketplace = mkMarketplace;

      templates.marketplace = {
        path = ./templates/marketplace;
        description = "Bootstrap a new Claude plugin marketplace";
      };
    }
    // (utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        # Extra per-plugin packages (convenience, not from mkMarketplace)
        packages = (marketplaceOutputs.packages.${system} or {}) // {
          grit = grit.packages.${system}.default;
          get-hubbed = wrapGetHubbed system;
          lux = lux.packages.${system}.default;
          chix = chix.packages.${system}.default;
          tap-dancer = tap-dancer.packages.${system}.default;
          mcp-all = pkgs.symlinkJoin {
            name = "mcp-all";
            paths = [
              grit.packages.${system}.default
              (wrapGetHubbed system)
              lux.packages.${system}.default
              chix.packages.${system}.default
            ];
          };
        };

        # Preserve BATS_LIB_PATH in devShell
        devShells.default = (marketplaceOutputs.devShells.${system}.default or pkgs.mkShell {}).overrideAttrs (old: {
          BATS_LIB_PATH = "${pkgs.bats.libraries.bats-support}/share/bats:${pkgs.bats.libraries.bats-assert}/share/bats";
        });
      }
    ));
}
```

**Important considerations:**
- The `eachDefaultSystem` merging with per-system extras needs care — `mkMarketplace` already returns per-system outputs via `eachDefaultSystem`. The extras merge must not clobber them.
- `purse-first-src` filtering must happen at the top level (not inside `eachDefaultSystem`) since it's system-independent but needs `nixpkgs.lib`.
- `wrapGetHubbed` is system-dependent, called from the `plugins` function.
- The `devShells.default` merge with `BATS_LIB_PATH` may need adjustment — `overrideAttrs` on `mkShell` works but test it.

**Step 2: Verify the flake evaluates**

Run: `nix flake show --no-build`
Expected: Shows `packages`, `apps`, `devShells` for each system, plus `lib` and `templates`

**Step 3: Build the marketplace**

Run: `nix build --show-trace`
Expected: Successful build, `./result` contains the marketplace with all plugins

**Step 4: Verify marketplace contents match pre-refactor**

Run: `ls -la result/share/purse-first/` and `cat result/.claude-plugin/marketplace.json | jq .name`
Expected: Same plugins (grit, get-hubbed, lux, chix, robin, tap-dancer, bob) and `"purse-first"` name

**Step 5: Build no-hooks variant**

Run: `nix build .#marketplace-no-hooks --show-trace`
Expected: Successful build

**Step 6: Verify purse-first CLI package still builds**

Run: `nix build .#purse-first --show-trace`
Expected: Successful build

**Step 7: Verify per-plugin packages still build**

Run: `nix build .#grit && nix build .#lux && nix build .#chix`
Expected: All succeed

**Step 8: Verify devShell works**

Run: `nix develop --command just --list`
Expected: Shows justfile targets, BATS_LIB_PATH is set

**Step 9: Run existing tests**

Run: `just test`
Expected: All Go tests pass

**Step 10: Commit**

```bash
git add flake.nix
git commit -m "Refactor flake.nix to use lib/mkMarketplace"
```

---

### Task 3: Create Template — `flake.nix`

**Files:**
- Create: `templates/marketplace/flake.nix`

**Step 1: Write the template flake**

```nix
{
  description = "My Claude Plugin Marketplace";

  inputs = {
    purse-first.url = "github:amarbel-llc/purse-first";

    # Inherit nixpkgs from purse-first for consistency.
    nixpkgs.follows = "purse-first/nixpkgs";
    nixpkgs-master.follows = "purse-first/nixpkgs-master";
    utils.follows = "purse-first/utils";

    # --- Add your plugin inputs below ---
    # Each plugin should follow nixpkgs for consistent builds:
    #
    # my-plugin = {
    #   url = "github:my-org/my-plugin";
    #   inputs.nixpkgs.follows = "nixpkgs";
    #   inputs.nixpkgs-master.follows = "nixpkgs-master";
    # };
  };

  outputs =
    {
      purse-first,
      nixpkgs,
      nixpkgs-master,
      utils,
      ...
    }@inputs:
    purse-first.lib.mkMarketplace {
      inherit nixpkgs nixpkgs-master utils;

      # Provide the purse-first CLI from the flake input.
      purse-first-cli = purse-first;

      # --- Configure your marketplace ---
      name = "my-marketplace";
      owner = {
        name = "my-org";
        email = "team@my-org.com";
      };

      # List your plugins per system.
      plugins = system: [
        # inputs.my-plugin.packages.${system}.default
      ];

      # Optional: bundle skills (Markdown + YAML frontmatter).
      # skills = ./skills;

      # Optional: per-plugin metadata for the marketplace.
      # pluginConfig = builtins.fromJSON (builtins.readFile ./marketplace-config.json);
    };
}
```

**Step 2: Verify the template parses**

Run: `nix-instantiate --parse templates/marketplace/flake.nix`
Expected: Successful parse

**Step 3: Commit**

```bash
git add templates/marketplace/flake.nix
git commit -m "Add marketplace template flake.nix"
```

---

### Task 4: Create Template — Supporting Files

**Files:**
- Create: `templates/marketplace/justfile`
- Create: `templates/marketplace/.envrc`
- Create: `templates/marketplace/.gitignore`
- Create: `templates/marketplace/skills/.gitkeep`

**Step 1: Write `justfile`**

```make
# my-marketplace

default:
    @just --list

# Build the marketplace
build:
    nix build --show-trace

# Build without hooks
build-no-hooks:
    nix build .#marketplace-no-hooks --show-trace

# Install into Claude Code
install:
    nix run .#default -- install

# Check flake
check:
    nix flake check

# Update all flake inputs
update:
    nix flake update

# Update only plugin inputs (add your plugin names here)
update-plugins:
    @echo "Add your plugin input names to this target"
    # nix flake update my-plugin-a my-plugin-b

# Clean build artifacts
clean:
    rm -rf result result-*
```

**Step 2: Write `.envrc`**

```
# vim: ft=direnv
source_env "$HOME"
dotenv_if_exists ".env"
use flake .
dotenv_if_exists "$HOME/.env-dev"
```

**Step 3: Write `.gitignore`**

```
result
result-*
.direnv/
.env
```

**Step 4: Create `skills/.gitkeep`**

Empty file.

**Step 5: Commit**

```bash
git add templates/marketplace/justfile templates/marketplace/.envrc \
        templates/marketplace/.gitignore templates/marketplace/skills/.gitkeep
git commit -m "Add marketplace template supporting files"
```

---

### Task 5: Create Template — CI Workflow

**Files:**
- Create: `templates/marketplace/.github/workflows/ci.yml`

**Step 1: Write the CI workflow**

Based on purse-first's own `release.yml` pattern but simplified for downstream:

```yaml
name: CI

on:
  push:
    branches: [main, master]
  pull_request:

jobs:
  build:
    strategy:
      matrix:
        include:
          - os: ubuntu-latest
            system: x86_64-linux
          - os: macos-13
            system: x86_64-darwin
          - os: macos-latest
            system: aarch64-darwin

    runs-on: ${{ matrix.os }}

    steps:
      - uses: actions/checkout@v4

      - uses: DeterminateSystems/nix-installer-action@main

      - uses: DeterminateSystems/magic-nix-cache-action@main

      - name: Check flake
        run: nix flake check

      - name: Build marketplace
        run: nix build --show-trace

      - name: Build no-hooks variant
        run: nix build .#marketplace-no-hooks --show-trace

  # Uncomment to publish to FlakeHub on push to main/master.
  # Requires: Settings → Secrets → FLAKEHUB_TOKEN or OIDC id-token permissions.
  #
  # flakehub:
  #   needs: build
  #   if: github.ref == 'refs/heads/main' || github.ref == 'refs/heads/master'
  #   runs-on: ubuntu-latest
  #   permissions:
  #     id-token: write
  #     contents: read
  #
  #   steps:
  #     - uses: actions/checkout@v4
  #
  #     - uses: DeterminateSystems/nix-installer-action@main
  #
  #     - uses: DeterminateSystems/flakehub-push@main
  #       with:
  #         visibility: public
```

**Step 2: Commit**

```bash
git add templates/marketplace/.github/workflows/ci.yml
git commit -m "Add marketplace template CI workflow"
```

---

### Task 6: Integration Test — Template Produces a Working Marketplace

**Files:**
- Create: `zz-tests_bats/marketplace_template.bats`

This test verifies that the template can be initialized and builds successfully (with no plugins, which should still produce a valid empty marketplace).

**Step 1: Write the BATS test**

```bash
#!/usr/bin/env bats

load "${BATS_LIB_PATH}/bats-support/load.bash"
load "${BATS_LIB_PATH}/bats-assert/load.bash"

setup() {
  TEMPLATE_DIR="$(cd "$(dirname "$BATS_TEST_FILENAME")/.." && pwd)/templates/marketplace"
  TEST_DIR="$(mktemp -d)"
}

teardown() {
  rm -rf "$TEST_DIR"
}

@test "template flake.nix parses" {
  run nix-instantiate --parse "$TEMPLATE_DIR/flake.nix"
  assert_success
}

@test "template contains required files" {
  [ -f "$TEMPLATE_DIR/flake.nix" ]
  [ -f "$TEMPLATE_DIR/justfile" ]
  [ -f "$TEMPLATE_DIR/.envrc" ]
  [ -f "$TEMPLATE_DIR/.gitignore" ]
  [ -f "$TEMPLATE_DIR/.github/workflows/ci.yml" ]
}

@test "mkMarketplace.nix parses" {
  LIB_DIR="$(cd "$(dirname "$BATS_TEST_FILENAME")/.." && pwd)/lib"
  run nix-instantiate --parse "$LIB_DIR/mkMarketplace.nix"
  assert_success
}
```

**Step 2: Run the test**

Run: `nix develop --command bats --tap zz-tests_bats/marketplace_template.bats`
Expected: All tests pass

**Step 3: Commit**

```bash
git add zz-tests_bats/marketplace_template.bats
git commit -m "Add BATS tests for marketplace template"
```

---

### Task 7: Verify Full Build — End-to-End Smoke Test

No new files. This task verifies the complete refactored system works.

**Step 1: Build the default marketplace**

Run: `nix build --show-trace`
Expected: Successful build

**Step 2: Inspect marketplace.json**

Run: `cat result/.claude-plugin/marketplace.json | jq '.name, (.plugins | length)'`
Expected: `"purse-first"` and `7` (or current plugin count)

**Step 3: Verify all plugin binaries are present**

Run: `ls result/bin/`
Expected: `grit`, `get-hubbed`, `lux`, `purse-first` (and any others)

**Step 4: Verify skills are present**

Run: `ls result/share/purse-first/bob/skills/` (or `purse-first/skills/` if renamed)
Expected: `plugin-mcp/`, `context-saving/`, `go-cli-framework/`

**Step 5: Build no-hooks variant**

Run: `nix build .#marketplace-no-hooks --show-trace`
Expected: Successful build

**Step 6: Verify no-hooks variant has no hooks**

Run: `cat result/.claude-plugin/marketplace.json | jq '.plugins[].hooks // empty'`
Expected: No output (no hooks)

**Step 7: Run all tests**

Run: `just test-all`
Expected: All tests pass

**Step 8: Verify flake show includes lib and templates**

Run: `nix flake show --no-build 2>&1 | head -30`
Expected: Output includes `lib` and `templates.marketplace`

**Step 9: Commit (if any fixes were needed)**

```bash
git add -A
git commit -m "Fix marketplace framework integration issues"
```

---

### Task 8: Update Justfile with Framework Targets

**Files:**
- Modify: `justfile`

**Step 1: Add framework-specific targets**

Add these targets to the existing justfile:

```make
# Verify mkMarketplace.nix parses
check-lib:
    nix-instantiate --parse lib/mkMarketplace.nix

# Verify template parses
check-template:
    nix-instantiate --parse templates/marketplace/flake.nix

# Run template tests
test-template:
    nix develop --command bats --tap zz-tests_bats/marketplace_template.bats
```

**Step 2: Update `test-all` to include template tests**

Add `test-template` to the `test-all` dependency list.

**Step 3: Commit**

```bash
git add justfile
git commit -m "Add framework targets to justfile"
```

---

## Implementation Notes

### Merging `eachDefaultSystem` Outputs

The trickiest part is merging `mkMarketplace`'s per-system outputs with purse-first's extra per-system outputs (per-plugin packages, BATS_LIB_PATH). The `//` operator on attrsets with nested keys like `packages.x86_64-linux` requires recursive merging or separate `eachDefaultSystem` calls that are merged. Test this carefully — a shallow `//` will clobber the inner packages attrset.

Use `nixpkgs.lib.recursiveUpdate` if `//` doesn't merge deeply enough:

```nix
nixpkgs.lib.recursiveUpdate marketplaceOutputs (utils.lib.eachDefaultSystem ...)
```

### `purse-first-cli` vs `purse-first-build`

Two modes for obtaining the CLI:
- **Self-build** (`purse-first-build`): Used by purse-first itself. Passes `{ src, modules, version, overlays }` and mkMarketplace builds the Go binary.
- **Downstream** (`purse-first-cli`): Used by template consumers. Passes the purse-first flake input directly, CLI is extracted from `purse-first-cli.packages.${system}.purse-first`.

### `generate-marketplace` Without `--config`

The Go CLI's `generate-marketplace` command currently requires `--config`. If downstream marketplaces don't provide `pluginConfig`, mkMarketplace will omit the `--config` flag. Verify the CLI handles missing `--config` gracefully. If not, either:
- Generate a minimal config from `name`/`owner`/`description` and always pass it
- Or update the Go code to make `--config` optional

Check `cmd/purse-first/main.go` around line 109 for the flag definition and whether it's marked required.

### Template `nix flake init`

For `nix flake init -t purse-first#marketplace` to work, the `templates` attribute must be a top-level flake output (not inside `eachDefaultSystem`). The current design exports it at the top level alongside `lib`, which is correct.
