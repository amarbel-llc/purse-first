# Bob Extraction Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Extract all packages and general-purpose skills from purse-first into `~/eng/repos/bob`, making purse-first framework-only.

**Architecture:** Bob becomes a standalone purse-first package using `mkMarketplace` as a flake input. Go packages use published module versions of `libs/go-mcp`. Rust packages use git dependencies for `libs/rust-mcp`. Both repos build and test independently.

**Tech Stack:** Nix flakes, Go workspace, Cargo workspace, BATS

**Rollback:** Revert purse-first's flake.nix to pre-extraction commit. Bob repo is inert until integrated.

---

### Task 1: Tag libs/go-mcp at current HEAD

Bob's Go modules need a published version of `libs/go-mcp` to depend on (no
more `replace` directives). Tag the current HEAD so Go modules can reference a
real version.

**Files:**
- None (git tag only)

**Step 1: Tag libs/go-mcp**

```bash
git tag libs/go-mcp/v0.0.3
git tag libs/go-mcp/command/huh/v0.0.3
```

**Step 2: Push tags**

```bash
git push origin libs/go-mcp/v0.0.3 libs/go-mcp/command/huh/v0.0.3
```

**Step 3: Verify Go proxy picks up the tag**

```bash
GOPROXY=https://proxy.golang.org go list -m github.com/amarbel-llc/purse-first/libs/go-mcp@v0.0.3
```

Expected: resolves to v0.0.3

---

### Task 2: Scaffold bob repo structure

Set up the bob repo with the directory layout, flake.nix, go.work, Cargo.toml,
and package.toml. No code yet — just the skeleton.

**Files:**
- Create: `~/eng/repos/bob/flake.nix`
- Create: `~/eng/repos/bob/go.work`
- Create: `~/eng/repos/bob/Cargo.toml`
- Create: `~/eng/repos/bob/Cargo.lock`
- Create: `~/eng/repos/bob/package.toml`
- Create: `~/eng/repos/bob/marketplace-config.json`
- Create: `~/eng/repos/bob/CLAUDE.md`
- Create: `~/eng/repos/bob/.envrc`
- Create: `~/eng/repos/bob/.gitignore`

**Step 1: Create directory structure**

```bash
mkdir -p ~/eng/repos/bob/{packages,skills,lib/packages,devenvs,dummies,zz-tests_bats}
```

**Step 2: Create .gitignore**

```gitignore
result
result-*
build/
vendor/
```

**Step 3: Create .envrc**

```bash
use flake
```

**Step 4: Create package.toml**

```toml
name = "bob"
description = "MCP servers, CLI tools, and development workflow skills"

[author]
name = "friedenberg"
```

**Step 5: Create marketplace-config.json**

