---
name: "Go + Nix Monorepo: Workspace Build Pattern"
description: >-
  This skill should be used when the user asks to "build a Go package in a nix
  monorepo", "fix vendorHash for local dependencies", "migrate from gomod2nix",
  "use go work vendor with nix", "add a Go package to a monorepo flake", "share
  vendorHash across packages", "fix go-mcp dependency in nix build", or
  encounters vendorHash churn from local Go library changes, stale
  gomod2nix.toml files, or Go module resolution failures in Nix monorepo builds.
  Also applies when a Go package depends on a local library in the same monorepo
  and the nix build can't resolve it, or when the user wants one vendor hash
  stable across local code changes.
version: 0.2.0
---

# Go + Nix Monorepo: Workspace Build Pattern

> **Self-contained examples.** All code and configuration below is complete
> and illustrative. Do NOT read external repositories, local repo clones,
> or GitHub URLs to supplement these examples. Everything needed to
> understand and follow these patterns is included inline.

Go packages in a Nix monorepo that depend on local libraries need special
handling because Nix builds run in isolated sandboxes. This skill documents
the **workspace build pattern** using `go work vendor` with `buildGoModule`,
which provides a single stable `vendorHash` across all Go packages that only
changes when external dependencies change.

## The Problem

A Go monorepo uses `go.work` for local development, but `nix build` runs in a
sandbox. Three challenges arise:

1. **Local module resolution** --- Go needs to find workspace modules (e.g.,
   `libs/go-mcp`) during the nix build
2. **Vendor hash stability** --- changing local library code shouldn't require
   updating vendor hashes
3. **External consumer compatibility** --- `go get` from outside the monorepo
   should work normally

### Why Previous Approaches Fall Short

| Approach | Problem |
|----------|---------|
| `buildGoApplication` (gomod2nix) | Requires publishing library changes before building; stale `.toml` breaks builds |
| `buildGoModule` + per-package combined source | `vendorHash` covers local replace targets; any local code change invalidates ALL dependent hashes |

Both approaches create a publish-before-build bottleneck or constant hash churn.

## The Solution: Workspace Vendor Build

Use `go work vendor` (Go 1.22+) inside `buildGoModule`'s vendor derivation.
This vendors only external dependencies --- workspace modules stay in the source
tree and are never hashed into the vendor output.

**Key insight:** `go work vendor` produces a `vendor/` directory that contains
only third-party code. Workspace modules are listed in `vendor/modules.txt` with
redirect markers (`=> ./libs/go-mcp`) but no vendored files. This means the
`vendorHash` is determined entirely by external dependencies.

### Architecture

```
flake.nix
  |
  +-- goWorkspaceSrc (filtered: .go, go.mod, go.sum, go.work)
  +-- goVendorHash   (ONE hash, covers only external deps)
  |
  +-- lib/packages/grit.nix      --> buildGoModule { subPackages = ["packages/grit/cmd/grit"]; }
  +-- lib/packages/lux.nix       --> buildGoModule { subPackages = ["packages/lux/cmd/lux"]; }
  +-- lib/packages/get-hubbed.nix --> buildGoModule { subPackages = ["packages/get-hubbed/cmd/get-hubbed"]; }
```

All Go packages share the same `src`, `vendorHash`, and `overrideModAttrs`.
They differ only in `subPackages` and `postInstall`.

## The Pattern

### 1. Go workspace and module setup

```
# go.work (repo root)
go 1.25.6

use (
    .
    ./libs/go-mcp
    ./packages/grit
    ./packages/lux
)
```

Each package's `go.mod` has a `replace` directive for local resolution:

```go
// packages/grit/go.mod
module github.com/org/grit

go 1.25.6

require github.com/org/monorepo/libs/go-mcp v0.0.3-0.20260222205500-abcdef123456

replace github.com/org/monorepo/libs/go-mcp => ../../libs/go-mcp
```

**Why replace directives are required:** `go work vendor` uses them to resolve
workspace modules locally without hitting the module proxy. Without them,
Go tries to fetch unpublished pseudo-versions from the proxy and fails.

**External consumers never see them:** `replace` directives are module-local.
Running `go get github.com/org/grit` resolves `go-mcp` from the module proxy
at its published version, not through the replace directive.

### 2. Filtered Go source in flake.nix

```nix
# flake.nix (top-level let block)
goWorkspaceSrc = nixpkgs.lib.cleanSourceWith {
  src = ./.;
  filter =
    path: type:
    let baseName = builtins.baseNameOf path; in
    type == "directory"
    || nixpkgs.lib.hasSuffix ".go" baseName
    || baseName == "go.mod"
    || baseName == "go.sum"
    || baseName == "go.work"
    || baseName == "go.work.sum"
    # Add non-Go files needed by postInstall (e.g., scdoc for man pages)
    || nixpkgs.lib.hasSuffix ".scd" baseName;
};

# Single vendor hash for the entire Go workspace.
# Only covers external deps — workspace module changes don't affect it.
goVendorHash = "sha256-sjmgbpHFlLbyNWyC9pmetNDs+n0xO03+jy/xVFO/Sl4=";
```

The source filter includes all directories (so the tree structure is preserved)
plus Go-relevant files. Non-Go packages (Rust, bash, etc.) end up as empty
directories, which Go ignores.

### 3. Per-package nix expression

