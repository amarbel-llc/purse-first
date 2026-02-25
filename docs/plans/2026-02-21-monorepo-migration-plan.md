# Monorepo Migration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Migrate all 6 external packages (grit, get-hubbed, lux, chix, batman, tap-dancer) into the purse-first monorepo under `packages/`, eliminating circular dependencies.

**Architecture:** Single flake.nix builds all packages from local sources. Go workspace (`go.work`) links all Go modules. Rust packages use path dependencies to `libs/rust-mcp`. Per-package Nix expressions in `lib/packages/` handle individual builds.

**Tech Stack:** Nix (flakes, buildGoApplication, crane), Go (workspace), Rust (cargo, crane), Bash

---

### Task 1: Copy Package Sources

**Files:**
- Create: `packages/grit/` (from `/home/sasha/eng/repos/grit/`)
- Create: `packages/get-hubbed/` (from `/home/sasha/eng/repos/get-hubbed/`)
- Create: `packages/lux/` (from `/home/sasha/eng/repos/lux/`)
- Create: `packages/chix/` (from `/home/sasha/eng/repos/chix/`)
- Create: `packages/batman/` (from `/home/sasha/eng/repos/batman/`)
- Create: `packages/tap-dancer/` (from `/home/sasha/eng/repos/tap-dancer/`)

**Step 1: Create packages directory and copy sources**

For each package, copy source files excluding `.git/`, `target/`, `result/`, `flake.lock`, and `flake.nix` (those Nix files will be replaced by `lib/packages/*.nix`). Keep `.envrc`, `justfile`, `go.mod`, `go.sum`, `gomod2nix.toml`, `Cargo.toml`, `Cargo.lock`, `.claude-plugin/`, `skills/`, `docs/`, `zz-tests_bats/`, and all source code.

```bash
mkdir -p packages

for repo in grit get-hubbed lux chix batman tap-dancer; do
  rsync -a --exclude='.git' --exclude='target' --exclude='result' \
    "$HOME/eng/repos/$repo/" "packages/$repo/"
done
```

**Step 2: Remove per-package flake.nix and flake.lock files**

These are replaced by `lib/packages/*.nix` and the top-level flake.

```bash
for repo in grit get-hubbed lux chix batman tap-dancer; do
  rm -f "packages/$repo/flake.nix" "packages/$repo/flake.lock"
done
```

**Step 3: Remove get-hubbed's deps/ directory**

get-hubbed currently copies purse-first sources into `deps/purse-first` at build time. This is no longer needed.

```bash
rm -rf packages/get-hubbed/deps
```

**Step 4: Commit**

```bash
git add packages/
git commit -m "$(cat <<'EOF'
feat: copy package sources into monorepo

Copy grit, get-hubbed, lux, chix, batman, and tap-dancer source
files into packages/ directory. Nix build files are handled
separately.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Update Go Workspace

**Files:**
- Modify: `go.work`

**Step 1: Update go.work to include all Go modules**

```go
go 1.25.6

use (
	.
	./libs/go-mcp
	./libs/go-mcp/command/huh
	./packages/grit
	./packages/get-hubbed
	./packages/lux
	./packages/tap-dancer/go
)
```

**Step 2: Verify workspace resolves**

Run: `nix develop --command go work sync`
Expected: No errors. Local modules resolve against each other.

**Step 3: Verify each Go package builds**

Run: `nix develop --command go build ./packages/grit/... ./packages/get-hubbed/... ./packages/lux/... ./packages/tap-dancer/go/...`
Expected: Clean build, no errors.

**Step 4: Update get-hubbed go.mod**

get-hubbed currently has a `replace` directive pointing to `./deps/purse-first`. Remove it — the go.work workspace handles resolution now.

In `packages/get-hubbed/go.mod`, remove the line:
```
replace github.com/amarbel-llc/purse-first => ./deps/purse-first
```

Also update the require to reference the purse module correctly. get-hubbed currently imports `github.com/amarbel-llc/purse-first` (the main module) for the `purse` package. This should be updated to import `github.com/amarbel-llc/purse-first/purse` if needed, or the go.work will resolve it.

**Step 5: Run tests for each Go package**

```bash
nix develop --command go test ./packages/grit/...
nix develop --command go test ./packages/get-hubbed/...
nix develop --command go test ./packages/lux/...
nix develop --command go test ./packages/tap-dancer/go/...
```

**Step 6: Commit**

```bash
git add go.work packages/get-hubbed/go.mod
git commit -m "$(cat <<'EOF'
feat: add package Go modules to workspace

