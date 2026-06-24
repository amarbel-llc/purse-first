---
status: experimental
date: 2026-06-19
promotion-criteria: >
  proposed → experimental: `nix/linters/dewey-facade-export.nix` lands, is
  wired into conformistImpureEval, and `just lint-worktree` both detects facade
  drift (check) and repairs it (conformist repair / codemod-fmt). experimental →
  testing: the standalone `lint-dewey_pkgs_drift` recipe is retired (or reduced
  to a thin alias) and the merge gate still catches facade drift via the
  conformist lane for 2 weeks with no regressions. testing → accepted: a second
  consuming repo (madder) adopts the same module shape, and no tuning-lever
  adjustments are needed for 2 weeks.
---

# Facade export as a conformist module

## Problem Statement

`dagnabit export` regenerates `libs/dewey/pkgs/` public facades from the
`internal/` packages, but it lives *outside* the conformist fmt/lint surface:
purse-first gates drift read-only via the standalone `just lint-dewey_pkgs_drift`
recipe (`dagnabit export --check --library`), and the only fix is a manual
`dagnabit export` (no `--check`). Editing a public symbol in `internal/` without
remembering to regenerate produces **no local fmt/lint signal** — the drift only
surfaces late at the merge gate, forcing a regenerate-and-recommit cycle (this
bit a real merge: an `internal/bravo/markl` change passed `go build` and the
tests but failed the drift lane). The sibling `dagnabit reposition` drift check
already solved this exact shape — it is a conformist whole-tree linter with a
`command` (check) **and** a `repair-command` (apply) — so facade export is the
one remaining dagnabit whole-tree concern not yet expressed as a conformist
module.

## Interface

A new conformist linter module, `nix/linters/dewey-facade-export.nix`, mirroring
the existing `nix/linters/dewey-reposition.nix`:

- **`linters.dewey-facade-export.enable`** — `mkEnableOption`, enabled in
  `conformist-impure.nix` alongside `linters.dewey-reposition.enable`.
- **`command`** (read-only check) — runs `dagnabit export --check --library`
  from `libs/dewey`, exiting nonzero and naming the out-of-sync packages on
  drift. Equivalent to today's `lint-dewey_pkgs_drift` recipe body.
- **`repair-command`** — runs `dagnabit export --library` (no `--check`),
  regenerating the `pkgs/` facades in place. This is the half that does not
  exist today; it makes `conformist` repair (and therefore `nix fmt` /
  `just codemod-fmt-conformist`) resync facades.
- **`includes`** — `libs/dewey/**/*.go` as a *trigger gate* only (matching the
  sibling `dewey-reposition`). This deliberately covers **both** `internal/`
  (the facade source) and `pkgs/` (the generated facades): conformist only runs
  a `passes-files = false` whole-tree linter when a file matching `includes` is
  in scope, so an `internal/`-only glob silently skips the check on a `pkgs/`-only
  edit (a stale or hand-edited facade with `internal/` untouched). Verified: with
  the narrower `internal/**` glob, perturbing a committed facade did not trip the
  lane; broadening to `libs/dewey/**` does.
- **`passes-files = false`** — whole-tree; the script reads the real
  `internal/ → pkgs/` relationship itself rather than per-file.

Both the check and repair scripts thread the environment that the #159 fix
introduced and that `lint-dewey_pkgs_drift` already sets:

- `DAGNABIT_CONFORMIST_CONFIG=<conformist-config store path>` — points dagnabit
  at purse-first's own Nix-generated config so its facade-formatting pass does
  not walk up to a stray ancestor `conformist.toml`.
- `DAGNABIT_CEILING_DIRECTORIES=<worktree root>` — belt-and-suspenders bound on
  any upward config walk.

**Impurity placement.** Like `dagnabit reposition`, `dagnabit export` shells out
(to `go`/`go list` for the package graph and to `conformist` for facade
formatting), so the module lives in the **impure self-check lane**
(`conformist-impure.nix` → `conformistImpureEval`, run by `just lint-worktree`),
NOT the sandboxed `checks.<sys>.formatting`. This is the same constraint that
keeps `dewey-reposition` and conformist's own gomod2nix linter out of the
sandbox.

**Recipe consolidation.** Once the module's check half is wired into
`lint-worktree`, the standalone `lint-dewey_pkgs_drift` recipe (and its place in
the `lint` aggregate) is retired or reduced to a thin alias, since the conformist
impure lane now subsumes it. The `debug-dewey-export-library` /
`debug-dewey-export` recipes stay as the manual escape hatches.