```nix
# lib/packages/grit.nix
{
  pkgs,
  goWorkspaceSrc,
  goVendorHash,
}:

pkgs.buildGoModule {
  pname = "grit";
  version = "0.1.0";
  src = goWorkspaceSrc;
  vendorHash = goVendorHash;

  # Enable workspace mode (buildGoModule defaults to GOWORK=off)
  GOWORK = "";

  overrideModAttrs = _: _: {
    GOWORK = "";
    buildPhase = ''
      runHook preBuild
      go work vendor -e
      runHook postBuild
    '';
  };

  subPackages = [ "packages/grit/cmd/grit" ];

  postInstall = ''
    $out/bin/grit generate-plugin $out
  '';

  meta = with pkgs.lib; {
    description = "MCP for git";
    license = licenses.mit;
  };
}
```

**Critical details:**

- `GOWORK = ""` on both the main derivation and the vendor derivation
  (`overrideModAttrs`) --- `buildGoModule` sets `GOWORK=off` by default,
  which disables workspace resolution
- `overrideModAttrs` replaces the vendor phase to use `go work vendor -e`
  instead of the default `go mod vendor`
- `subPackages` uses workspace-relative paths (e.g., `packages/grit/cmd/grit`)
- No `sourceRoot` --- the source root IS the workspace root

### 4. Wire in flake.nix

```nix
# flake.nix (in buildPackages)
gritPkg = import ./lib/packages/grit.nix {
  inherit pkgs goWorkspaceSrc goVendorHash;
};

luxPkg = import ./lib/packages/lux.nix {
  inherit pkgs goWorkspaceSrc goVendorHash;
};
```

All Go packages receive the same `goWorkspaceSrc` and `goVendorHash`.

## Computing `vendorHash`

When you first write the nix expression or change external Go dependencies:

1. Set `vendorHash` to a dummy value:
   ```nix
   goVendorHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
   ```

2. Build any Go package. Nix fails with the correct hash:
   ```
   error: hash mismatch in fixed-output derivation:
     specified: sha256-AAAAAAA...
        got:    sha256-sjmgbpH...
   ```

3. Replace `goVendorHash` with the correct hash. All packages use it.

**When to update:** Only when external dependencies change (adding/updating
packages in any workspace module's `go.mod`). Local code changes to workspace
modules (go-mcp, grit, lux, etc.) never affect the hash.

## Adding a New Go Package

1. Create `packages/new-pkg/` with `go.mod` including a `replace` directive
2. Add `use ./packages/new-pkg` to `go.work`
3. Create `lib/packages/new-pkg.nix` using the workspace pattern (copy grit.nix,
   change `pname`, `subPackages`, `postInstall`)
4. Add the import to `flake.nix` passing `goWorkspaceSrc` and `goVendorHash`
5. Build --- the existing `goVendorHash` works unless the new package adds
   external dependencies

## Mixed-Language Packages

For packages with both Go and non-Go components (e.g., tap-dancer has Go + Rust
+ bash), only the Go CLI uses the workspace build. Non-Go components keep their
own source and build system:

```nix
# lib/packages/tap-dancer.nix
{ pkgs, src, craneLib, goWorkspaceSrc, goVendorHash }:

let
  # Go CLI uses workspace build
  tap-dancer-cli = pkgs.buildGoModule {
    src = goWorkspaceSrc;
    vendorHash = goVendorHash;
    GOWORK = "";
    overrideModAttrs = _: _: {
      GOWORK = "";
      buildPhase = ''
        runHook preBuild
        go work vendor -e
        runHook postBuild
      '';
    };
    subPackages = [ "packages/tap-dancer/go/cmd/tap-dancer" ];
  };

  # Rust component uses crane with its own source
  tap-dancer-rust = craneLib.buildPackage {
    src = craneLib.cleanCargoSource "${src}/rust";
  };
in
{
  default = pkgs.symlinkJoin {
    name = "tap-dancer";
    paths = [ tap-dancer-cli tap-dancer-rust ];
  };
}
```

## Anti-Patterns

- **Per-package vendorHash** --- defeats the purpose. One hash for the whole
  workspace. If you have separate hashes, you're back to hash churn.
- **Per-package combined source trees** --- the old pattern. Assembling
  `runCommand "grit-src" ...` per package means vendorHash includes local code
  and changes with every library edit.
- **Removing replace directives** --- `go work vendor` needs them to resolve
  workspace modules without hitting the module proxy.
- **Setting `GOWORK=off` in the main build** --- workspace mode is needed for
  the build phase too, so workspace modules resolve via `go.work` use directives.
- **Using `vendorHash = null`** --- only valid when a checked-in `vendor/`
  directory exists. Use a real hash with workspace vendor.

## Decision Guide

| Scenario | Approach |
|----------|----------|
| Go monorepo with shared local libraries | Workspace build (this pattern) |
| Go package with only external dependencies | Plain `buildGoModule` with `vendorHash` |
| Must pin exact dependency versions for audit | `buildGoApplication` + `gomod2nix.toml` |
| Standalone Go binary, zero dependencies | `buildGoModule` with `vendorHash = null` + checked-in vendor |

## Additional Resources

### Reference Files

For a detailed before/after showing the migration:
- **`references/purse-first-example.md`** --- Migration from gomod2nix through
  combined-source to workspace build

### Related Skills

- **chix:design_patterns-flake_monorepo** --- Broader monorepo flake pattern
  (sub-component imports, thin wrappers, no path inputs)
- **chix:nix-codebase** --- Full Nix codebase workflow including dependency
  management
- **bob:go-cli-framework** --- Building Go MCP servers with go-mcp