Copy from purse-first's `marketplace-config.json` but update:
- `name` → `"bob"`
- `repo` → `"amarbel-llc/bob"`
- Remove the `"bob"` plugin entry (it's now the marketplace itself)
- Update all `homepage` URLs to `"https://github.com/amarbel-llc/bob"`

**Step 6: Create go.work**

```go
go 1.25.6

use (
	./dummies/go
	./packages/get-hubbed
	./packages/grit
	./packages/lux
	./packages/mgp
	./packages/potato
	./packages/spinclass
	./packages/tap-dancer/go
)
```

**Step 7: Create Cargo.toml**

```toml
[workspace]
members = [
    "packages/chix",
    "packages/tap-dancer/rust",
]
resolver = "2"
```

**Step 8: Create flake.nix**

```nix
{
  description = "MCP servers, CLI tools, and development workflow skills";

  inputs = {
    purse-first.url = "github:amarbel-llc/purse-first";

    # Re-declare inputs mkMarketplace needs.
    # Follow purse-first's pins for consistency.
    nixpkgs.follows = "purse-first/nixpkgs";
    nixpkgs-master.follows = "purse-first/nixpkgs-master";
    utils.follows = "purse-first/utils";

    # Build tooling
    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    crane.follows = "purse-first/crane";
    rust-overlay = {
      url = "github:oxalica/rust-overlay";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    fh.url = "https://flakehub.com/f/DeterminateSystems/fh/*.tar.gz";
  };

  outputs =
    { self
    , purse-first
    , nixpkgs
    , nixpkgs-master
    , utils
    , gomod2nix
    , crane
    , rust-overlay
    , fh
    ,
    }:
    let
      mkMarketplace = purse-first.lib.mkMarketplace;

      goWorkspaceSrc = nixpkgs.lib.cleanSourceWith {
        src = ./.;
        filter =
          path: type:
          let
            baseName = builtins.baseNameOf path;
          in
          type == "directory"
          || nixpkgs.lib.hasSuffix ".go" baseName
          || baseName == "go.mod"
          || baseName == "go.sum"
          || baseName == "go.work"
          || baseName == "go.work.sum"
          || nixpkgs.lib.hasSuffix ".scd" baseName;
      };

      # Computed after first `go work vendor` — placeholder until then.
      goVendorHash = nixpkgs.lib.fakeHash;

      buildDevenvs =
        system:
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

      buildPackages =
        system:
        let
          pkgs = import nixpkgs { inherit system; };
          pkgs-master = import nixpkgs-master { inherit system; };
          pkgs-overlay = import nixpkgs {
            inherit system;
            overlays = [ (import rust-overlay) ];
          };
          craneLib = (crane.mkLib pkgs).overrideToolchain (pkgs-overlay.rust-bin.stable.latest.default);
          rustWorkspaceSrc = craneLib.cleanCargoSource ./.;
          rustCommonArgs = {
            src = rustWorkspaceSrc;
            pname = "rust-workspace-deps";
            version = "0.1.0";
            strictDeps = true;
          };
          rustCargoArtifacts = craneLib.buildDepsOnly rustCommonArgs;
          fhPkg = fh.packages.${system}.default;
          purse-first-cli = purse-first.packages.${system}.purse-first;

          mkGoModule = import ./lib/mkGoWorkspaceModule.nix {
            inherit pkgs goWorkspaceSrc goVendorHash;
          };

          sandcastlePkg = import ./lib/packages/sandcastle.nix {
            inherit pkgs;
            src = ./packages/sandcastle;
          };

          andSoCanYouRepoPkg = import ./lib/packages/and-so-can-you-repo.nix {
            inherit pkgs;
            src = ./packages/and-so-can-you-repo;
          };

          potatoPkg = import ./lib/packages/potato.nix {
            inherit pkgs goWorkspaceSrc goVendorHash;
          };

          spinclassPkg = import ./lib/packages/spinclass.nix {
            inherit pkgs goWorkspaceSrc goVendorHash;
            src = ./packages/spinclass;
          };

          gritPkg = import ./lib/packages/grit.nix {
            inherit pkgs goWorkspaceSrc goVendorHash;
          };

          get-hubbed-unwrapped = import ./lib/packages/get-hubbed.nix {
            inherit pkgs goWorkspaceSrc goVendorHash;
          };

          get-hubbed-wrapped =
            pkgs.runCommand "get-hubbed"
              {
                nativeBuildInputs = [ pkgs.makeWrapper ];
              }
              ''
                mkdir -p $out/bin
                makeWrapper ${get-hubbed-unwrapped}/bin/get-hubbed $out/bin/get-hubbed \
                  --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.gh ]}
                if [ -d "${get-hubbed-unwrapped}/share" ]; then
                  cp -r ${get-hubbed-unwrapped}/share $out/share
                fi
              '';

          luxPkg = import ./lib/packages/lux.nix {
            inherit pkgs goWorkspaceSrc goVendorHash;
          };

          mgpPkg = import ./lib/packages/mgp.nix {
            inherit pkgs goWorkspaceSrc goVendorHash;
          };

          chixPkg = import ./lib/packages/chix.nix {
            inherit
              pkgs
              craneLib
              fhPkg
              rustWorkspaceSrc
              rustCargoArtifacts
              ;
            src = ./packages/chix;
          };

          tapDancerPkgs = import ./lib/packages/tap-dancer.nix {
            inherit
              pkgs
              craneLib
              purse-first-cli
              goWorkspaceSrc
              goVendorHash
              rustWorkspaceSrc
              rustCargoArtifacts
              ;
            src = ./packages/tap-dancer;
          };

          batmanPkgs = import ./lib/packages/batman.nix {
            inherit pkgs purse-first-cli;
            sandcastle = sandcastlePkg;
            tap-dancer-cli = tapDancerPkgs.cli;
            src = ./packages/batman;
          };
        in
        {
          inherit
            gritPkg
            get-hubbed-wrapped
            luxPkg
            mgpPkg
            chixPkg
            tapDancerPkgs
            batmanPkgs
            sandcastlePkg
            andSoCanYouRepoPkg
            potatoPkg
            spinclassPkg
            ;
        };

      marketplaceOutputs = mkMarketplace {
        inherit nixpkgs nixpkgs-master utils;
        name = "bob";
        owner = {
          name = "friedenberg";
          email = "sasha@friedenberg.me";
        };
        description = "MCP servers, CLI tools, and development workflow skills";
        repo = "amarbel-llc/bob";
        purse-first-cli = purse-first.packages;
        plugins =
          system:
          let
            pkgs = buildPackages system;
          in
          [
            pkgs.gritPkg
            pkgs.luxPkg
            pkgs.mgpPkg
            pkgs.chixPkg
            pkgs.get-hubbed-wrapped
            pkgs.batmanPkgs.robin
            pkgs.tapDancerPkgs.default
          ];
        skills = ./skills;
        packageToml = ./package.toml;
        pluginConfig = builtins.fromJSON (builtins.readFile ./marketplace-config.json);
        devShellPackages =
          system: pkgs: pkgs-master:
          let
            localPkgs = buildPackages system;
          in
          [
            pkgs-master.claude-code
            localPkgs.batmanPkgs.default
          ];
        devShellInputsFrom =
          system:
          let
            devenvs = buildDevenvs system;
          in
          [
            devenvs.go.devShell
            devenvs.shell.devShell
            devenvs.bats.devShell
            devenvs.rust.devShell
          ];
        devShellHook = ''
          echo "bob - dev environment"
        '';
      };
    in
    nixpkgs.lib.recursiveUpdate marketplaceOutputs (
      utils.lib.eachDefaultSystem (
        system:
        let
          pkgs = import nixpkgs { inherit system; };
          localPkgs = buildPackages system;
          devenvs = buildDevenvs system;
        in
        {
          packages =
            let
              marketplacePkgs = marketplaceOutputs.packages.${system} or { };
              nonPluginPkgs = [
                localPkgs.sandcastlePkg
                localPkgs.andSoCanYouRepoPkg
                localPkgs.potatoPkg
                localPkgs.spinclassPkg
              ];
            in
            marketplacePkgs
            // {
              default = pkgs.symlinkJoin {
                name = "bob-all";
                paths = [ marketplacePkgs.default ] ++ nonPluginPkgs;
              };
              grit = localPkgs.gritPkg;
              get-hubbed = localPkgs.get-hubbed-wrapped;
              lux = localPkgs.luxPkg;
              mgp = localPkgs.mgpPkg;
              chix = localPkgs.chixPkg;
              robin = localPkgs.batmanPkgs.robin;
              batman = localPkgs.batmanPkgs.default;
              tap-dancer = localPkgs.tapDancerPkgs.default;
              sandcastle = localPkgs.sandcastlePkg;
              and-so-can-you-repo = localPkgs.andSoCanYouRepoPkg;
              potato = localPkgs.potatoPkg;
              spinclass = localPkgs.spinclassPkg;
              mcp-all = pkgs.symlinkJoin {
                name = "mcp-all";
                paths = [
                  localPkgs.gritPkg
                  localPkgs.get-hubbed-wrapped
                  localPkgs.luxPkg
                  localPkgs.mgpPkg
                  localPkgs.chixPkg
                ];
              };
            };

          devShells = {
            default = marketplaceOutputs.devShells.${system}.default;
            go = devenvs.go.devShell;
            shell = devenvs.shell.devShell;
            bats = devenvs.bats.devShell;
            rust = devenvs.rust.devShell;
          };
        }
      )
    );
}
```

**Step 9: Create initial CLAUDE.md**

Adapt from purse-first's CLAUDE.md — remove framework-specific sections, keep
package build/test commands, update paths.

**Step 10: Commit**

```bash
git add -A
git commit -m "feat: scaffold bob repo structure"
```

---

### Task 3: Copy packages to bob

Copy all 11 `packages/*` directories from purse-first to bob. No modifications
yet — just raw copies.

**Files:**
- Create: `~/eng/repos/bob/packages/{grit,get-hubbed,lux,mgp,chix,tap-dancer,batman,sandcastle,and-so-can-you-repo,potato,spinclass}/`

**Step 1: Copy package directories**

```bash
cp -r packages/{grit,get-hubbed,lux,mgp,chix,tap-dancer,batman,sandcastle,and-so-can-you-repo,potato,spinclass} ~/eng/repos/bob/packages/
```

Source: purse-first worktree `packages/` directory.

**Step 2: Commit**

```bash
git add packages/
git commit -m "feat: copy all packages from purse-first"
```

---

### Task 4: Copy skills to bob

Copy the 22 general-purpose skills from purse-first to bob.

**Files:**
- Create: `~/eng/repos/bob/skills/{brainstorming,commit,writing-plans,test-driven-development,systematic-debugging,verification-before-completion,requesting-code-review,receiving-code-review,using-git-worktrees,executing-plans,subagent-driven-development,finishing-a-development-branch,dispatching-parallel-agents,adr,fdr,rfc,using-superpowers,writing-skills,freud,voldemort,minus-chorevaults,design_patterns-just}/`

**Step 1: Copy skill directories**

```bash
for skill in brainstorming commit writing-plans test-driven-development systematic-debugging \
  verification-before-completion requesting-code-review receiving-code-review \
  using-git-worktrees executing-plans subagent-driven-development \
  finishing-a-development-branch dispatching-parallel-agents adr fdr rfc \
  using-superpowers writing-skills freud voldemort minus-chorevaults \
  design_patterns-just; do
  cp -r skills/"$skill" ~/eng/repos/bob/skills/
done
```

Source: purse-first worktree `skills/` directory.

**Step 2: Commit**

```bash
git add skills/
git commit -m "feat: copy general-purpose skills from purse-first"
```

---

### Task 5: Copy build infrastructure to bob

Copy Nix build expressions, devenvs, dummies, and mkGoWorkspaceModule.

**Files:**
- Create: `~/eng/repos/bob/lib/packages/*.nix`
- Create: `~/eng/repos/bob/lib/mkGoWorkspaceModule.nix`
- Create: `~/eng/repos/bob/devenvs/{go,rust,bats,shell}/`
- Create: `~/eng/repos/bob/dummies/go/`

**Step 1: Copy lib/packages/*.nix**

```bash
cp lib/packages/{grit,get-hubbed,lux,mgp,chix,tap-dancer,batman,sandcastle,and-so-can-you-repo,potato,spinclass}.nix ~/eng/repos/bob/lib/packages/
```

**Step 2: Copy mkGoWorkspaceModule.nix**

```bash
cp lib/mkGoWorkspaceModule.nix ~/eng/repos/bob/lib/
```

**Step 3: Copy devenvs**

```bash
cp -r devenvs/{go,rust,bats,shell} ~/eng/repos/bob/devenvs/
```

**Step 4: Copy dummies**

```bash
cp -r dummies/go ~/eng/repos/bob/dummies/
```

**Step 5: Commit**

```bash
git add lib/ devenvs/ dummies/
git commit -m "feat: copy Nix build expressions and devenvs from purse-first"
```

---

### Task 6: Copy BATS tests to bob

Copy package-specific BATS tests and the shared common.bash.

Note: `package_brew.bats`, `homebrew_tap.bats`, `brew_hashes.bats`,
`brew_tarball.bats` are framework-level (test purse-first CLI) and stay.

**Files:**
- Create: `~/eng/repos/bob/zz-tests_bats/common.bash`
- Create: `~/eng/repos/bob/zz-tests_bats/validate_plugin_repos.bats`
- Create: `~/eng/repos/bob/zz-tests_bats/lux_service.bats`

**Step 1: Copy test files**

```bash
cp zz-tests_bats/common.bash ~/eng/repos/bob/zz-tests_bats/
cp zz-tests_bats/validate_plugin_repos.bats ~/eng/repos/bob/zz-tests_bats/
cp zz-tests_bats/lux_service.bats ~/eng/repos/bob/zz-tests_bats/
```

**Step 2: Commit**

```bash
git add zz-tests_bats/
git commit -m "feat: copy package-specific BATS tests from purse-first"
```

---

### Task 7: Update Go modules in bob — remove replace directives

All Go packages switch from local `replace` directives to published module
versions. Also update `dummies/go`.

**Files:**
- Modify: `~/eng/repos/bob/packages/grit/go.mod`
- Modify: `~/eng/repos/bob/packages/get-hubbed/go.mod`
- Modify: `~/eng/repos/bob/packages/lux/go.mod`
- Modify: `~/eng/repos/bob/packages/mgp/go.mod`
- Modify: `~/eng/repos/bob/packages/tap-dancer/go/go.mod`
- Modify: `~/eng/repos/bob/packages/spinclass/go.mod`
- Modify: `~/eng/repos/bob/dummies/go/go.mod`

**Step 1: Update each go.mod**

For each Go module that has a `replace` directive pointing to `../../libs/go-mcp`:

1. Remove the `replace github.com/amarbel-llc/purse-first/libs/go-mcp => ...` line
2. Update the `require` version to `v0.0.3` (the tag from Task 1)

For spinclass, also remove the `replace` for `tap-dancer/go` and update to use
the bob-local path (since tap-dancer is in the same workspace).

Example for `packages/grit/go.mod`:

```go
module github.com/friedenberg/grit

go 1.25.6

require github.com/amarbel-llc/purse-first/libs/go-mcp v0.0.3

require mvdan.cc/sh/v3 v3.12.0 // indirect
```

**Step 2: Run go work sync to update go.work.sum**

```bash
cd ~/eng/repos/bob && nix develop --command go work sync
```

**Step 3: Vendor dependencies**

```bash
cd ~/eng/repos/bob && nix develop --command go work vendor
```

**Step 4: Verify Go builds**

```bash
cd ~/eng/repos/bob && nix develop --command go build ./packages/grit/...
cd ~/eng/repos/bob && nix develop --command go build ./packages/get-hubbed/...
cd ~/eng/repos/bob && nix develop --command go build ./packages/lux/...
cd ~/eng/repos/bob && nix develop --command go build ./packages/mgp/...
```

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: update Go modules to use published go-mcp dependency"
```

---

### Task 8: Update Rust dependencies in bob

chix and tap-dancer/rust switch from path to git dependencies for `mcp-server`.

**Files:**
- Modify: `~/eng/repos/bob/packages/chix/Cargo.toml`
- Modify: `~/eng/repos/bob/packages/tap-dancer/rust/Cargo.toml` (if it exists)
- Modify: `~/eng/repos/bob/Cargo.lock`

**Step 1: Update chix Cargo.toml**

Change:
```toml
mcp-server = { path = "../../libs/rust-mcp", features = ["tools", "resources", "command"] }
```
To:
```toml
mcp-server = { git = "https://github.com/amarbel-llc/purse-first", path = "libs/rust-mcp", features = ["tools", "resources", "command"] }
```

**Step 2: Update tap-dancer/rust Cargo.toml similarly**

Check if it depends on `mcp-server` via path and update the same way.

**Step 3: Update Cargo.lock**

```bash
cd ~/eng/repos/bob && nix develop --command cargo update
```

**Step 4: Verify Rust builds**

```bash
cd ~/eng/repos/bob && nix develop --command cargo check --workspace
```

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: update Rust deps to use git dependency for mcp-server"
```

---

### Task 9: Compute goVendorHash and verify Nix build

Update `flake.nix` with the real `goVendorHash` and build.

**Files:**
- Modify: `~/eng/repos/bob/flake.nix`

**Step 1: Attempt build to get hash mismatch**

```bash
cd ~/eng/repos/bob && nix build 2>&1 | grep 'got:'
```

Extract the `sha256-...` hash from the error.

**Step 2: Update goVendorHash in flake.nix**

Replace `nixpkgs.lib.fakeHash` with the real hash.

**Step 3: Build again**

```bash
cd ~/eng/repos/bob && nix build
```

Expected: successful build producing `result/` symlink.

**Step 4: Verify marketplace output**

```bash
ls result/share/purse-first/
ls result/.claude-plugin/marketplace.json
jq '.plugins | length' result/.claude-plugin/marketplace.json
```

Expected: all packages present, marketplace.json has correct plugin count.

**Step 5: Commit**

```bash
git add flake.nix
git commit -m "feat: set goVendorHash for bob workspace"
```

---

### Task 10: Run bob tests

Verify Go tests, Rust tests, and BATS integration tests pass in bob.

**Files:**
- None (verification only)

**Step 1: Run Go tests**

```bash
cd ~/eng/repos/bob && nix develop --command go test ./packages/grit/...
cd ~/eng/repos/bob && nix develop --command go test ./packages/get-hubbed/...
cd ~/eng/repos/bob && nix develop --command go test ./packages/lux/...
cd ~/eng/repos/bob && nix develop --command go test ./packages/mgp/...
```

**Step 2: Run Rust tests**

```bash
cd ~/eng/repos/bob && nix develop --command cargo test --workspace
```

**Step 3: Run BATS tests**

```bash
cd ~/eng/repos/bob && nix develop --command bats --tap zz-tests_bats/validate_plugin_repos.bats
```

**Step 4: Run nix flake check**

```bash
cd ~/eng/repos/bob && nix flake check
```

---

### Task 11: Slim down purse-first — remove packages

Remove all package directories, build expressions, and package-specific tests
from purse-first. Update flake.nix, go.work, Cargo.toml.

**Files:**
- Delete: `packages/` (all 11 directories)
- Delete: `lib/packages/*.nix` (all 11 package build expressions)
- Delete: `dummies/go/`
- Delete: `zz-tests_bats/validate_plugin_repos.bats`
- Delete: `zz-tests_bats/lux_service.bats`
- Modify: `flake.nix` — remove all package build logic
- Modify: `go.work` — remove package modules
- Modify: `Cargo.toml` — remove chix and tap-dancer
- Modify: `marketplace-config.json` — remove all plugin entries except purse-first's own

**Step 1: Delete package directories**

```bash
rm -rf packages/ dummies/go/ lib/packages/
rm zz-tests_bats/validate_plugin_repos.bats zz-tests_bats/lux_service.bats
```

**Step 2: Update go.work**

```go
go 1.25.6

use (
	.
	./libs/go-mcp
	./libs/go-mcp/command/huh
)
```

**Step 3: Update Cargo.toml**

```toml
[workspace]
members = ["libs/rust-mcp"]
resolver = "2"
```

**Step 4: Update flake.nix**

Remove `buildPackages`, all package imports, the `goWorkspaceSrc` filter for
`.scd` files. The flake now only builds `purse-first` CLI, exposes
`lib.mkMarketplace`, and publishes framework skills as package "purse-first".

The `plugins` argument to mkMarketplace becomes an empty list (or is omitted).
`skills = ./skills;` points to the remaining 8 framework skills.

**Step 5: Update package.toml**

```toml
name = "purse-first"
description = "Package framework for bundling CLIs, MCP servers, and skills"

[author]
name = "friedenberg"
```

**Step 6: Update marketplace-config.json**

Remove all plugin entries. Keep only framework metadata. The "purse-first"
skill package is auto-generated from the `skills` parameter.

**Step 7: Update .claude-plugin/plugin.json**

List only the 8 remaining framework skills:

```json
{
  "name": "purse-first",
  "skills": [
    "./skills/overview",
    "./skills/creating-packages",
    "./skills/using-packages",
    "./skills/go-cli-framework",
    "./skills/context-saving",
    "./skills/mcp",
    "./skills/claude-plugins",
    "./skills/design_patterns-downstream_rust"
  ]
}
```

**Step 8: Delete moved skills**

```bash
for skill in brainstorming commit writing-plans test-driven-development systematic-debugging \
  verification-before-completion requesting-code-review receiving-code-review \
  using-git-worktrees executing-plans subagent-driven-development \
  finishing-a-development-branch dispatching-parallel-agents adr fdr rfc \
  using-superpowers writing-skills freud voldemort minus-chorevaults \
  design_patterns-just; do
  rm -rf skills/"$skill"
done
```

**Step 9: Remove devenvs that are no longer needed**

purse-first no longer builds Rust or BATS packages, but keep bats for framework
tests and rust for libs/rust-mcp development.

**Step 10: Verify purse-first still builds**

```bash
nix build
nix flake check
```

**Step 11: Commit**

```bash
git add -A
git commit -m "refactor: remove packages and general skills, now in bob repo"
```

---

### Task 12: Update purse-first BATS tests

Update remaining BATS tests to reflect the slimmed-down marketplace.

**Files:**
- Modify: `zz-tests_bats/validate_marketplace.bats`

**Step 1: Update plugin_names_match_config test**

The `plugin_names_match_config` test currently asserts:
```
assert_output "bob,chix,get-hubbed,grit,lux,mgp,robin,tap-dancer"
```

Update to reflect only the purse-first framework skill package (if any plugins
remain) or remove the test if the marketplace has no plugins.

**Step 2: Run BATS tests**

```bash
nix develop --command bats --tap zz-tests_bats/
```

**Step 3: Commit**

```bash
git add zz-tests_bats/
git commit -m "fix: update BATS tests for framework-only marketplace"
```

---

### Task 13: Write bob's CLAUDE.md

Write a proper CLAUDE.md for the bob repo with build commands, architecture,
conventions.

**Files:**
- Modify: `~/eng/repos/bob/CLAUDE.md`

**Step 1: Write CLAUDE.md**

Cover:
- Overview (what bob is)
- Build & test commands (just build, just test, per-package commands)
- Go workspace layout
- Rust workspace layout
- Skill structure
- Nix conventions (stable-first nixpkgs, mkMarketplace)
- Code style
- GPG signing requirement

**Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add CLAUDE.md for bob repo"
```

---

### Task 14: Create bob justfile

Bob needs its own justfile with build, test, format, vendor, and per-package
targets.

**Files:**
- Create: `~/eng/repos/bob/justfile`

**Step 1: Write justfile**

Model after purse-first's justfile. Include:
- `build`, `test`, `fmt`, `lint`, `validate`
- `test-go`, `test-grit`, `test-lux`, `test-get-hubbed`, `test-go-mcp`, `test-chix`
- `test-integration`
- `vendor`, `vendor-hash`, `deps`
- Per-package build targets

**Step 2: Verify key targets**

```bash
cd ~/eng/repos/bob && just build
cd ~/eng/repos/bob && just test
```

**Step 3: Commit**

```bash
git add justfile
git commit -m "feat: add justfile with build and test targets"
```

---

### Task 15: Update design_patterns-just skill references

The `design_patterns-just` skill references purse-first-specific patterns.
Update to be self-contained or reference bob's own justfile.

**Files:**
- Modify: `~/eng/repos/bob/skills/design_patterns-just/SKILL.md`

**Step 1: Read current content and update references**

Replace purse-first-specific examples with generic or bob-specific examples.

**Step 2: Commit**

```bash
git add skills/design_patterns-just/
git commit -m "docs: update design_patterns-just skill to be self-contained"
```

---

### Task 16: Verify end-to-end

Full verification of both repos.

**Files:**
- None (verification only)

**Step 1: Build bob**

```bash
cd ~/eng/repos/bob && nix build
```

**Step 2: Verify bob marketplace output**

```bash
jq . ~/eng/repos/bob/result/.claude-plugin/marketplace.json
ls ~/eng/repos/bob/result/share/purse-first/
ls ~/eng/repos/bob/result/bin/
```

**Step 3: Build purse-first**

```bash
cd /path/to/purse-first && nix build
```

**Step 4: Verify purse-first marketplace output**

```bash
jq . result/.claude-plugin/marketplace.json
ls result/share/purse-first/
```

**Step 5: Run all bob tests**

```bash
cd ~/eng/repos/bob && just test
```

**Step 6: Run all purse-first tests**

```bash
just test
```
