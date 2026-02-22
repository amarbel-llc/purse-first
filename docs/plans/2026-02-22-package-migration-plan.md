# Package Migration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Migrate sandcastle, and-so-can-you-repo, potato, and spinclass (renamed from sweatshop) into the purse-first monorepo under `packages/`.

**Architecture:** Copy source files from each repo into `packages/<name>/`, create Nix build expressions in `lib/packages/`, wire into `flake.nix` and `marketplace-config.json`. For spinclass, do a full rename from sweatshop (module path, binary, imports) while keeping `sweatfile` config name.

**Tech Stack:** Nix (buildNpmPackage, buildGoApplication, writeScriptBin), Go workspace (go.work), TypeScript/Node (npm)

---

### Task 1: Copy sandcastle source into packages/sandcastle/

**Files:**
- Create: `packages/sandcastle/` (directory with source files)

**Step 1: Copy source files from sandcastle repo**

```bash
mkdir -p packages/sandcastle
# Copy source files, excluding .git, node_modules, dist, .direnv, result
cp -r /home/sasha/eng/repos/sandcastle/src packages/sandcastle/
cp -r /home/sasha/eng/repos/sandcastle/vendor packages/sandcastle/
cp -r /home/sasha/eng/repos/sandcastle/skills packages/sandcastle/
cp -r /home/sasha/eng/repos/sandcastle/.claude-plugin packages/sandcastle/
cp -r /home/sasha/eng/repos/sandcastle/zz-tests_bats packages/sandcastle/
cp /home/sasha/eng/repos/sandcastle/sandcastle-cli.mjs packages/sandcastle/
cp /home/sasha/eng/repos/sandcastle/package.json packages/sandcastle/
cp /home/sasha/eng/repos/sandcastle/package-lock.json packages/sandcastle/
cp /home/sasha/eng/repos/sandcastle/tsconfig.json packages/sandcastle/
cp /home/sasha/eng/repos/sandcastle/.npmrc packages/sandcastle/
```

**Step 2: Verify files copied correctly**

```bash
ls -la packages/sandcastle/
ls packages/sandcastle/src/
```

Expected: src/, vendor/, skills/, .claude-plugin/, zz-tests_bats/, sandcastle-cli.mjs, package.json, package-lock.json, tsconfig.json, .npmrc

**Step 3: Commit**

```bash
git add packages/sandcastle/
git commit -m "feat: add sandcastle source to packages/"
```

---

### Task 2: Create lib/packages/sandcastle.nix

**Files:**
- Create: `lib/packages/sandcastle.nix`

**Step 1: Write the Nix build expression**

Based on the existing sandcastle flake.nix, create `lib/packages/sandcastle.nix`:

```nix
{ pkgs, src }:

pkgs.buildNpmPackage {
  pname = "sandcastle";
  version = "0.0.37";

  inherit src;

  npmDepsHash = "sha256-LMqLtMWMmzEiHW+VJAPnivqHtoJV2wWWP2S8Z/smfWc=";

  nativeBuildInputs = [ pkgs.makeWrapper ];

  buildPhase = ''
    runHook preBuild
    npm run build
    runHook postBuild
  '';

  installPhase = ''
    runHook preInstall

    mkdir -p $out/lib/sandcastle $out/bin

    cp -r dist/* $out/lib/sandcastle/
    cp -r node_modules $out/lib/sandcastle/
    cp package.json $out/lib/sandcastle/
    cp ${src}/sandcastle-cli.mjs $out/lib/sandcastle/sandcastle-cli.mjs

    makeWrapper ${pkgs.nodejs_22}/bin/node $out/bin/sandcastle \
      --add-flags "$out/lib/sandcastle/sandcastle-cli.mjs" \
      --prefix PATH : ${
        pkgs.lib.makeBinPath (
          [
            pkgs.socat
            pkgs.ripgrep
          ]
          ++ pkgs.lib.optionals pkgs.stdenv.isLinux [ pkgs.bubblewrap ]
        )
      }

    runHook postInstall
  '';
}
```

**Step 2: Commit**

```bash
git add lib/packages/sandcastle.nix
git commit -m "feat: add sandcastle Nix build expression"
```

---

### Task 3: Wire sandcastle into flake.nix

**Files:**
- Modify: `flake.nix`

**Step 1: Remove sandcastle flake input**

In `flake.nix`, remove lines 20-25 (the `sandcastle` input block):
```nix
    sandcastle = {
      url = "github:amarbel-llc/sandcastle";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
      inputs.utils.follows = "utils";
    };
```

Remove `sandcastle,` from the outputs destructuring (line 41).

**Step 2: Replace sandcastlePkg with local build**

In `buildPackages`, replace:
```nix
sandcastlePkg = sandcastle.packages.${system}.default;
```

