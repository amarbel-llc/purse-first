# Nixpkgs Circular Dependency Examples

Detailed examples of how nixpkgs applies each cycle-breaking strategy in
practice.

## Multiple Outputs: glib and GObject Introspection

glib needs gobject-introspection for generating typelib files, but
gobject-introspection depends on glib. Nixpkgs splits glib into outputs:

```nix
# pkgs/development/libraries/glib/default.nix (simplified)
stdenv.mkDerivation {
  pname = "glib";
  outputs = [ "out" "dev" "devdoc" ];

  # gobject-introspection is only in nativeBuildInputs when
  # the introspection output is requested, not in the base lib
  nativeBuildInputs = lib.optional introspectionSupport gobject-introspection;
}
```

Packages that only need glib's shared libraries use `glib.out`, which has no
dependency on gobject-introspection. The cycle only exists when introspection
support is enabled, and even then the `out` output is cycle-free.

## Staged Bootstrap: GCC, glibc, and binutils

The Linux stdenv bootstrap in `pkgs/stdenv/linux/default.nix` proceeds through
multiple stages. A simplified view:

```
Stage 0 (bootstrap tarball):
  - Pre-built: busybox, a static GCC, a static glibc, binutils
  - No Nix derivations involved --- these are fetched from a fixed URL

Stage 1 (build with bootstrap tools):
  - gcc-stage1 = build GCC using stage-0 tools
  - binutils-stage1 = build binutils using stage-0 tools

Stage 2 (build glibc with stage-1 compiler):
  - glibc = build glibc using gcc-stage1 and binutils-stage1

Stage 3 (rebuild GCC against real glibc):
  - gcc = build GCC linked against stage-2 glibc

Stage 4 (rebuild everything for purity):
  - stdenv = rebuild all of stdenv using stage-3 GCC + stage-2 glibc
```

Each stage produces distinct derivations. There is never a cycle because
stage N only depends on stages 0..N-1.

## Stripped-Down Variant: Python and OpenSSL

Python optionally depends on OpenSSL (for `ssl`, `hashlib` modules). Some
packages that OpenSSL's tests use depend on Python. Nixpkgs breaks this:

```nix
# Simplified from pkgs/development/interpreters/python/
python-minimal = python3.override {
  openssl = null;         # no SSL support
  readline = null;        # no interactive shell
  self = python-minimal;  # self-reference for sub-packages
};

# OpenSSL can now use python-minimal for its build scripts
openssl = callPackage ./openssl.nix {
  python3 = python-minimal;
};

# Full Python is built with the real OpenSSL
python3 = callPackage ./python.nix {
  inherit openssl;
};
```

The key insight: `python-minimal` has no dependency on `openssl`, so
`openssl -> python-minimal` does not create a cycle. The full `python3`
depends on `openssl` but `openssl` does not depend on the full `python3`.

## Deferred Testing: systemd

systemd is a core dependency of most NixOS services. Many NixOS integration
tests depend on services that depend on systemd. Including those tests as
build-time checks would create massive cycles.

```nix
# pkgs/os-specific/linux/systemd/default.nix (simplified)
stdenv.mkDerivation {
  pname = "systemd";

  # Build inputs --- no NixOS tests here
  buildInputs = [ util-linux libcap kmod ];

  # Tests evaluated separately by Hydra
  passthru.tests = {
    # Each of these NixOS tests builds a full VM that includes
    # systemd, creating a dependency on this derivation.
    # If these were in checkPhase, it would be circular.
    inherit (nixosTests)
      systemd-boot
      systemd-networkd
      systemd-resolved
      ;
  };
}
```

Hydra evaluates `passthru.tests` in a separate evaluation, not as part of the
systemd build. The test derivations depend on systemd, but systemd's build
derivation does not depend on the tests.

## Lazy Self-Reference: callPackage and Overlays

The entire nixpkgs package set is constructed as a fixpoint:

```nix
# lib/fixed-points.nix (simplified)
fix = f: let x = f x; in x;

# pkgs/top-level/default.nix (simplified)
pkgs = fix (self: {
  hello = self.callPackage ./pkgs/hello { };
  curl = self.callPackage ./pkgs/curl { };
  # ...thousands more
});
```

`callPackage` automatically passes attributes from `self` (the package set)
to each package function:

```nix
# callPackage resolves function arguments from pkgs
# pkgs/applications/networking/curl/default.nix
{ lib, stdenv, openssl, zlib, ... }:
stdenv.mkDerivation {
  pname = "curl";
  buildInputs = [ openssl zlib ];
}
```

This looks circular (`pkgs` defines `curl`, `curl` references `pkgs` via
`callPackage`), but Nix's lazy evaluation means:

1. `pkgs.curl` is a thunk until evaluated
2. Evaluating it calls the function with `openssl`, `zlib`, etc. from `pkgs`
3. Those are also thunks, evaluated only when their derivation is built
4. The derivation graph is always acyclic --- only the expression graph has
   self-reference

Overlays follow the same pattern with `final`/`prev` (older code uses
`self`/`super`):

```nix
overlay = final: prev: {
  curl = prev.curl.overrideAttrs (old: {
    # final.openssl is the overlayed version
    buildInputs = [ final.openssl ];
  });
};
```

`self` is the final fixed-point, `super` is the previous layer. Lazy
evaluation ensures this converges without cycles.
