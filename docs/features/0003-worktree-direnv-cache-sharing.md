---
status: exploring
date: 2026-03-01
promotion-criteria: prototype a nix-built devshell loaded without nix-direnv, measure against baseline
---

# Fast direnv for worktrees

## Motivation

nix-direnv caches `nix print-dev-env` output per-directory. In git worktrees,
each fresh `.direnv/` triggers a full rebuild even when the flake is
byte-identical to the main checkout. Measured in purse-first (a light flake):

| Scenario | `direnv exec . true` |
|----------|---------------------|
| Main checkout (warm cache) | 0.05s |
| Fresh worktree (cold cache, store has outputs) | 6.5s |
| `nix print-dev-env` alone | 0.75s |

The 5.7s gap is nix-direnv overhead plus the `source_up` chain (5 `.envrc`
files up to `$HOME`, each independently evaluating). Heavier flakes and deeper
chains compound.

The worktree problem is a symptom of a broader issue: nix-direnv adds a
path-based caching layer on top of nix's content-addressed store. This caching
layer doesn't understand that two directories with the same flake.lock produce
the same environment.

## Alternative: nix-built devshell without nix-direnv

Instead of caching `nix print-dev-env` output, build the devshell as a nix
derivation and source it directly:

- The devshell is a store path. Identical flakes produce the same path — no
  per-directory cache needed.
- Loading is sourcing a file, not evaluating nix — instant.
- The `source_up` chain could be collapsed: compose environments at the nix
  level (one flake imports what it needs) instead of N independent evaluations.
- Staleness detection: direnv `watch_file` on `flake.lock` triggers a
  `nix build`, or accept the ~0.75s eval cost every time and skip caching.

## Interface

TBD.

## Examples

TBD.

## Limitations

- Rebuilding the devshell still requires `nix build` — the question is whether
  the common case (no change) can be made instant without nix-direnv's cache.

## Open Questions

1. **`nix print-dev-env` vs `nix build`**: can a devshell derivation be sourced
   directly, or does `nix print-dev-env` do transformations that a raw build
   output doesn't?

2. **`source_up` composition**: if environments are composed at the nix level
   instead of chained via `source_up`, what breaks? Some `.envrc` files do
   non-nix work (PATH_add, env vars, secrets).

3. **Staleness without nix-direnv**: is `watch_file flake.lock` + `nix build`
   fast enough, or do we need our own lightweight cache check? `nix build`
   with a cached result should be sub-second.

4. **Scope**: is this a direnvrc library, a spinclass feature, or a standalone
   tool that replaces nix-direnv?

5. **Existing work**: has anyone else built this? devenv.sh, numtide/devshell,
   or other projects may have solved this already.

## More Information

- Observed in purse-first worktrees created by spinclass
- Measurements taken 2026-03-01 on x86_64-linux