With:
```nix
sandcastlePkg = import ./lib/packages/sandcastle.nix {
  inherit pkgs;
  src = ./packages/sandcastle;
};
```

**Step 3: Register sandcastle in packages output**

In the `packages` attribute set (around line 235-252), add:
```nix
sandcastle = localPkgs.sandcastlePkg;
```

Also add `sandcastlePkg` to the `buildPackages` return set (around line 142-151):
```nix
inherit
  gritPkg
  get-hubbed-wrapped
  luxPkg
  chixPkg
  tapDancerPkgs
  batmanPkgs
  sandcastlePkg
  ;
```

**Step 4: Verify Nix evaluation**

```bash
nix flake check --no-build 2>&1 | head -20
```

Expected: No evaluation errors (build errors OK at this stage)

**Step 5: Commit**

```bash
git add flake.nix
git commit -m "feat: wire sandcastle into flake.nix from local source"
```

---

### Task 4: Copy and-so-can-you-repo source into packages/

**Files:**
- Create: `packages/and-so-can-you-repo/bin/and-so-can-you-repo.bash`

**Step 1: Copy source files**

```bash
mkdir -p packages/and-so-can-you-repo/bin
cp /home/sasha/eng/repos/and-so-can-you-repo/bin/and-so-can-you-repo.bash packages/and-so-can-you-repo/bin/
```

**Step 2: Commit**

```bash
git add packages/and-so-can-you-repo/
git commit -m "feat: add and-so-can-you-repo source to packages/"
```

---

### Task 5: Create lib/packages/and-so-can-you-repo.nix and wire into flake.nix

**Files:**
- Create: `lib/packages/and-so-can-you-repo.nix`
- Modify: `flake.nix`

**Step 1: Write the Nix build expression**

```nix
{ pkgs, src }:

let
  name = "and-so-can-you-repo";
  script = (
    pkgs.writeScriptBin name (builtins.readFile "${src}/bin/and-so-can-you-repo.bash")
  ).overrideAttrs (old: {
    buildCommand = "${old.buildCommand}\n patchShebangs $out";
  });
  buildInputs = with pkgs; [
    gum
    gh
  ];
in
pkgs.symlinkJoin {
  inherit name;
  paths = [ script ] ++ buildInputs;
  buildInputs = [ pkgs.makeWrapper ];
  postBuild = "wrapProgram $out/bin/${name} --prefix PATH : $out/bin";
}
```

**Step 2: Wire into flake.nix buildPackages**

Add to `buildPackages` function:
```nix
andSoCanYouRepoPkg = import ./lib/packages/and-so-can-you-repo.nix {
  inherit pkgs;
  src = ./packages/and-so-can-you-repo;
};
```

Add `andSoCanYouRepoPkg` to the return set.

Add to the `packages` output:
```nix
and-so-can-you-repo = localPkgs.andSoCanYouRepoPkg;
```

**Step 3: Verify Nix evaluation**

```bash
nix flake check --no-build 2>&1 | head -20
```

**Step 4: Commit**

```bash
git add lib/packages/and-so-can-you-repo.nix flake.nix
git commit -m "feat: add and-so-can-you-repo Nix build and flake wiring"
```

---

### Task 6: Copy potato source into packages/potato/

**Files:**
- Create: `packages/potato/` (directory with Go source)

**Step 1: Copy source files**

```bash
mkdir -p packages/potato
cp -r /home/sasha/eng/repos/potato/cmd packages/potato/
cp -r /home/sasha/eng/repos/potato/internal packages/potato/
cp /home/sasha/eng/repos/potato/go.mod packages/potato/
cp /home/sasha/eng/repos/potato/go.sum packages/potato/
cp /home/sasha/eng/repos/potato/gomod2nix.toml packages/potato/
```

**Step 2: Verify files**

```bash
ls -la packages/potato/
ls packages/potato/cmd/potato/
ls packages/potato/internal/
```

Expected: cmd/potato/, internal/timer/, internal/zmx/, go.mod, go.sum, gomod2nix.toml

**Step 3: Commit**

```bash
git add packages/potato/
git commit -m "feat: add potato source to packages/"
```

---

### Task 7: Create lib/packages/potato.nix, update go.work, wire into flake.nix

**Files:**
- Create: `lib/packages/potato.nix`
- Modify: `go.work`
- Modify: `flake.nix`

**Step 1: Write the Nix build expression**

```nix
{
  pkgs,
  src,
  goOverlay,
}:

let
  goPkgs = import pkgs.path {
    inherit (pkgs) system;
    overlays = [ goOverlay ];
  };
in
goPkgs.buildGoApplication {
  pname = "potato";
  version = "0.1.0";
  inherit src;
  modules = "${src}/gomod2nix.toml";
  subPackages = [ "cmd/potato" ];

  meta = with pkgs.lib; {
    description = "pomodoro timer that requires the potato to rest for 5 minutes";
    homepage = "https://github.com/friedenberg/potato";
    license = licenses.mit;
  };
}
```

