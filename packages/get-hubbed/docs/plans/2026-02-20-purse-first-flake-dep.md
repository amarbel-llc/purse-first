# Purse-First as Direct Flake Dependency + install-local Testing

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the Go module fetch of `purse-first` with a direct Nix flake input using gomod2nix's `pwd` + `replace` directive pattern, then use `purse-first install-local` to install the built package for local testing.

**Architecture:** purse-first exposes its cleaned Go source as a flake output (`lib.goSrc`). get-hubbed adds purse-first as a flake input, constructs a composite source derivation that places the purse-first source at `deps/purse-first`, adds a `replace` directive in `go.mod`, and passes `pwd` to `buildGoApplication`. The purse-first entry is removed from `gomod2nix.toml` since locally-replaced modules are handled by the builder directly.

**Tech Stack:** Nix flakes, gomod2nix (`buildGoApplication` with `pwd` parameter), Go `replace` directives

**Repos touched:** `repos/purse-first` (expose source), `repos/get-hubbed` (consume source + fix content tools)

---

### Task 1: Expose Go source from purse-first flake

**Files:**
- Modify: `repos/purse-first/flake.nix:150-158`

**Step 1: Add `lib.goSrc` to purse-first flake outputs**

In `repos/purse-first/flake.nix`, the `purse-first-src` variable is already defined (lines 64-72) but not exposed. Add it to the `lib` output alongside `mkMarketplace`:

```nix
lib.mkMarketplace = mkMarketplace;
lib.goSrc = purse-first-src;
```

This replaces the existing:
```nix
lib.mkMarketplace = mkMarketplace;
```

**Step 2: Verify purse-first flake evaluates**

Run: `nix flake show repos/purse-first`
Expected: `lib.goSrc` and `lib.mkMarketplace` both appear under `lib`

**Step 3: Commit in purse-first**

```bash
cd repos/purse-first
git add flake.nix
git commit -m "feat: expose Go source as lib.goSrc for downstream flake consumers"
```

---

### Task 2: Add purse-first flake input to get-hubbed

**Files:**
- Modify: `repos/get-hubbed/flake.nix`

**Step 1: Add purse-first input and wire it through outputs**

Add to the `inputs` block:
```nix
purse-first = {
  url = "github:amarbel-llc/purse-first";
  inputs.nixpkgs.follows = "nixpkgs";
  inputs.nixpkgs-master.follows = "nixpkgs-master";
};
```

Add `purse-first` to the `outputs` function parameters.

**Step 2: Build composite source and update buildGoApplication**

In the `let` block, add a composite source derivation and update `get_hubbed`:

```nix
purse-first-go-src = purse-first.lib.goSrc;

get-hubbed-src = pkgs.runCommand "get-hubbed-src" {} ''
  cp -r ${./.} $out
  chmod -R u+w $out
  mkdir -p $out/deps
  cp -r ${purse-first-go-src} $out/deps/purse-first
'';
```

Update the `buildGoApplication` call to use `pwd`:

```nix
get_hubbed = pkgs.buildGoApplication {
  pname = "get-hubbed";
  inherit version;
  pwd = get-hubbed-src;
  src = get-hubbed-src;
  modules = ./gomod2nix.toml;
  subPackages = [ "cmd/get-hubbed" ];

  postInstall = ''
    $out/bin/get-hubbed generate-plugin $out/share/purse-first
  '';

  meta = with pkgs.lib; {
    description = "`gh` cli wrapper with MCP support packaged by nix";
    homepage = "https://github.com/friedenberg/get-hubbed";
    license = licenses.mit;
  };
};
```

**Step 3: Verify flake evaluates**

Run: `nix flake show repos/get-hubbed` (after locking — may need `nix flake lock` first)
Expected: No evaluation errors

---

### Task 3: Add replace directive and regenerate gomod2nix.toml

**Files:**
- Modify: `repos/get-hubbed/go.mod`
- Modify: `repos/get-hubbed/gomod2nix.toml`
- Modify: `repos/get-hubbed/go.sum`

**Step 1: Add replace directive to go.mod**

Append to `repos/get-hubbed/go.mod`:
```
replace github.com/amarbel-llc/purse-first => ./deps/purse-first
```

**Step 2: Remove purse-first entry from gomod2nix.toml**

Remove the `[mod.'github.com/amarbel-llc/purse-first']` block (lines 8-10) from `gomod2nix.toml`. gomod2nix skips locally-replaced modules, so this entry should not be present.

**Step 3: Run go mod tidy to update go.sum**

Run: `nix develop repos/get-hubbed --command go mod tidy` (from a directory where `./deps/purse-first` doesn't exist, this may error — that's expected for Nix-only replace directives)

Note: The replace directive targets `./deps/purse-first` which only exists inside the Nix build's composite source. For local development, use `go.work` instead (not checked into git or filtered by Nix builds). If `go mod tidy` fails due to the missing path, the gomod2nix.toml manual edit from Step 2 is sufficient for the Nix build.

**Step 4: Commit in get-hubbed**

```bash
cd repos/get-hubbed
git add go.mod gomod2nix.toml
git commit -m "feat: use purse-first as direct flake input via replace directive"
```

---

### Task 4: Build and verify get-hubbed

**Step 1: Lock the flake**

Run: `nix flake lock repos/get-hubbed`
Expected: `flake.lock` updated with purse-first input

**Step 2: Build the package**

Run: `nix build repos/get-hubbed`
Expected: Build succeeds, `./result/bin/get-hubbed` exists

**Step 3: Verify purse-first plugin manifest is generated**

Run: `ls ./result/share/purse-first/get-hubbed/plugin.json`
Expected: File exists

**Step 4: Run tests**

Run: `nix develop repos/get-hubbed --command go test ./...`
Expected: All tests pass (or report `[no test files]`)

**Step 5: Commit lock file**

```bash
cd repos/get-hubbed
git add flake.lock
git commit -m "chore: lock purse-first flake input"
```

---

### Task 5: Test locally with purse-first install-local

**Step 1: Run install-local from get-hubbed repo root**

Run: `cd repos/get-hubbed && purse-first install-local`

If `purse-first` is not on PATH, use the nix-built binary:
Run: `nix run github:amarbel-llc/purse-first -- install-local --root repos/get-hubbed`

Expected output (TAP-14):
```
1..3
ok 1 - discover and update skills in plugin.json
ok 2 - install MCP servers to .claude/settings.json (1 server)  # or skip if no mcpServers
ok 3 - install hooks to .claude/settings.json
```

**Step 2: Verify .claude/settings.json was updated**

Check that `.claude/settings.json` (project-scoped) contains the get-hubbed MCP server entry and hooks.

**Step 3: Verify the MCP server works**

Run: `echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.1.0"}}}' | ./result/bin/get-hubbed`
Expected: JSON response with server capabilities

---

### Task 6: Include content tools fix in the commit history

The `--method GET` fix to `internal/tools/content.go` is already applied in the working tree. Ensure it's included in the final commit set.

**Step 1: Verify the fix is present**

Run: `grep -n '"--method", "GET"' repos/get-hubbed/internal/tools/content.go`
Expected: 5 matches (content_tree, content_read, content_commits, content_compare, content_search)

**Step 2: Commit the fix separately (if not already committed)**

```bash
cd repos/get-hubbed
git add internal/tools/content.go
git commit -m "fix: add --method GET to content tools to prevent gh api implicit POST"
```
