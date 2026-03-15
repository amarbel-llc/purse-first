---
status: accepted
date: 2026-02-25
---

# Convert Devenvs from Sub-Flakes to Plain Nix Functions

## Context and Problem Statement

Devenv sub-flakes used `path:` inputs for cross-referencing (e.g., `url = "path:./devenvs/go"`). When a downstream flake consumes purse-first transitively, Nix resolves these relative paths against the wrong source tree, breaking `nix flake update` for all transitive consumers ([NixOS/nix#14762](https://github.com/nixos/nix/issues/14762)).

## Considered Options

* Keep sub-flakes with `path:` inputs
* Convert to plain Nix functions called from the top-level flake
* Use flake-parts

## Decision Outcome

Chosen option: "Convert to plain Nix functions", because it eliminates the broken `path:` input resolution while keeping each devenv self-contained and independently usable via a thin `flake.nix` wrapper, accepting that devenv-specific inputs must be hoisted to the top-level flake.

Each devenv becomes a `default.nix` function taking concrete values (`pkgs`, `pkgs-master`, and any devenv-specific args like `gomod2nix`). A companion `flake.nix` wrapper imports `default.nix` for standalone `nix develop` and direnv use. The top-level `flake.nix` removes all `path:./devenvs/*` inputs and calls `import ./devenvs/<name>` directly.

**Naming convention (2026-03-15):** `default.nix` returns `{ devShells.default = ...; }` (not `{ devShell = ...; }`). This mirrors the flake output schema so the pattern is identical at every level --- `default.nix`, `flake.nix` wrappers, monorepo `flake.nix`, and downstream consumers all use `devShells.default`.

### Consequences

* Good, because transitive flake consumers no longer hit NixOS/nix#14762 when resolving devenv inputs.
* Good, because `default.nix` functions have explicit parameter contracts --- missing arguments fail immediately with clear Nix error messages.
* Good, because standalone `nix develop ./devenvs/go` and direnv continue to work via the thin `flake.nix` wrapper.
* Bad, because devenv-specific inputs (e.g., `gomod2nix`) must be hoisted to the top-level flake, increasing its input count.
* Bad, because consumers must migrate from `purse-first?dir=devenvs/go` to `purse-first.devShells.${system}.go`.
* Good, because `devShells.default` naming is consistent from `default.nix` through flake outputs, eliminating confusion between the monorepo internal pattern and the flake output pattern.

## More Information

See [docs/plans/2026-02-23-devenvs-as-nix-functions-design.md](../plans/2026-02-23-devenvs-as-nix-functions-design.md) for the full design including the `default.nix` contract, consumer migration examples, and eng repo follow-up plan.