**Step 2: Add potato to go.work**

Add `./packages/potato` to the `use` block:
```
use (
	.
	./libs/go-mcp
	./libs/go-mcp/command/huh
	./packages/get-hubbed
	./packages/grit
	./packages/lux
	./packages/potato
	./packages/tap-dancer/go
)
```

**Step 3: Wire into flake.nix buildPackages**

Add to `buildPackages`:
```nix
potatoPkg = import ./lib/packages/potato.nix {
  inherit pkgs goOverlay;
  src = ./packages/potato;
};
```

Add `potatoPkg` to the return set.

Add to the `packages` output:
```nix
potato = localPkgs.potatoPkg;
```

**Step 4: Verify Go workspace resolves**

```bash
go work sync 2>&1
```

Expected: No errors

**Step 5: Commit**

```bash
git add lib/packages/potato.nix go.work flake.nix
git commit -m "feat: add potato Nix build, go.work entry, and flake wiring"
```

---

### Task 8: Copy sweatshop source into packages/spinclass/ with rename

**Files:**
- Create: `packages/spinclass/` (directory with renamed Go source)

**Step 1: Copy source files**

```bash
mkdir -p packages/spinclass
cp -r /home/sasha/eng/repos/sweatshop/cmd packages/spinclass/
cp -r /home/sasha/eng/repos/sweatshop/internal packages/spinclass/
cp -r /home/sasha/eng/repos/sweatshop/completions packages/spinclass/
cp /home/sasha/eng/repos/sweatshop/go.mod packages/spinclass/
cp /home/sasha/eng/repos/sweatshop/go.sum packages/spinclass/
cp /home/sasha/eng/repos/sweatshop/gomod2nix.toml packages/spinclass/
```

**Step 2: Rename cmd directory**

```bash
mv packages/spinclass/cmd/sweatshop packages/spinclass/cmd/spinclass
```

**Step 3: Rename Go module path in go.mod**

In `packages/spinclass/go.mod`, change:
```
module github.com/amarbel-llc/sweatshop
```
to:
```
module github.com/amarbel-llc/spinclass
```

**Step 4: Rename all Go import paths**

In all `.go` files under `packages/spinclass/`, replace:
```
github.com/amarbel-llc/sweatshop
```
with:
```
github.com/amarbel-llc/spinclass
```

Files to update (Go source files only — skip docs/):
- `cmd/spinclass/main.go`
- `internal/clean/clean.go`
- `internal/completions/completions.go`
- `internal/merge/merge.go`
- `internal/perms/cmd.go`
- `internal/pull/pull.go`
- `internal/shop/shop.go`
- `internal/status/status.go`
- `internal/sweatfile/apply.go`
- `internal/worktree/worktree.go`

**Step 5: Rename cobra root command in cmd/spinclass/main.go**

Change:
```go
var rootCmd = &cobra.Command{
	Use:   "sweatshop",
	Short: "Shell-agnostic git worktree session manager",
	Long:  `sweatshop manages git worktree lifecycles...`,
}
```
to:
```go
var rootCmd = &cobra.Command{
	Use:   "spinclass",
	Short: "Shell-agnostic git worktree session manager",
	Long:  `spinclass manages git worktree lifecycles: creating worktrees + sessions, and offering close workflows (rebase, merge, cleanup, push).`,
}
```

Note: The `rootCmd.Use = filepath.Base(os.Args[0])` at line 201 means the binary adapts its name dynamically, but the default Use field should still say `spinclass`.

**Step 6: Rename shell completions filenames**

The completions directory should already have both `sweatshop.*` and `spinclass.*` files. Keep them as-is since the tool supports running under both names via `filepath.Base(os.Args[0])`. Actually — since this is a full rename, rename the sweatshop completion files:

```bash
mv packages/spinclass/completions/sweatshop.bash-completion packages/spinclass/completions/spinclass-legacy.bash-completion 2>/dev/null || true
mv packages/spinclass/completions/sweatshop.fish packages/spinclass/completions/spinclass-legacy.fish 2>/dev/null || true
```

Wait — the completions reference the binary name. Check if the spinclass completions already exist and keep them. Remove the sweatshop-named ones since the binary is now `spinclass`.

Actually, keep the existing spinclass.* completion files and remove the sweatshop.* ones:
```bash
rm packages/spinclass/completions/sweatshop.bash-completion
rm packages/spinclass/completions/sweatshop.fish
```

**Step 7: Verify Go compiles**

```bash
cd packages/spinclass && go build ./cmd/spinclass/ && cd ../..
```

Expected: Compiles without errors

**Step 8: Commit**

```bash
git add packages/spinclass/
git commit -m "feat: add spinclass source to packages/ (renamed from sweatshop)"
```

