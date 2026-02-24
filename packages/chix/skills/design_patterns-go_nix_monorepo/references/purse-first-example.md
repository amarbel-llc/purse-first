# Purse-First Migration: Three Generations of Go + Nix Monorepo Builds

This reference traces the evolution of Go package builds in the purse-first
monorepo through three approaches, showing why the workspace build pattern is
the recommended solution.

## Generation 1: buildGoApplication (gomod2nix)

### lib/packages/grit.nix

```nix
{
  pkgs,
  src,
  goOverlay,
}:

let
  goPkgs = import pkgs.path {
    inherit (pkgs.stdenv.hostPlatform) system;
    overlays = [ goOverlay ];
  };
in
goPkgs.buildGoApplication {
  pname = "grit";
  version = "0.1.0";
  inherit src;
  modules = "${src}/gomod2nix.toml";
  subPackages = [ "cmd/grit" ];

  postInstall = ''
    $out/bin/grit generate-plugin $out
  '';
}
```

### flake.nix call

```nix
gritPkg = import ./lib/packages/grit.nix {
  inherit pkgs goOverlay;
  src = ./packages/grit;
};
```

**Fatal flaw:** `gomod2nix.toml` pins go-mcp at a published version. Local
changes to `libs/go-mcp` aren't picked up until published. Adding a new
function to go-mcp and using it in grit fails because the pinned version
doesn't have it.

## Generation 2: buildGoModule + Combined Source (Intermediate)

### lib/packages/grit.nix

```nix
{ pkgs, src, go-mcp-src }:

let
  combinedSrc = pkgs.runCommand "grit-src" { } ''
    mkdir -p $out/packages $out/libs
    cp -r ${src} $out/packages/grit
    cp -r ${go-mcp-src} $out/libs/go-mcp
  '';
in
pkgs.buildGoModule {
  pname = "grit";
  version = "0.1.0";
  src = combinedSrc;
  sourceRoot = "grit-src/packages/grit";
  vendorHash = "sha256-D+8x+FShxSi+5mWHKhHQEeHIMmpmJu8LXxYRBuPkjRc=";
  subPackages = [ "cmd/grit" ];

  postInstall = ''
    $out/bin/grit generate-plugin $out
  '';
}
```

### flake.nix call

```nix
gritPkg = import ./lib/packages/grit.nix {
  inherit pkgs;
  src = ./packages/grit;
  go-mcp-src = ./libs/go-mcp;
};
```

**Fatal flaw:** `vendorHash` covers ALL dependencies including local replace
targets. Changing any line in `libs/go-mcp` invalidates the vendor hash for
every dependent package. Nix caches the vendor derivation by hash, so stale
hashes silently produce builds with old code. Discovered when a fix to
`generate_hooks.go` wasn't picked up despite the source changing --- the
vendor derivation was still cached with the old go-mcp code.

## Generation 3: Workspace Build (Current)

### lib/packages/grit.nix

```nix
{ pkgs, goWorkspaceSrc, goVendorHash }:

pkgs.buildGoModule {
  pname = "grit";
  version = "0.1.0";
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

  subPackages = [ "packages/grit/cmd/grit" ];

  postInstall = ''
    $out/bin/grit generate-plugin $out
  '';
}
```

### flake.nix (shared config + per-package calls)

```nix
# Shared across all Go packages
goWorkspaceSrc = nixpkgs.lib.cleanSourceWith {
  src = ./.;
  filter = path: type:
    let baseName = builtins.baseNameOf path; in
    type == "directory"
    || nixpkgs.lib.hasSuffix ".go" baseName
    || baseName == "go.mod"
    || baseName == "go.sum"
    || baseName == "go.work"
    || baseName == "go.work.sum"
    || nixpkgs.lib.hasSuffix ".scd" baseName;
};

goVendorHash = "sha256-sjmgbpHFlLbyNWyC9pmetNDs+n0xO03+jy/xVFO/Sl4=";

# Per-package calls --- all share src and vendorHash
gritPkg = import ./lib/packages/grit.nix {
  inherit pkgs goWorkspaceSrc goVendorHash;
};

luxPkg = import ./lib/packages/lux.nix {
  inherit pkgs goWorkspaceSrc goVendorHash;
};

get-hubbed-unwrapped = import ./lib/packages/get-hubbed.nix {
  inherit pkgs goWorkspaceSrc goVendorHash;
};
```

### go.mod (unchanged from gen 2)

```go
module github.com/friedenberg/grit

go 1.25.6

require github.com/amarbel-llc/purse-first/libs/go-mcp v0.0.3-0.20260222205500-74480472530e

replace github.com/amarbel-llc/purse-first/libs/go-mcp => ../../libs/go-mcp
```

## Comparison

| Aspect | Gen 1: gomod2nix | Gen 2: Combined Source | Gen 3: Workspace Build |
|--------|-----------------|----------------------|----------------------|
| Builder | `buildGoApplication` | `buildGoModule` | `buildGoModule` |
| Dep mechanism | `gomod2nix.toml` | `replace` + per-pkg source assembly | `go work vendor` + shared source |
| Local lib changes | Requires publish | Picked up, but hash changes | Picked up, hash stable |
| Hash count | 0 (gomod2nix handles) | 1 per package | 1 total |
| Hash changes when... | N/A | Any local dep code changes | Only external dep changes |
| Source per package | Package dir only | Package + deps assembled | Full monorepo (filtered) |
| Flake args | `goOverlay`, `src` | `src`, `go-mcp-src` | `goWorkspaceSrc`, `goVendorHash` |

## Verified Properties

Tested during the purse-first migration:

1. **Hash stability** --- Modified `libs/go-mcp/command/generate_hooks.go`,
   rebuilt grit. Build succeeded with unchanged `goVendorHash`.
2. **External consumers** --- Created a standalone module that imports
   `go-mcp/command` and `go-mcp/protocol` via `go get`. Resolved `v0.0.2` from
   the module proxy. Built and ran successfully. Replace directives invisible.
3. **Full marketplace** --- All 7 packages (grit, lux, get-hubbed, tap-dancer,
   chix, robin, bob) build from the same workspace source and vendor hash.

## Files Changed (Gen 2 to Gen 3)

1. `flake.nix` --- Added `goWorkspaceSrc` filter and `goVendorHash`; updated
   all Go package calls from `src`/`go-mcp-src` to `goWorkspaceSrc`/`goVendorHash`
2. `lib/packages/grit.nix` --- Removed `combinedSrc`/`sourceRoot`; added
   `GOWORK=""` and `overrideModAttrs`
3. `lib/packages/lux.nix` --- Same transformation
4. `lib/packages/get-hubbed.nix` --- Same transformation
5. `lib/packages/tap-dancer.nix` --- Replaced inline `go mod vendor` with
   workspace build for Go CLI component
