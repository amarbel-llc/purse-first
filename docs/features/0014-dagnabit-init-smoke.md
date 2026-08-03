---
status: proposed
date: 2026-08-03
promotion-criteria: >
  proposed → experimental: `dagnabit init-smoke` (generate / --check / run)
  lands, a `dagnabit.toml` declares purse-first/dewey's target arches, and both
  the drift check (via the `dewey-init-smoke` conformist module in
  `lint-worktree`) and the strict-loader run lane are wired into the default
  `just` gate — with the run demonstrably RED against a reintroduced #177-style
  init open on js/wasm and GREEN after. experimental → testing: a second repo
  (piggy, papi, hyphence, or madder) adopts the published
  `lib.conformistLinters.dewey-init-smoke` plus the run lane by declaring its
  arches, with no generator change needed. testing → accepted: no tuning-lever
  adjustments for 2 weeks of that adoption, and the browser-vs-bun strict-proxy
  fidelity question (below) resolved (a browser-only init hazard either
  demonstrably caught by the bun strict harness, or documented as requiring a
  repo-supplied browser loader).
---

# dagnabit init-smoke tests

## Motivation

A package `init()` (or package-level var initialization) that cannot be
satisfied on a particular `GOOS`/`GOARCH` compiles clean on **every** arch and
only detonates at module *instantiation* on the arch that can't satisfy it.
purse-first#177 is the reference: dewey `ui`'s `init()` opened `/dev/null`,
which panics under the js/wasm strict filesystem — invisible to `go build`,
caught only by actually running the module, and it reached papi as a downstream
wasm panic months later. This class is currently policed by hand-written,
per-repo-scripted tests (purse-first#179) that don't scale: a NEW package with a
bad `init()` ships uncovered until some consumer's wasm binary panics at load.

dagnabit already walks the full in-module package graph (to generate `pkgs/`
facades) and already runs in every dewey-layout repo's gate (`export --check`).
That makes it the natural home for a graph-wide, per-arch init-smoke capability:
write the mechanism ONCE, and every dagnabit-using repo (purse-first/dewey,
piggy, papi, hyphence, cutting-garden, madder) inherits it by declaring its
target arches. New buildable package ⇒ auto-covered, structurally — the drift
check fails until the generated import list is regenerated.

The design-critical refinement (purse-first#177 comments, verified landing
papi#62): the init guarantee is **loader-parametric**, not host-parametric. A
`GOOS=js` module's filesystem strictness is set by the JS glue it is instantiated
under — Go's generic `wasm_exec.js` supplies a *stub* filesystem where every op
fails with `ENOSYS` (this is what catches a `/dev/null` open); `wasm_exec_node.js`
supplies a *real* filesystem that opens `/dev/null` fine and **masks** the bug.
bun-with-generic-`wasm_exec.js` is strict *despite* bun having a real filesystem
— the shim, not the host, decides. So the gate must run the strict loader, and
the loader choice must be encoded as committed, code-reviewed config that cannot
be silently "cleaned up" into permissiveness (papi's `fec6b21` records the exact
trap: swapping to `wasm_exec_node.js` blinds the gate to the entire init-failure
class without turning a single test red).

## Interface

init-smoke is a new mode of the existing `dagnabit` binary (go only for now),
split into a codegen pair that mirrors `dagnabit export` and a distinct execute
action:

- **`dagnabit init-smoke`** — regenerate the per-arch blank-import test files
  from the live package graph (analogous to `dagnabit export`).
- **`dagnabit init-smoke --check`** — drift check: regenerate into a temp dir and
  byte-compare against the committed files, exiting nonzero and naming the
  out-of-sync arch(s) on drift (analogous to `dagnabit export --check`). No
  writes, no execution.
- **`dagnabit init-smoke run`** — build and *instantiate* each generated test
  under its declared loader, failing with the offending package named.

The generate/check pair is flag-shaped to mirror `export`; `run` is a distinct
subverb because it is a genuinely different operation (it executes code and
requires runtimes), not a variant of codegen.

### Configuration

Target arches are declared in a new **`dagnabit.toml`** at the dewey-module root
(`libs/dewey/dagnabit.toml` for purse-first, `go/dagnabit.toml` for madder —
the same `deweyDir` root the facade linter uses). This is dagnabit's first
repo-global config file; today every dagnabit mode is flag-driven, with only
per-unit directives (`//go:generate dagnabit export`, Cargo
`[package.metadata.dagnabit]`). A committed file is required here because both
the drift check and the run lane must read the *same* arch/loader/skiplist, and
because the loader choice is exactly the value that must not drift into a bash
recipe.

    # libs/dewey/dagnabit.toml
    [[init-smoke.arch]]
    goos   = "js"
    goarch = "wasm"
    loader = "strict"        # built-in bun + generic wasm_exec.js (stub FS, ENOSYS)
    # skip is usually empty: packages that don't build for the arch are
    # auto-excluded. List only a package that BUILDS for the arch but must be
    # deliberately excluded from init-smoke anyway.
    skip   = []

    [[init-smoke.arch]]
    goos   = "wasip1"
    goarch = "wasm"
    loader = "strict"        # wasmtime, no preopens
    skip   = []

Each `[[init-smoke.arch]]` entry declares:

- **`goos` / `goarch`** — the target arch. The generated test file carries the
  corresponding `//go:build` constraint (build-tag emission is already a dagnabit
  primitive).
- **`skip`** — packages, relative to the module root, that BUILD for this arch
  but should be excluded from init-smoke anyway (usually empty). Packages that do
  NOT build for the arch are auto-excluded: init-smoke imports whatever builds
  and instantiates it, and detecting a package that *should* build for the arch
  but regressed is #174's build-gate job, kept deliberately separate. A `skip`
  entry that matches no package is an error (typo guard). Per-arch, because
  eligibility can differ across arches.
- **`loader`** — the run-time instantiation profile, whose meaning is
  `goos`-dependent (see below).

### Loader taxonomy

The `loader` value selects how `run` instantiates the compiled test binary. Its
vocabulary depends on `goos`:

| goos | loader | meaning |
|---|---|---|
| `js` | `strict` (default) | built-in harness: bun evaluating Go's **generic** `wasm_exec.js` with **no** real filesystem — the stub FS that fails with `ENOSYS`. Catches the #177 class. |
| `js` | `node` | Go's stock `go_js_wasm_exec` → `wasm_exec_node.js`, real filesystem. The *false-confidence* path; offered only for explicit opt-in/parity, never the default. |
| `js` | `<repo-path>` | a repo-supplied loader (e.g. `clients/ts/papi.ts`) — exercise the *exact* shim the repo ships. |
| `wasip1` | `strict` (default) | wasmtime with no `--dir` preopens (no ambient filesystem). |
| `wasip1` | `<repo-path>` | a repo-supplied wasmtime invocation / preopen policy. |
| host arch | `native` | plain `go test` (no wasm exec); a blank-import-everything test for the host too. |

The built-in js `strict` harness is the write-once win: dagnabit ships an
embedded JS entry that resolves `wasm_exec.js` from `GOROOT/lib/wasm` (falling
back to `misc/wasm`), instantiates the `.wasm` under it, and deliberately does
NOT wire a real `fs`, so every filesystem op returns `ENOSYS` — the browser-FS
behavior, reproduced under bun without a browser.

### Run mechanics

`dagnabit init-smoke run` owns the exec orchestration (rather than a justfile
recipe) so the loader choice stays with the config that declares it. For each
declared arch it invokes `go test` with an `-exec` wrapper built from the
loader, wrapping the wasm host in an `env -i` scrub: `wasm_exec_node.js` and the
generic shim hand the module the entire ambient environment, and `Go.run`
rejects an argv+env payload over its size cap — which a nix devshell's
environment alone exceeds. The scrub keeps only the host binary's directory plus
coreutils on `PATH` (the wrapper scripts shell out to `dirname`/`readlink`).
This reuses the exact trick proven in the `debug-test-wasm` recipe — but under
the **strict** loader, which that recipe (node loader) is not.

### Gate wiring

Mirroring the two existing dagnabit whole-tree concerns, the two halves land in
two lanes:

- **Drift check** → a conformist whole-tree linter **`dewey-init-smoke`**
  (`nix/linters/dewey-init-smoke.nix`), mirroring `dewey-facade-export`:
  `command` runs `dagnabit init-smoke --check`, `repair-command` runs
  `dagnabit init-smoke`, `includes = ["<deweyDir>/**/*.go", "<deweyDir>/dagnabit.toml"]`
  as a trigger gate, `passes-files = false`. It runs in the impure self-check
  lane (`just lint-worktree`) because enumeration shells to `go list`. Published
  from purse-first's flake as `lib.conformistLinters.dewey-init-smoke`
  (alongside `dewey-facade-export` / `dewey-reposition`), parameterized by
  `deweyDir` and `dagnabitPackage`, so downstream repos import it.
- **Run** → a justfile leaf (e.g. `validate-init-smoke`) that calls
  `dagnabit init-smoke run`, wired into the default gate. The wasm runtimes
  (bun, wasmtime) are **flake-pinned** and put on `PATH` via the devshell — NOT
  pulled with `nix shell nixpkgs#…` at run time — so the lane is reproducible in
  the merge gate (purse-first#174's explicit requirement for promoting a wasm
  lane).

## Examples

Declare arches, then regenerate the per-arch import tests:

    cd libs/dewey
    dagnabit init-smoke
    # writes init_smoke/init_smoke_js_wasm_test.go     (//go:build js && wasm)
    #        init_smoke/init_smoke_wasip1_wasm_test.go (//go:build wasip1 && wasm)
    # each blank-imports every buildable, non-skipped package for that arch

Fail the gate on a stale import list (a new buildable package was added):

    dagnabit init-smoke --check
    # error: init-smoke tests are out of sync with the package graph
    #   js/wasm: internal/charlie/newpkg (missing — not imported)
    # run `dagnabit init-smoke` and commit

Instantiate every package under the strict loader; catch a #177-style init:

    dagnabit init-smoke run
    # js/wasm (strict): FAIL
    #   panic: open /dev/null: ENOSYS
    #   offending import: code.linenisgreat.com/purse-first/libs/dewey/internal/charlie/ui
    # exit status 1

Exercise the repo's own shipped shim instead of the built-in strict harness:

    # dagnabit.toml: loader = "clients/ts/papi.ts"
    dagnabit init-smoke run   # runs each test binary under papi's exact loader

## Limitations

- **New non-buildable packages are silently uncovered.** A new package that is
  *buildable* for an arch is auto-covered (drift catches the stale import list);
  a new package that does NOT build for the arch is auto-excluded with no signal.
  If such a package was *meant* to be arch-safe, that gap belongs to #174's build
  gate, not init-smoke — init-smoke only instantiates what builds. `skip` is for
  the rarer case of a buildable package deliberately left out; a `skip` entry
  matching no package errors (typo guard). Given dewey's wasm reality (only a
  small subset builds for wasm), this keeps the config tiny rather than requiring
  a large skiplist of everything unbuildable.
- **bun strict is a faithful proxy, not a browser.** The built-in js `strict`
  harness reproduces the browser's *filesystem* strictness (`ENOSYS` stub) — the
  #177 class — under bun, with no DOM or browser-specific globals. An init
  hazard that depends on a browser API *other* than the FS stub is not covered;
  such a repo sets `loader = "<its browser harness>"`. Whether bun strict fully
  substitutes for a browser for the FS class is the open fidelity question in the
  promotion criteria.
- **wasip1 preopen policy is minimal.** Only `strict` (no preopens) is modeled;
  a repo needing preopens declares a repo-path loader. A first-class
  `preopen = [...]` surface is deferred until a real consumer needs it.
- **go only.** Rust workspaces (dagnabit rust mode, FDR 0011) are out of scope;
  the wasm init-hazard class is Go-specific here. A future rust analog would be a
  separate arch model.
- **Run impurity.** `run` shells to bun/wasmtime and cannot run in the sandboxed
  `checks.formatting`; it is a test-lane concern needing runtimes on `PATH`. The
  drift check is lighter (only `go list`) and rides the existing impure conformist
  lane.
- **Does not subsume the build gate (#174).** init-smoke instantiates packages
  that *build* for the arch; the wasm-*build* check (#174) that a package
  compiles at all is a separate lane. The two could later share this
  enumeration + skiplist, but this feature deliberately does not fold #174's
  hand-maintained portable-path list in.

## Tuning Levers

| Lever | Current | Rationale | Change signal |
|---|---|---|---|
| built-in js strict runtime | bun | the strict-shim combo proven to catch #177 landing papi#62; single small dep | bun proves flaky/heavy in CI, or a repo needs node semantics → allow `strict` to back onto node's eval of the generic shim |
| skip polarity | auto-exclude unbuildable; `skip` = buildable exclusions (usually empty) | dewey's wasm-safe set is small, so a skiplist-of-unbuildable would be huge and duplicate #174's build gate | a repo wants the strict "every unbuildable must be declared" forcing function → add an opt-in strict/allowlist mode |
| generated-test location | `<deweyDir>/init_smoke/` (module-root sibling of `internal/`/`pkgs/`) | keeps the package out of `internal/`, so facade export (`--library` scans `internal/`) and reposition (operates on `internal/`) both ignore it — no private-marker or NATO-level dance | a repo wants it elsewhere → a config `output-dir` key (mirrors export) |
| enumeration | `go list -e` under the arch env, keep in-module buildable non-main non-test, minus `skip` | `-e` lists every package with per-package build errors and exits 0, so unbuildable packages are cleanly auto-excluded rather than aborting the list | a repo needs the strict forcing function → opt-in allowlist mode (see skip polarity) |
| runtime provenance | flake-pinned bun/wasmtime on the devshell PATH | reproducible merge gate (#174) | a target needs a runtime not in the flake's nixpkgs → add an input or a repo-loader escape hatch |
| loader as config data | committed `dagnabit.toml` field, read by both check and run | keeps the strict/permissive choice code-reviewed, not a bash line (papi `fec6b21`) | dagnabit learns to introspect the repo's shipped loader automatically → derive rather than declare |

## More Information

- **Subsumes** purse-first#179 (the per-repo generated "import every package"
  test) — implemented here as a dagnabit capability instead of a bespoke
  per-repo script. #179 closes on this landing.
- **Complements** purse-first#174 (promote a wasm-*build* check into the gate):
  #174 is "does it compile for wasm", this is "does its init run without panic".
  Shared enumeration is a possible future unification (see Limitations).
- **Motivating bug**: purse-first#177 (dewey `ui` init `/dev/null` open) — this
  feature would have caught it graph-wide at the dewey change, not months later
  downstream. Regression guard: `libs/dewey/internal/charlie/ui/printer_null_test.go`.
- **Loader refinement source**: purse-first#180 comments; verified landing
  papi#62; the anti-permissiveness constraint recorded in papi `fec6b21`.
- **Pattern sources**: `nix/linters/dewey-facade-export.nix` (FDR 0013 — the
  conformist whole-tree check/repair module and `lib.conformistLinters`
  publishing this drift module mirrors) and the `debug-test-wasm` justfile recipe
  (the `env -i` scrub and GOROOT `wasm_exec` resolution this run lane reuses,
  under the strict loader).
- **Prior dagnabit capability**: FDR 0011 (dagnabit rust mode) — the language-mode
  precedent and the drift-checked codegen loop this extends.
- **Per-repo adoption follow-ups** (each becomes "declare arches in
  dagnabit.toml + import the published module + wire the run lane"): piggy#240,
  papi#64, hyphence#12, and madder.
