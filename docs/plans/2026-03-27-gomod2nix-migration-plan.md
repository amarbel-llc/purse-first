# Migrate purse-first Go build from buildGoModule to gomod2nix

## Problem

The current build uses `buildGoModule` with a manually-managed `goVendorHash`.
Adding/removing external Go deps requires running `just vendor-hash` to
recompute the hash. gomod2nix generates a deterministic lockfile
(`gomod2nix.toml`) that tracks individual dependencies, eliminating the opaque
hash.

## Key constraint

gomod2nix does not support `go.work`. The root module currently resolves
`libs/go-mcp` via the workspace. We need `replace` directives in root `go.mod`
so gomod2nix can resolve local libs without `go.work`.

## Changes

### 1. Add `replace` directive to root `go.mod`

    replace github.com/amarbel-llc/purse-first/libs/go-mcp => ./libs/go-mcp

Only this one replace is needed --- the root module only imports `libs/go-mcp`,
not `libs/go-mcp/command/huh`.

### 2. Generate `gomod2nix.toml`

Run `gomod2nix` in the repo root to generate the lockfile from
`go.mod`/`go.sum`. This file gets checked in.

### 3. Add justfile recipe `build-go-gomod2nix`

``` just
build-go-gomod2nix:
    {{cmd_nix_dev}} gomod2nix
```

Make it a dependency of `build`:

``` just
build: build-go-gomod2nix
    nix build
```

### 4. Update `lib/mkGoWorkspaceModule.nix` -\> use `buildGoApplication`

Replace `pkgs.buildGoModule` with gomod2nix's `buildGoApplication`. This
requires the gomod2nix overlay to be applied to pkgs. Key changes:

- Remove `vendorHash` / `goVendorHash` parameter
- Remove `overrideModAttrs` (no more `go work vendor`)
- Add `modules` parameter pointing to `gomod2nix.toml`
- Keep `GOWORK = "";` for workspace-aware compilation (the build still uses
  go.work for local module resolution at compile time)

New signature:

``` nix
{ pkgs, goWorkspaceSrc }:
attrs:
pkgs.buildGoApplication ({
  version = "0.1.0";
  src = goWorkspaceSrc;
  modules = goWorkspaceSrc + "/gomod2nix.toml";
} // attrs)
```

### 5. Update `flake.nix`

- Apply gomod2nix overlay to pkgs:
  `pkgs = import nixpkgs { inherit system; overlays = [ gomod2nix.overlays.default ]; };`
- Remove `goVendorHash` binding
- Update `purse-first-build` to no longer pass `goVendorHash`
- Add `gomod2nix.toml` to `goWorkspaceSrc` filter
- Pass gomod2nix overlay through to mkMarketplace

### 6. Update `lib/mkMarketplace.nix`

- Remove `goVendorHash` from `purse-first-build` destructuring
- Apply gomod2nix overlay to the pkgs used for Go builds

### 7. Remove `just vendor-hash` recipe

No longer needed --- replaced by `build-go-gomod2nix`.

### 8. Update CLAUDE.md

Remove references to `goVendorHash` and `just vendor-hash`. Document
`just build-go-gomod2nix` and that `gomod2nix.toml` must be regenerated after
dependency changes.

## Files modified

1.  `go.mod` --- add replace directive
2.  `gomod2nix.toml` --- new file (generated)
3.  `justfile` --- add `build-go-gomod2nix`, make it dep of `build`, remove
    `vendor-hash`
4.  `lib/mkGoWorkspaceModule.nix` --- switch to `buildGoApplication`
5.  `flake.nix` --- remove `goVendorHash`, apply overlay, update source filter
6.  `lib/mkMarketplace.nix` --- remove `goVendorHash` from purse-first-build
7.  `CLAUDE.md` --- update build docs

## Open question: go.work support in gomod2nix

gomod2nix has no open issue or PR for `go.work` support. If it lands upstream,
the `replace` directive workaround can be removed and `go.work` used natively.
See research notes in the commit that added this plan.
