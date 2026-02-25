---
name: "Go + Nix Monorepo: Workspace Build Pattern"
description: >-
  This skill should be used when the user asks to "build a Go package in a nix
  monorepo", "fix vendorHash for local dependencies", "migrate from gomod2nix",
  "use go work vendor with nix", "add a Go package to a monorepo flake", "share
  vendorHash across packages", "fix go-mcp dependency in nix build", or
  encounters vendorHash churn from local Go library changes, stale
  gomod2nix.toml files, or Go module resolution failures in Nix monorepo builds.
  Also applies when a Go package depends on a local library or another package
  in the same monorepo and the nix build can't resolve it, when encountering
  "not marked as explicit in vendor/modules.txt" errors, or when the user wants
  one vendor hash stable across local code changes.
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

The local `vendor/` directory (produced by `go work vendor` for IDE support and
local builds) must be gitignored --- it is a build artifact, not checked-in
source. Add `vendor/` to `.gitignore`.

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

Any workspace module that depends on another workspace module needs a `replace`
directive in its `go.mod`. This applies to **all** intra-workspace dependencies
--- packages depending on libraries, and packages depending on other packages.

```go
// packages/grit/go.mod — package depending on a library
module github.com/org/grit

go 1.25.6

require github.com/org/monorepo/libs/go-mcp v0.0.3-0.20260222205500-abcdef123456

replace github.com/org/monorepo/libs/go-mcp => ../../libs/go-mcp
```

```go
// packages/spinclass/go.mod — package depending on another package
module github.com/org/spinclass

go 1.25.6

require github.com/org/monorepo/packages/tap-dancer/go v0.0.0-20260222022802-be680fd2b4ac

replace github.com/org/monorepo/packages/tap-dancer/go => ../tap-dancer/go
```

**Why replace directives are required:** `go work vendor` uses them to resolve
workspace modules locally without hitting the module proxy. Without them,
Go tries to fetch unpublished pseudo-versions from the proxy and fails. This
applies to every intra-workspace dependency, whether the target is under `libs/`
or `packages/`.

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

Run `just vendor && just vendor-hash` --- this regenerates the local `vendor/`
directory, then hashes it with `nix hash path` and writes the result to
`flake.nix`. The local vendor output matches what nix's fixed-output derivation
produces, so the hash is identical.

Manually, you can also use the dummy-hash approach:

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

1. Create `packages/new-pkg/` with `go.mod` including `replace` directives for
   every workspace module it depends on (libraries under `libs/` **and** other
   packages under `packages/`)
2. Add `use ./packages/new-pkg` to `go.work`
3. Create `lib/packages/new-pkg.nix` using the workspace pattern (copy grit.nix,
   change `pname`, `subPackages`, `postInstall`)
4. Add the import to `flake.nix` passing `goWorkspaceSrc` and `goVendorHash`
5. Build --- the existing `goVendorHash` works unless the new package adds
   external dependencies

## Adding a Cross-Package Dependency

When an existing package adds a dependency on another workspace module:

1. Add the `require` to the package's `go.mod`
2. Add a `replace` directive pointing to the relative path of the dependency
3. Run `just vendor` (or `go work vendor`) to regenerate the vendor directory
4. Run `just vendor-hash` to recompute `goVendorHash` if the new dependency
   brings in new external transitive deps

## Per-Package Justfile

Each Go package should have a `justfile` for local development. The pattern
delegates to the root flake for builds and uses `nix develop` for Go tooling:

```just
# packages/new-pkg/justfile
root := justfile_directory() + "/../.."

default: build test

build:
    nix build {{root}}#new-pkg

test:
    nix develop {{root}} --command go test ./packages/new-pkg/...

fmt:
    nix develop {{root}} --command go fmt ./packages/new-pkg/...

lint:
    nix develop {{root}} --command go vet ./packages/new-pkg/...

clean:
    rm -rf result
```

**Key details:**

- `root` resolves to the monorepo root so `nix build` and `nix develop` find
  the flake
- `build` uses `nix build` with the package's flake output name
- `test`, `fmt`, `lint` use `nix develop --command` to get Go tooling from the
  devshell without needing Go installed on the host
- All Go commands use workspace-relative paths (`./packages/new-pkg/...`)

The root justfile should also have workspace-wide recipes for vendor management:

```just
# Root justfile (relevant recipes)

# Regenerate workspace vendor directory after dependency changes
vendor:
    nix develop --command go work vendor

# Update go dependencies, tidy all modules, and re-vendor
deps:
    nix develop --command go work sync
    nix develop --command go work vendor

# Recompute goVendorHash in flake.nix from the local vendor directory
vendor-hash:
    #!/usr/bin/env bash
    set -euo pipefail
    # Hash the vendor directory — matches what nix's fixed-output derivation produces
    hash=$(nix hash path vendor/)
    # Update goVendorHash in flake.nix
    sed -i '' -E 's|(goVendorHash = )"sha256-[^"]+";|\1"'"$hash"'";|' flake.nix
    echo "updated goVendorHash to $hash"
```

- `just vendor` --- quick re-vendor after adding a `replace` directive
- `just deps` --- full sync + re-vendor when external dependencies change
- `just vendor-hash` --- hash the local `vendor/` directory with `nix hash path`
  and write it to `flake.nix` (the local vendor output matches what nix's
  fixed-output derivation produces, so the hash is identical)

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
- **Removing or omitting replace directives** --- `go work vendor` needs them
  for every intra-workspace dependency (libraries and packages alike). Missing a
  `replace` causes "not marked as explicit in vendor/modules.txt" build failures.
- **Setting `GOWORK=off` in the main build** --- workspace mode is needed for
  the build phase too, so workspace modules resolve via `go.work` use directives.
- **Using `vendorHash = null`** --- only valid when a checked-in `vendor/`
  directory exists. Use a real hash with workspace vendor.
- **Checking in `vendor/`** --- the local `vendor/` directory is a build
  artifact from `go work vendor`, not source. The nix build runs
  `go work vendor -e` in its own sandbox via `overrideModAttrs`. Add `vendor/`
  to `.gitignore`.

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