Update go.work to include grit, get-hubbed, lux, and tap-dancer/go.
Remove get-hubbed's replace directive for deps/purse-first.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Update Rust Path Dependencies

**Files:**
- Modify: `packages/chix/Cargo.toml`
- Possibly modify: `packages/tap-dancer/rust/Cargo.toml`

**Step 1: Update chix Cargo.toml**

Change the `mcp-server` dependency from a git URL to a path:

```toml
# Before:
mcp-server = { git = "https://github.com/amarbel-llc/rust-lib-mcp", features = ["tools", "resources"] }

# After:
mcp-server = { path = "../../libs/rust-mcp", features = ["tools", "resources"] }
```

**Step 2: Verify chix builds**

Run: `cd packages/chix && cargo check`
Expected: Clean check, no errors.

**Step 3: Check tap-dancer/rust for rust-mcp dependency**

Read `packages/tap-dancer/rust/Cargo.toml` to see if it depends on rust-mcp. If so, update similarly. (Based on exploration, tap-dancer's Rust library is standalone — no mcp-server dependency — but verify.)

**Step 4: Commit**

```bash
git add packages/chix/Cargo.toml
git commit -m "$(cat <<'EOF'
feat: update chix to use local rust-mcp path dependency

Replace git URL with relative path to libs/rust-mcp, eliminating
the external dependency.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: Write Nix Build Expressions for Go Packages (grit, get-hubbed, lux)

**Files:**
- Create: `lib/packages/grit.nix`
- Create: `lib/packages/get-hubbed.nix`
- Create: `lib/packages/lux.nix`

**Step 1: Write lib/packages/grit.nix**

Based on the original `grit/flake.nix`. Uses `buildGoApplication`:

```nix
{ pkgs, src, goOverlay }:

let
  gritPkgs = import pkgs.path {
    inherit (pkgs) system;
    overlays = [ goOverlay ];
  };
in
gritPkgs.buildGoApplication {
  pname = "grit";
  version = "0.1.0";
  inherit src;
  modules = "${src}/gomod2nix.toml";
  subPackages = [ "cmd/grit" ];

  postInstall = ''
    $out/bin/grit generate-plugin $out
  '';

  meta = with pkgs.lib; {
    description = "MCP for git, wow that's grit";
    homepage = "https://github.com/amarbel-llc/grit";
    license = licenses.mit;
  };
}
```

**Step 2: Write lib/packages/get-hubbed.nix**

Based on `get-hubbed/flake.nix`. Key difference: no longer needs to copy `purse-first.lib.goSrc` into a deps directory. The go.work workspace handles module resolution during development, and for Nix builds we need to ensure the purse-first source is available. The cleanest approach: since get-hubbed imports the `purse` package from the root module, the Nix build needs access to that source. Use `purse-first-src` (already defined in flake.nix) as an overlay.

```nix
{ pkgs, src, goOverlay, purse-first-src }:

let
  ghPkgs = import pkgs.path {
    inherit (pkgs) system;
    overlays = [ goOverlay ];
  };

  # get-hubbed needs the purse-first source for its replace directive
  get-hubbed-src = pkgs.runCommand "get-hubbed-src" { } ''
    cp -r ${src} $out
    chmod -R u+w $out
    rm -f $out/go.work $out/go.work.sum
    mkdir -p $out/deps
    cp -r ${purse-first-src} $out/deps/purse-first
  '';
in
ghPkgs.buildGoApplication {
  pname = "get-hubbed";
  version = "0.1.0";
  pwd = get-hubbed-src;
  src = get-hubbed-src;
  modules = "${src}/gomod2nix.toml";
  subPackages = [ "cmd/get-hubbed" ];

  postInstall = ''
    $out/bin/get-hubbed generate-plugin $out/share/purse-first
  '';

  meta = with pkgs.lib; {
    description = "`gh` cli wrapper with MCP support packaged by nix";
    homepage = "https://github.com/amarbel-llc/get-hubbed";
    license = licenses.mit;
  };
}
```

Note: get-hubbed's `go.mod` has a `replace` directive for purse-first. In Task 2 we removed it for local dev (go.work handles it). For the Nix build (which ignores go.work), we need to either:
- (a) Re-add the replace directive in the Nix build source prep, or
- (b) Keep the replace directive in go.mod and have go.work override it locally

Option (b) is simpler — keep the replace directive in `packages/get-hubbed/go.mod` as `replace github.com/amarbel-llc/purse-first => ./deps/purse-first` and the Nix build copies purse-first source there. go.work overrides it for local dev. **Revise Task 2 Step 4**: do NOT remove the replace directive.

**Step 3: Write lib/packages/lux.nix**

Based on `lux/flake.nix`. Includes scdoc man page generation:

```nix
{ pkgs, src, goOverlay }:

let
  luxPkgs = import pkgs.path {
    inherit (pkgs) system;
    overlays = [ goOverlay ];
  };
in
luxPkgs.buildGoApplication {
  pname = "lux";
  version = "0.1.0";
  inherit src;
  modules = "${src}/gomod2nix.toml";
  subPackages = [ "cmd/lux" ];

  nativeBuildInputs = [ pkgs.scdoc ];

  ldflags = [ "-X main.version=0.1.0" ];

  postInstall = ''
    $out/bin/lux _generate $out

    mkdir -p $out/share/man/man5
    scdoc < ${src}/doc/lux-config.5.scd > $out/share/man/man5/lux-config.5
  '';

  meta = with pkgs.lib; {
    description = "LSP Multiplexer that routes requests to language servers based on file type";
    homepage = "https://github.com/amarbel-llc/lux";
    license = licenses.mit;
  };
}
```

**Step 4: Commit**

```bash
git add lib/packages/
git commit -m "$(cat <<'EOF'
feat: add Nix build expressions for Go packages

Add grit.nix, get-hubbed.nix, and lux.nix to lib/packages/.
Each builds the package from local sources using buildGoApplication.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: Write Nix Build Expressions for Rust/Shell Packages (chix, batman, tap-dancer)

**Files:**
- Create: `lib/packages/chix.nix`
- Create: `lib/packages/batman.nix`
- Create: `lib/packages/tap-dancer.nix`

**Step 1: Write lib/packages/chix.nix**

Based on `chix/flake.nix`. Uses crane for Rust, wraps with fh/cachix/nil, includes hooks:

```nix
{ pkgs, src, craneLib, fhPkg }:

let
  rustSrc = craneLib.cleanCargoSource src;

  commonArgs = {
    src = rustSrc;
    strictDeps = true;
  };

  cargoArtifacts = craneLib.buildDepsOnly commonArgs;

  chix-unwrapped = craneLib.buildPackage (
    commonArgs // { inherit cargoArtifacts; }
  );

  formatNixHook = pkgs.writeShellScript "format-nix" ''
    set -euo pipefail
    input=$(cat)
    file_path=$(${pkgs.jq}/bin/jq -r '.tool_input.file_path // empty' <<< "$input")
    if [[ -n "$file_path" && "$file_path" == *.nix ]]; then
      ${pkgs.nixfmt-rfc-style}/bin/nixfmt "$file_path" 2>/dev/null || true
    fi
  '';
in
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

    mkdir -p $out/share/purse-first/chix/hooks
    cp ${src}/.claude-plugin/plugin.json $out/share/purse-first/chix/plugin.json
    install -m 755 ${formatNixHook} $out/share/purse-first/chix/hooks/format-nix
  ''
```

**Step 2: Write lib/packages/batman.nix**

Based on `batman/flake.nix`. Builds bats-support, bats-assert, bats-assert-additions, tap-writer, bats wrapper, and robin skill. Key change: robin no longer calls `purse-first generate-local-plugin` from an external flake input — it uses the local purse-first binary.

```nix
{ pkgs, src, sandcastle, purse-first-cli }:

let
  bats-support = pkgs.stdenvNoCC.mkDerivation {
    pname = "bats-support";
    version = "0.3.0";
    src = "${src}/lib/bats-support";
    dontBuild = true;
    installPhase = ''
      mkdir -p $out/share/bats/bats-support/src
      cp load.bash $out/share/bats/bats-support/
      cp src/*.bash $out/share/bats/bats-support/src/
    '';
  };

  bats-assert = pkgs.stdenvNoCC.mkDerivation {
    pname = "bats-assert";
    version = "2.1.0";
    src = "${src}/lib/bats-assert";
    dontBuild = true;
    installPhase = ''
      mkdir -p $out/share/bats/bats-assert/src
      cp load.bash $out/share/bats/bats-assert/
      cp src/*.bash $out/share/bats/bats-assert/src/
    '';
  };

  bats-assert-additions = pkgs.stdenvNoCC.mkDerivation {
    pname = "bats-assert-additions";
    version = "0.1.0";
    src = "${src}/lib/bats-assert-additions";
    dontBuild = true;
    installPhase = ''
      mkdir -p $out/share/bats/bats-assert-additions/src
      cp load.bash $out/share/bats/bats-assert-additions/
      cp src/*.bash $out/share/bats/bats-assert-additions/src/
    '';
  };

  tap-writer = pkgs.stdenvNoCC.mkDerivation {
    pname = "tap-writer";
    version = "0.1.0";
    src = "${src}/lib/tap-writer";
    dontBuild = true;
    installPhase = ''
      mkdir -p $out/share/bats/tap-writer/src
      cp load.bash $out/share/bats/tap-writer/
      cp src/*.bash $out/share/bats/tap-writer/src/
    '';
  };

  bats-libs = pkgs.symlinkJoin {
    name = "bats-libs";
    paths = [ bats-support bats-assert bats-assert-additions tap-writer ];
  };

  bats = pkgs.writeShellApplication {
    name = "bats";
    runtimeInputs = [
      pkgs.bats
      pkgs.coreutils
      sandcastle
    ];
    text = builtins.readFile "${src}/bats-wrapper.bash";
    # Note: the bats wrapper script content is inline in the original flake.
    # Extract it to packages/batman/bats-wrapper.bash or inline it here.
  };

  robin = pkgs.stdenvNoCC.mkDerivation {
    pname = "robin";
    version = "0.1.0";
    inherit src;
    dontBuild = true;
    nativeBuildInputs = [ purse-first-cli ];
    installPhase = ''
      mkdir -p $out/share/purse-first/robin/skills
      cp -r skills/* $out/share/purse-first/robin/skills/
      staging=$(mktemp -d)
      ln -s $out/share/purse-first/robin/skills $staging/skills
      mkdir -p $staging/.claude-plugin
      cp .claude-plugin/plugin.json $staging/.claude-plugin/plugin.json
      chmod u+w $staging/.claude-plugin/plugin.json
      purse-first generate-local-plugin --root $staging
      cp $staging/.claude-plugin/plugin.json $out/share/purse-first/robin/plugin.json
    '';
  };
in
{
  default = pkgs.symlinkJoin {
    name = "batman";
    paths = [ bats-libs bats robin ];
  };
  inherit bats-support bats-assert bats-assert-additions tap-writer bats-libs bats robin;
}
```

Note: The `bats` wrapper has its script inline in the original flake.nix. For the monorepo, either:
- Extract the script to `packages/batman/bats-wrapper.bash` and reference it
- Keep it inline in the nix expression

The inline approach is simpler since the script references Nix store paths.

**Step 3: Write lib/packages/tap-dancer.nix**

Based on `tap-dancer/flake.nix`. Multi-output: Go CLI, Rust lib, Bash lib, skill:

```nix
{ pkgs, src, goOverlay, craneLib }:

let
  tdPkgs = import pkgs.path {
    inherit (pkgs) system;
    overlays = [ goOverlay ];
  };

  version = "0.1.0";

  tap-dancer-cli = tdPkgs.buildGoApplication {
    pname = "tap-dancer";
    inherit version;
    src = "${src}/go";
    modules = "${src}/go/gomod2nix.toml";
    subPackages = [ "cmd/tap-dancer" ];
    postInstall = ''
      $out/bin/tap-dancer generate-plugin $out
    '';
    meta = with pkgs.lib; {
      description = "TAP-14 validator and writer toolkit";
      homepage = "https://github.com/amarbel-llc/tap-dancer";
      license = licenses.mit;
    };
  };

  rustSrc = craneLib.cleanCargoSource "${src}/rust";
  cargoArtifacts = craneLib.buildDepsOnly {
    src = rustSrc;
    strictDeps = true;
  };
  tap-dancer-rust = craneLib.buildPackage {
    src = rustSrc;
    inherit cargoArtifacts;
    strictDeps = true;
  };

  tap-dancer-skill = pkgs.runCommand "tap-dancer-skill" { } ''
    mkdir -p $out/share/purse-first/tap-dancer/skills
    cp -r ${src}/skills/* $out/share/purse-first/tap-dancer/skills/
    cp ${src}/.claude-plugin/plugin.json $out/share/purse-first/tap-dancer/plugin.json
  '';

  tap-dancer-bash = pkgs.stdenvNoCC.mkDerivation {
    pname = "tap-dancer-bash";
    inherit version;
    src = "${src}/bash";
    dontBuild = true;
    installPhase = ''
      mkdir -p $out/share/tap-dancer/lib/src
      cp load.bash $out/share/tap-dancer/lib/
      cp src/*.bash $out/share/tap-dancer/lib/src/
      mkdir -p $out/nix-support
      echo 'export TAP_DANCER_LIB="'"$out"'/share/tap-dancer/lib"' > $out/nix-support/setup-hook
    '';
  };
in
{
  default = pkgs.symlinkJoin {
    name = "tap-dancer";
    paths = [ tap-dancer-cli tap-dancer-rust tap-dancer-skill tap-dancer-bash ];
  };
  cli = tap-dancer-cli;
  rust = tap-dancer-rust;
  skill = tap-dancer-skill;
  bash-lib = tap-dancer-bash;
}
```

**Step 4: Commit**

```bash
git add lib/packages/
git commit -m "$(cat <<'EOF'
feat: add Nix build expressions for chix, batman, tap-dancer

Add chix.nix (Rust/crane), batman.nix (shell/skill), and
tap-dancer.nix (multi-output Go+Rust+Bash) to lib/packages/.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: Rewrite flake.nix

**Files:**
- Modify: `flake.nix`

This is the most critical task. The flake.nix must:
1. Remove all 6 external package inputs
2. Add inputs needed by packages (crane, rust-overlay, fh, sandcastle)
3. Build all packages from local sources
4. Pass local packages to mkMarketplace
5. Export individual packages

**Step 1: Rewrite flake.nix**

Key changes from current flake.nix:

**Removed inputs:** `grit`, `get-hubbed`, `lux`, `chix`, `batman`, `tap-dancer`

**Added inputs:** `crane`, `rust-overlay`, `fh`, `sandcastle` (needed by chix, batman, tap-dancer)

**Structure:**

```nix
{
  description = "Claude Plugin Marketplace: MCP servers and tool routing for Claude Code";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/<current-sha>";
    nixpkgs-master.url = "github:NixOS/nixpkgs/<current-sha>";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";

    # Dev environment inputs
    go.url = "github:amarbel-llc/purse-first?dir=devenvs/go";
    shell.url = "github:amarbel-llc/purse-first?dir=devenvs/shell";
    bats.url = "github:amarbel-llc/purse-first?dir=devenvs/bats";
    rust.url = "github:amarbel-llc/purse-first?dir=devenvs/rust";

    # Build inputs for Rust packages
    crane.url = "github:ipetkov/crane";
    rust-overlay = {
      url = "github:oxalica/rust-overlay";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    # Runtime dependencies
    fh.url = "https://flakehub.com/f/DeterminateSystems/fh/*.tar.gz";
    sandcastle = {
      url = "github:amarbel-llc/sandcastle";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
      inputs.utils.follows = "utils";
    };
  };

  outputs = { self, nixpkgs, nixpkgs-master, utils, go, shell, bats, rust,
              crane, rust-overlay, fh, sandcastle }:
    let
      mkMarketplace = import ./lib/mkMarketplace.nix;
      purse-first-src = nixpkgs.lib.cleanSourceWith { /* ... same as before ... */ };

      # ... marketplaceOutputs using local packages ...
    in
    # ... outputs structure ...
  ;
}
```

In the outputs, for each system:
- Set up crane with rust-overlay
- Import each `lib/packages/*.nix` with appropriate args
- Pass the resulting derivations to `mkMarketplace` via `plugins`
- Export individual packages

The `wrapGetHubbed` helper is replaced by the wrapping built into `lib/packages/get-hubbed.nix` (or kept in flake.nix if the `gh` wrapping is marketplace-specific).

**Step 2: Verify the flake evaluates**

Run: `nix flake check --no-build`
Expected: No evaluation errors.

**Step 3: Build each package individually**

```bash
nix build .#grit
nix build .#get-hubbed
nix build .#lux
nix build .#chix
nix build .#robin
nix build .#tap-dancer
```

**Step 4: Build the full marketplace**

Run: `nix build`
Expected: Marketplace builds with all packages included.

**Step 5: Commit**

```bash
git add flake.nix flake.lock
git commit -m "$(cat <<'EOF'
feat: rewrite flake.nix for monorepo

Remove external package flake inputs (grit, get-hubbed, lux, chix,
batman, tap-dancer). Build all packages from local sources in
packages/. Add crane, rust-overlay, fh, sandcastle as build inputs.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: Update marketplace-config.json

**Files:**
- Modify: `marketplace-config.json`

**Step 1: Update repo references**

Change all individual repo references to point to the monorepo:

```json
{
  "plugins": {
    "grit": {
      "repo": "amarbel-llc/purse-first",
      "homepage": "https://github.com/amarbel-llc/purse-first"
    },
    "get-hubbed": {
      "repo": "amarbel-llc/purse-first",
      "homepage": "https://github.com/amarbel-llc/purse-first"
    },
    "lux": {
      "repo": "amarbel-llc/purse-first",
      "homepage": "https://github.com/amarbel-llc/purse-first"
    },
    "chix": {
      "repo": "amarbel-llc/purse-first",
      "homepage": "https://github.com/amarbel-llc/purse-first"
    },
    "robin": {
      "repo": "amarbel-llc/purse-first",
      "homepage": "https://github.com/amarbel-llc/purse-first"
    },
    "tap-dancer": {
      "repo": "amarbel-llc/purse-first",
      "homepage": "https://github.com/amarbel-llc/purse-first"
    }
  }
}
```

Keep all other fields (description, version, category, tags) unchanged.

**Step 2: Commit**

```bash
git add marketplace-config.json
git commit -m "$(cat <<'EOF'
chore: update marketplace-config.json repo references

Point all package repo and homepage fields to the monorepo.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 8: Update justfile

**Files:**
- Modify: `justfile`

**Step 1: Add per-package build and test targets**

Add targets for building and testing individual packages. Update existing targets that reference external builds.

New/updated targets:

```just
# Build individual packages (these already exist, verify they work)
build-grit:
    nix build .#grit

build-lux:
    nix build .#lux

build-get-hubbed:
    nix build .#get-hubbed

build-chix:
    nix build .#chix

build-robin:
    nix build .#robin

build-tap-dancer:
    nix build .#tap-dancer

# Test individual Go packages
test-grit:
    nix develop --command go test ./packages/grit/...

test-get-hubbed:
    nix develop --command go test ./packages/get-hubbed/...

test-lux:
    nix develop --command go test ./packages/lux/...

test-tap-dancer-go:
    nix develop --command go test ./packages/tap-dancer/go/...

# Test Rust packages
test-chix:
    cd packages/chix && nix develop ../../ --command cargo test

test-tap-dancer-rust:
    cd packages/tap-dancer/rust && nix develop ../../../ --command cargo test

# Test BATS
test-grit-bats:
    nix build .#grit
    nix develop --command bats --tap packages/grit/zz-tests_bats/*.bats

test-batman-bats:
    nix develop --command bats --tap packages/batman/zz-tests_bats/*.bats
```

Update `test-all` to include package tests.

**Step 2: Commit**

```bash
git add justfile
git commit -m "$(cat <<'EOF'
chore: add per-package build and test targets to justfile

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 9: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Update repository layout table**

Add `packages/` directory to the layout table:

```markdown
| `packages/grit/` | Git MCP server (Go) |
| `packages/get-hubbed/` | GitHub MCP server (Go) |
| `packages/lux/` | LSP multiplexer MCP server (Go) |
| `packages/chix/` | Nix MCP+Skill server (Rust) |
| `packages/batman/` | BATS testing skill + libraries (Shell/Nix) |
| `packages/tap-dancer/` | TAP-14 libraries + skill (Go/Rust/Bash) |
| `lib/packages/` | Per-package Nix build expressions |
```

**Step 2: Add note about monorepo structure**

Add a section explaining the Go workspace and how packages are built.

**Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "$(cat <<'EOF'
docs: update CLAUDE.md for monorepo layout

Add packages/ directory entries and monorepo build notes.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```

---

### Task 10: Full Integration Test

**Step 1: Run full Nix build**

Run: `nix build`
Expected: Marketplace builds successfully with all packages.

**Step 2: Inspect marketplace output**

```bash
cat result/.claude-plugin/marketplace.json | jq '.plugins | keys'
```
Expected: All 7 plugins listed (grit, get-hubbed, lux, chix, purse-first, robin, tap-dancer).

**Step 3: Run all tests**

Run: `just test-all`
Expected: All tests pass.

**Step 4: Run BATS integration tests**

Run: `just test-integration`
Expected: Marketplace validation, document validation, all pass.

**Step 5: Verify individual package builds**

```bash
for pkg in grit get-hubbed lux chix robin tap-dancer purse-first; do
  echo "Building $pkg..."
  nix build ".#$pkg"
done
```

**Step 6: Run nix flake check**

Run: `nix flake check`
Expected: All checks pass.

---

### Task 11: Clean Up Package Artifacts

**Step 1: Remove per-package .envrc files**

The packages don't need their own `.envrc` since they use the monorepo's dev shell:

```bash
for pkg in grit get-hubbed lux chix batman tap-dancer; do
  rm -f "packages/$pkg/.envrc"
done
```

**Step 2: Remove per-package docs/plans/**

Plan documents from original repos are historical — they can be removed or kept. Recommend removing to avoid confusion:

```bash
for pkg in grit get-hubbed lux chix batman tap-dancer; do
  rm -rf "packages/$pkg/docs/plans"
done
```

**Step 3: Remove per-package CLAUDE.md files (if redundant)**

If any packages have CLAUDE.md files that reference the old repo structure, remove or update them.

**Step 4: Commit**

```bash
git add -A
git commit -m "$(cat <<'EOF'
chore: clean up per-package artifacts

Remove per-package .envrc files (monorepo dev shell used instead)
and historical plan documents.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
EOF
)"
```