## Examples

Detect facade drift through the conformist lane (was a separate recipe):

    just lint-worktree
    # dewey-facade-export: libs/dewey/pkgs/ is out of sync with internal/:
    #   internal/bravo/markl
    # run `just codemod-fmt` (or dagnabit export --library) and commit.

Repair facades through conformist repair (the new capability):

    just codemod-fmt            # conformist repair runs dewey-facade-export's
                                # repair-command → regenerates pkgs/ in place
    git add libs/dewey/pkgs && git commit

Manual escape hatch (unchanged):

    just debug-dewey-export-library          # regenerate all facades
    just debug-dewey-export internal/0/go_module   # one package

## Limitations

- **Impure lane only.** Because facade export shells to `go`/`conformist`, the
  check runs in `just lint-worktree` (working-tree, host tools on PATH), not the
  hermetic sandboxed `checks.formatting`. A contributor who only runs the
  sandboxed gate will not see facade drift; the merge hook (`just`) runs the full
  `lint` aggregate, which includes `lint-worktree`, so the gate is still closed
  at merge time.
- **No new whole-tree-hook concept in conformist.** This deliberately reuses
  conformist's existing per-linter `command` / `repair-command` /
  `passes-files = false` surface (already proven by `dewey-reposition`) rather
  than introducing a project-level/whole-tree fixer abstraction upstream. #153's
  "may warrant an ADR for a whole-tree hook" concern is resolved by *not* needing
  one — the existing linter surface suffices.
- **`dagnabit` must be on PATH.** Both scripts require the working-tree
  `dagnabit` (via `just build-dagnabit` into `build/`, or the dev shell); they
  fail loud with a build hint if it is absent, matching `dewey-reposition`.
- **Repair ordering vs reposition.** A `conformist` repair pass that triggers
  *both* reposition (which moves packages and rewrites facades) and facade-export
  could interact. In practice reposition already updates facades as part of its
  move, and a subsequent export is idempotent; if ordering ever matters, the
  linter `priority` field is the lever (see below).

## Tuning Levers

| Lever | Current | Rationale | Change signal |
|---|---|---|---|
| lane placement | impure (`lint-worktree`), not sandboxed `checks.formatting` | export shells to `go`/`conformist`; cannot run in a read-only `/nix/store` sandbox — matches `dewey-reposition` | dagnabit gains a fully-hermetic export path (no `go list`, vendored graph) → move to the sandbox |
| `lint-dewey_pkgs_drift` recipe | retired / thin alias once module check lands | conformist impure lane subsumes the standalone read-only check; one "make it conform" entrypoint | a consumer needs the drift check *without* the rest of the impure lane → keep a standalone recipe |
| config threading | explicit `DAGNABIT_CONFORMIST_CONFIG` + `DAGNABIT_CEILING_DIRECTORIES` env in the script | reuses the #159 plumbing already proven in `lint-dewey_pkgs_drift`; avoids the stray-ancestor-config walk | dagnabit learns to resolve the conformist config itself (e.g. as a conformist sub-invocation) → drop the explicit env |
| reposition/export repair ordering | unspecified (rely on export idempotence) | reposition already rewrites facades on move; export is idempotent | a real repair run produces churn or a non-fixpoint → set explicit `priority` so reposition repairs before export |

## More Information

- Consolidates purse-first issues **#163** (canonical: express facade-format as
  a first-class conformist module/linter), **#153** (facade sync through
  conformist: fmt autofix + lint check), and **#156** (facade regeneration as
  part of conformist repair). On landing, #153 and #156 close as duplicates of
  #163.
- Pattern source: `nix/linters/dewey-reposition.nix` (purse-first#160) — the
  reposition-drift whole-tree linter this module mirrors.
- Enabling plumbing: purse-first#159 (bounded the dagnabit formatter-config walk
  and added `DAGNABIT_CONFORMIST_CONFIG` / `DAGNABIT_CEILING_DIRECTORIES`), which
  `lint-dewey_pkgs_drift` already consumes — the check half of this feature is a
  re-wiring of that recipe into the conformist module shape.
- Impure-lane wiring: `conformist-impure.nix`, `flake.nix` `conformistImpureEval`
  (`conformist-impure-config` output), `just lint-worktree`.
- Related dagnabit followups still open: #162 (tolerate the conformist wrapper's
  baked `--tree-root-file`) and #161 (`lint-worktree` should use the hermetic
  flake wrapper) — both should land before or with this feature, since the repair
  half invokes the formatter through conformist.