---

### Task 9: Create lib/packages/spinclass.nix, update go.work, wire into flake.nix

**Files:**
- Create: `lib/packages/spinclass.nix`
- Modify: `go.work`
- Modify: `flake.nix`

**Step 1: Write the Nix build expression**

```nix
{
  pkgs,
  src,
  goOverlay,
}:

let
  goPkgs = import pkgs.path {
    inherit (pkgs) system;
    overlays = [ goOverlay ];
  };

  spinclass = goPkgs.buildGoApplication {
    pname = "spinclass";
    version = "0.1.0";
    inherit src;
    modules = "${src}/gomod2nix.toml";
    subPackages = [ "cmd/spinclass" ];
  };

  shellCompletions = pkgs.runCommand "spinclass-completions" { } ''
    install -Dm644 ${src}/completions/spinclass.bash-completion \
      $out/share/bash-completion/completions/spinclass
    install -Dm644 ${src}/completions/spinclass.fish \
      $out/share/fish/vendor_completions.d/spinclass.fish
  '';
in
pkgs.symlinkJoin {
  name = "spinclass";
  paths = [
    spinclass
    shellCompletions
  ];
}
```

**Step 2: Add spinclass to go.work**

Add `./packages/spinclass` to the `use` block:
```
use (
	.
	./libs/go-mcp
	./libs/go-mcp/command/huh
	./packages/get-hubbed
	./packages/grit
	./packages/lux
	./packages/potato
	./packages/spinclass
	./packages/tap-dancer/go
)
```

**Step 3: Wire into flake.nix buildPackages**

Add to `buildPackages`:
```nix
spinclassPkg = import ./lib/packages/spinclass.nix {
  inherit pkgs goOverlay;
  src = ./packages/spinclass;
};
```

Add `spinclassPkg` to the return set.

Add to the `packages` output:
```nix
spinclass = localPkgs.spinclassPkg;
```

**Step 4: Verify Go workspace resolves**

```bash
go work sync 2>&1
```

**Step 5: Commit**

```bash
git add lib/packages/spinclass.nix go.work flake.nix
git commit -m "feat: add spinclass Nix build, go.work entry, and flake wiring"
```

---

### Task 10: Add all four packages to marketplace-config.json

**Files:**
- Modify: `marketplace-config.json`

**Step 1: Add entries**

Add to the `plugins` object in `marketplace-config.json`:

```json
"sandcastle": {
  "description": "Sandbox runtime for wrapping security boundaries around arbitrary processes",
  "version": "0.0.37",
  "homepage": "https://github.com/amarbel-llc/purse-first",
  "repo": "amarbel-llc/purse-first",
  "category": "security",
  "tags": ["sandbox", "security", "isolation", "testing"]
},
"and-so-can-you-repo": {
  "description": "Interactive scaffold for creating new Nix-flake-backed repositories",
  "version": "0.1.0",
  "homepage": "https://github.com/amarbel-llc/purse-first",
  "repo": "amarbel-llc/purse-first",
  "category": "development",
  "tags": ["scaffold", "nix", "templates", "cli"]
},
"potato": {
  "description": "Pomodoro timer that detaches from terminal sessions",
  "version": "0.1.0",
  "homepage": "https://github.com/amarbel-llc/purse-first",
  "repo": "amarbel-llc/purse-first",
  "category": "productivity",
  "tags": ["pomodoro", "timer", "tui", "cli"]
},
"spinclass": {
  "description": "Shell-agnostic git worktree session manager",
  "version": "0.1.0",
  "homepage": "https://github.com/amarbel-llc/purse-first",
  "repo": "amarbel-llc/purse-first",
  "category": "development",
  "tags": ["git", "worktrees", "sessions", "cli"]
}
```

**Step 2: Commit**

```bash
git add marketplace-config.json
git commit -m "feat: add sandcastle, and-so-can-you-repo, potato, spinclass to marketplace"
```

---

### Task 11: Build verification

**Step 1: Verify Nix flake evaluates**

```bash
nix flake check --no-build 2>&1 | head -30
```

Expected: No evaluation errors

**Step 2: Verify Go workspace compiles**

```bash
go build ./packages/potato/cmd/potato/
go build ./packages/spinclass/cmd/spinclass/
```

Expected: Both compile without errors

**Step 3: Try a Nix build (optional, may take time)**

```bash
nix build .#sandcastle --show-trace 2>&1 | tail -20
nix build .#potato --show-trace 2>&1 | tail -20
nix build .#spinclass --show-trace 2>&1 | tail -20
nix build .#and-so-can-you-repo --show-trace 2>&1 | tail -20
```

**Step 4: Run existing tests to verify nothing broke**

```bash
just test 2>&1 | tail -30
```

Expected: All existing tests still pass

**Step 5: Commit any fixups if needed**
