# Robin (Purse-First BATS Skill) Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Publish batman's BATS testing skill as `robin` in the purse-first marketplace.

**Architecture:** Batman is a skill-only plugin (no MCP server). The flake.nix gets a `robin` derivation that copies skills into `$out/share/purse-first/robin/` and uses `purse-first generate-local-plugin` to produce the manifest. Purse-first's marketplace flake then aggregates robin alongside existing plugins.

**Tech Stack:** Nix flakes, purse-first CLI (`generate-local-plugin`), BATS skills

---

### Task 1: Update batman's .claude-plugin/plugin.json

**Files:**
- Modify: `/home/sasha/eng/repos/batman/.claude-plugin/plugin.json`

**Step 1: Update plugin.json**

Replace the contents of `.claude-plugin/plugin.json` with:

```json
{
  "name": "robin",
  "description": "Expert skill for setting up and writing BATS integration tests with bats-support libraries, justfile integration, and sandcastle environment isolation",
  "author": {
    "name": "friedenberg"
  },
  "skills": [
    "./skills/bats-testing"
  ]
}
```

**Step 2: Commit**

```bash
git add .claude-plugin/plugin.json
git commit -m "Rename plugin to robin and add skills array for purse-first"
```

---

### Task 2: Add purse-first input and robin package to batman's flake.nix

**Files:**
- Modify: `/home/sasha/eng/repos/batman/flake.nix`

**Step 1: Add purse-first flake input**

Add to the `inputs` block (after the `shell` input):

```nix
purse-first = {
  url = "github:amarbel-llc/purse-first";
  inputs.nixpkgs.follows = "nixpkgs";
  inputs.nixpkgs-master.follows = "nixpkgs-master";
};
```

Add `purse-first` to the `outputs` function parameter list.

**Step 2: Add the robin derivation**

Add after the `bats-libs` definition (after line 69 in current file), inside the `let` block:

```nix
robin = pkgs.stdenvNoCC.mkDerivation {
  pname = "robin";
  version = "0.1.0";
  src = ./.;
  dontBuild = true;

  nativeBuildInputs = [
    purse-first.packages.${system}.purse-first
  ];

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
```

**Step 3: Update default package**

Change the default package from `bats-libs` to a `symlinkJoin` of both:

```nix
packages = {
  default = pkgs.symlinkJoin {
    name = "batman";
    paths = [
      bats-libs
      robin
    ];
  };
  inherit bats-support bats-assert bats-assert-additions bats-libs robin;
};
```

**Step 4: Build and verify**

Run: `nix build` in `/home/sasha/eng/repos/batman/`

Then verify output:
- `ls ./result/share/purse-first/robin/` should show `plugin.json` and `skills/`
- `ls ./result/share/purse-first/robin/skills/bats-testing/` should show `SKILL.md`, `examples/`, `references/`
- `cat ./result/share/purse-first/robin/plugin.json` should show the manifest with `skills` array
- `ls ./result/share/bats/` should still show the three bats libraries

**Step 5: Format**

Run: `nix run ~/eng/devenvs/nix#fmt -- flake.nix`

**Step 6: Commit**

```bash
git add flake.nix flake.lock
git commit -m "Add purse-first input and robin package for marketplace integration"
```

---

### Task 3: Register robin in purse-first marketplace

**Files:**
- Modify: `/home/sasha/eng/repos/purse-first/flake.nix`
- Modify: `/home/sasha/eng/repos/purse-first/marketplace-config.json`

**Step 1: Add batman flake input**

In `/home/sasha/eng/repos/purse-first/flake.nix`, add after the `nix-mcp-server` input (line 33):

```nix
batman = {
  url = "github:amarbel-llc/batman";
  inputs.nixpkgs.follows = "nixpkgs";
  inputs.nixpkgs-master.follows = "nixpkgs-master";
};
```

Add `batman` to the `outputs` function parameter list (after `nix-mcp-server` on line 49).

**Step 2: Add robin package binding**

After the `get-hubbed-pkg` definition (after line 89), add:

```nix
robin-pkg = batman.packages.${system}.robin;
```

**Step 3: Add robin to marketplace paths and postBuild**

In the `marketplace` symlinkJoin `paths` list (lines 136-141), add `robin-pkg`:

```nix
paths = [
  grit-pkg
  get-hubbed-pkg
  lux-pkg
  nix-mcp-server-pkg
  robin-pkg
];
```

**Step 4: Add robin metadata to marketplace-config.json**

In `/home/sasha/eng/repos/purse-first/marketplace-config.json`, add after the `bob` entry (line 49):

```json
,
"robin": {
  "description": "BATS integration testing skill with bundled assertion libraries, sandcastle isolation, and justfile patterns",
  "version": "0.1.0",
  "homepage": "https://github.com/amarbel-llc/batman",
  "repo": "amarbel-llc/batman",
  "category": "testing",
  "tags": ["bats", "testing", "integration", "shell", "nix"]
}
```

**Step 5: Update flake inputs and build**

Run in `/home/sasha/eng/repos/purse-first/`:

```bash
nix flake update batman
just build-all
```

Verify: `ls ./result/share/purse-first/robin/` should show `plugin.json` and `skills/`

**Step 6: Format**

Run: `nix run ~/eng/devenvs/nix#fmt -- flake.nix`

**Step 7: Run validation tests**

```bash
just test-validate-repos
```

**Step 8: Commit**

```bash
git add flake.nix flake.lock marketplace-config.json
git commit -m "Add robin (batman BATS skill) to marketplace"
```
