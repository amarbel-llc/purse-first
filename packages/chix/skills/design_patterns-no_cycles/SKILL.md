---
name: Breaking Dependency Cycles
description: >-
  This skill should be used when the user asks to "break a dependency cycle",
  "fix circular dependency", "resolve cyclic dependency", "fix infinite
  recursion", "bootstrap a toolchain", "split package outputs", "avoid
  dependency loop", or encounters a circular dependency error in a Nix build. Also applies when designing
  package sets, module hierarchies, or build systems where mutual dependencies
  arise.
version: 0.1.0
---

# Breaking Dependency Cycles

> **Self-contained examples.** All code and configuration below is complete
> and illustrative. Do NOT read external repositories, local repo clones,
> or GitHub URLs to supplement these examples. Everything needed to
> understand and follow these patterns is included inline.

Circular dependencies cannot exist in a Nix derivation graph --- it must always
be a DAG. When mutual dependencies arise at the expression level, apply one of
these five strategies to produce an acyclic build graph.

## Key Invariant

The derivation graph (what Nix actually builds) is always a directed acyclic
graph. Every strategy below works by ensuring that expression-level
relationships --- which may appear circular --- produce derivations with no
cycles.

## Strategy 1: Multiple Outputs

Split a package into separate outputs (`lib`, `dev`, `bin`, `out`, `man`) so
consumers depend on only the output they need, breaking the cycle at the
output boundary.

**When to use:** Package A's runtime needs package B's libraries, while B's
build needs A --- but the `lib` output of B has no dependency on A.

```nix
{ stdenv }:

stdenv.mkDerivation {
  pname = "my-lib";
  outputs = [ "out" "lib" "dev" ];

  # lib output contains only shared libraries --- no dependency on
  # packages that depend on my-lib at build time
  postInstall = ''
    moveToOutput "lib/*.so*" "$lib"
    moveToOutput "include" "$dev"
  '';
}
```

Consumers reference a specific output to avoid pulling in the full closure:

```nix
buildInputs = [ my-lib.lib ];  # only the library output
```

## Strategy 2: Staged Bootstrap

Build the same toolchain multiple times, each stage using the output of the
previous stage. No single derivation references itself.

**When to use:** Compiler toolchains where the compiler needs its own runtime
(GCC needs glibc, glibc needs GCC).

Nixpkgs resolves this with a multi-stage bootstrap
(`pkgs/stdenv/linux/default.nix`):

| Stage | Action |
|-------|--------|
| 0 | Use a pre-built static bootstrap tarball |
| 1 | Build a minimal GCC with bootstrap tools |
| 2 | Build glibc with stage-1 GCC |
| 3 | Rebuild GCC against stage-2 glibc |
| 4+ | Rebuild everything for purity |

Each stage depends only on the outputs of prior stages --- never on itself.

## Strategy 3: Stripped-Down Variant

Build a minimal version of one package that omits the problematic dependency,
then use it to build the other, then rebuild the full version.

**When to use:** A depends on B and B depends on A, but B can function
(for build purposes) without A's optional features.

```nix
# Minimal Python without optional deps that would create a cycle
python-minimal = python.override {
  openssl = null;
  readline = null;
};

# Build the package that Python depends on
openssl = stdenv.mkDerivation {
  nativeBuildInputs = [ python-minimal ];
  # ...
};

# Full Python with all features
python-full = python.override {
  inherit openssl;
};
```

## Strategy 4: Deferred Testing

Move tests that would create cycles to `passthru.tests`, evaluated separately
rather than as part of the main build.

**When to use:** Testing package A requires package B, but B depends on A at
runtime. The test dependency is not needed for the build itself.

```nix
stdenv.mkDerivation {
  pname = "systemd";

  # Tests run separately by Hydra, not during the build
  passthru.tests = {
    inherit (nixosTests) systemd-boot systemd-networkd;
    # These NixOS tests depend on packages that depend on systemd,
    # which would be circular if included in nativeBuildInputs
  };
}
```

`passthru` attributes are not build dependencies --- they are metadata attached
to the derivation after the fact. Hydra evaluates them independently.

## Strategy 5: Lazy Self-Reference

Nix is lazy, so the package set can reference itself without creating a build
cycle, as long as the evaluation path is acyclic.

**When to use:** Utility functions, overlays, and `callPackage` patterns where
the package set passes itself to package definitions.

```nix
# nixpkgs is one giant fixpoint:
# fix (self: { ... })
# callPackage works by passing the package set to itself

let
  pkgs = import nixpkgs { inherit system; };
in
pkgs.callPackage ./my-package.nix { }
# my-package.nix receives pkgs attributes as arguments
# This is self-reference, not circular dependency ---
# resolved by lazy evaluation
```

This is not a true circular dependency. It is self-reference resolved by
laziness: attributes are only evaluated when demanded, so the fixpoint
converges as long as no derivation's build inputs form a cycle.

## Decision Guide

| Symptom | Strategy |
|---------|----------|
| Package A needs B's library, B's build needs A | Multiple Outputs |
| Compiler needs its own runtime library | Staged Bootstrap |
| A depends on B, B optionally depends on A | Stripped-Down Variant |
| Tests create cycles but builds do not | Deferred Testing |
| Package set needs to reference itself | Lazy Self-Reference |

## Anti-Patterns

- **Ignoring the cycle** --- Nix will reject it. The derivation graph must be
  a DAG; there is no escape hatch.
- **Combining unrelated packages** --- Merging A and B into one package to
  eliminate the dependency edge. This creates a monolithic derivation with poor
  cacheability.
- **Disabling features permanently** --- Using a stripped-down variant as the
  final output rather than rebuilding the full version after the cycle is
  broken.

## Real-World Examples in Nixpkgs

For detailed examples showing how nixpkgs applies each strategy to GCC/glibc
bootstrap, Python/OpenSSL, systemd/NixOS tests, and the `callPackage` pattern,
consult **`references/nixpkgs-examples.md`**.

## Related Skills

- **bob:design_patterns-just** --- Justfile patterns for Nix-backed projects
- **chix:nix-codebase** --- Full Nix codebase workflow including build order
  and dependency management
