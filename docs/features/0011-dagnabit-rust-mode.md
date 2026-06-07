---
status: experimental
date: 2026-06-07
promotion-criteria: >
  experimental → testing: dagnabit rust mode adopted by at least one real
  cargo workspace outside this repo's test fixtures, with reposition and
  export running in its CI gate. testing → accepted: no tuning-lever
  adjustments needed for 2 weeks of that adoption, and the #[macro_export]
  glob-facade question resolved (verified or documented as unsupported).
---

# dagnabit rust mode

## Problem Statement

dagnabit organizes Go packages by dependency depth (NATO-tier reposition),
generates public facades from internal packages (export), and relocates
packages with reference rewrites (move/rename) — but only for Go modules.
Rust workspaces want the same discipline: crates tiered by dependency
height, `pkgs/` facade crates over `internal/` crates, and renames that
don't leave dependents broken. Without tool support, each of those is a
manual, error-prone multi-file edit.

## Interface

Rust support is a language mode of the existing `dagnabit` binary; every
subcommand keeps its flag vocabulary where the concept transfers.

**Language selection.** Auto-detected by walking up from the current
directory: `go.mod` → go mode; a `Cargo.toml` containing a `[workspace]`
table → rust mode (a `[package]`-only manifest does not count). Both
markers at one level, or neither anywhere, is an error; `-lang go|rust`
overrides and restricts the walk. Go mode keeps its run-from-module-root
contract; rust mode operates on the detected workspace root from any
subdirectory.

**reposition** (`dagnabit [-n] [-v] [-depth N] [--initial] <prefix>...`):
crates are the unit. The dependency graph comes from
`cargo metadata --format-version 1 --no-deps`, considering only path
dependencies between workspace members (registry deps ignored;
cross-prefix edges dropped, mirroring go mode). Moves are `git mv` plus
comment-preserving rewrites of relative `path = "…"` dependency entries
and the `[workspace] members` list — no `.rs` file is touched. Each move
is gated on `cargo metadata` still resolving.

**export** (`dagnabit export [--library] [--check] [packages...]`):
generates a glob-facade crate `pkgs/<name>/` per internal crate —
`Cargo.toml` with a single path dependency plus `src/lib.rs` containing
`pub use <name>_internal::*;` — and appends it to `[workspace] members`.
Internal crates MUST use the `<name>_internal` package-name convention
(collision with the facade name is a hard error). The directive analog of
`//go:generate dagnabit export` is `[package.metadata.dagnabit]
export = true` in the crate manifest. `--check` regenerates into a temp
dir and byte-compares against the committed facades. Generated content is
byte-canonical; no formatter pass runs (deliberate — a best-effort
formatter would reintroduce environment-dependent drift between export
and check).

**move / rename** (`dagnabit move <src> <dst>`, `dagnabit rename <src>
[new-leaf]`): a same-leaf move is Cargo.toml-only (delegates to the
reposition mover). A leaf rename additionally rewrites the crate's
`[package]` name (preserving the `_internal` suffix), dependents'
dependency keys, and dependents' source references via ast-grep using a
single whole-identifier pattern (old lib-target identifier → new). Renames
are gated by `cargo check --workspace` before and after the rewrite
(`--force` skips the pre-flight only); ast-grep presence and a stale
`pkgs/<oldLeaf>` facade are checked unconditionally before any mutation.
`rename` computes the destination level from the crate's transitive
in-workspace dependency height, then delegates to `move`.

**Runtime requirements.** Rust mode shells out to `cargo` (PATH
expectation, like `go`). Renames need `ast-grep`; the nix-built dagnabit
wraps the binary with ast-grep as a PATH *suffix* fallback, so a
user-provided ast-grep wins.

## Examples

Dry-run reposition of a tiered workspace:

    cd my-workspace
    dagnabit -n internal
    {"dst":"internal/alfa/store","event":"would-move","src":"internal/0/store"}

Export glob facades for every internal crate and verify drift later:

    dagnabit export --library
    dagnabit export --check --library   # exit 1 + file list on drift

Rename a crate, rewriting all dependents:

    dagnabit move internal/0/blob_store_id internal/0/blob_id
    # store/Cargo.toml: blob_store_id_internal = {...} → blob_id_internal = {...}
    # store/src/lib.rs: blob_store_id_internal::make_id() → blob_id_internal::make_id()
    # gated by cargo check --workspace pre and post

## Limitations

- **Two-language dispatch.** The CLI selects go/rust via an if/else over
  detection plus a `-lang` flag. This was flagged at design time as not
  scaling: a third language is the trigger for a registry/backend-interface
  refactor.
- **Crates only.** Module-level granularity (organizing `mod` trees inside
  one crate) is out of scope; the organizational unit is the workspace
  member crate.
- **`#[macro_export]` coverage is UNVERIFIED.** Glob facades re-export
  items via `pub use <name>_internal::*;`. Whether exported macros flow
  through (expected since Rust 1.32, via crate-root re-export) was never
  verified — no macro fixture exists. Treat macro re-export as unknown,
  not as working, until a fixture settles it.
- **No consumer import rewriting and no `--copy`** in rust mode; both are
  go-only flags and error explicitly.
- **`[workspace.dependencies]` keys** (inherited deps) are outside the
  dep-key rename's scope; a workspace using them relies on the post-flight
  `cargo check` gate to surface the breakage.
- **Manifest edit coverage.** Quoted table keys (`[dependencies."x"]`) and
  target-specific deps (`[target.'cfg(...)'.dependencies]`) are not
  matched by the span-based Cargo.toml editor; misses surface via the
  cargo gates rather than silently corrupting.
- **macOS symlink hardening deferred.** Workspace-root paths are not
  symlink-resolved before comparison; current joins resolve correctly
  through symlinks on the supported platform (Linux), and the macOS
  divergence case is speculative.

## Tuning Levers

| Lever | Current | Rationale | Change signal |
|---|---|---|---|
| language dispatch shape | if/else over two languages + `-lang` flag | simplest thing that works for two languages; refactor cost not yet justified | a third language lands → registry/backend interface |
| `_internal` naming convention | enforced suffix for internal crates (hard error on facade-name collision) | gives the facade the bare name without workspace-unique-name conflicts | a real workspace where the convention fights existing naming → make configurable via `[package.metadata.dagnabit]` |
| cargo check gate scope | full `--workspace`, pre- and post-flight | maximal safety; fixture workspaces check in well under a second | too slow on large workspaces → dependents-only (`cargo check -p`) |
| ast-grep pattern set | a single whole-identifier pattern (supersedes the design's curated multi-pattern set; empirically validated across use decls, brace use-trees, qualified expressions, type positions, with no substring false positives) | tree-sitter identifier matching covered every context the multi-pattern set missed | coverage gaps surfaced by the cargo-check oracle → extend the set; unmanageable growth → reconsider rust-analyzer SSR |

## More Information

- Design: `docs/plans/2026-06-06-dagnabit-rust-design.md`
- Implementation plan: `docs/plans/2026-06-06-dagnabit-rust-plan.md`
- Implemented across master commits `aea9e38..157f7b7` (devshell, dewey
  packages `*/cargo_workspace` `*/cargo_manifest` `*/cargo_metadata`
  `*/dagnabit_rust`, CLI dispatch, nix wrap, BATS lane, docs)
- Man page: `dagnabit(1)` LANGUAGES section
- Followups: purse-first#142 (go mode root detection from subdirs),
  purse-first#143 (explore ast-grep for go moves)
