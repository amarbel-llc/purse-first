# dagnabit Rust support — design

Date: 2026-06-06
Status: approved (interactive brainstorming session)

Extend dagnabit's three code-organization capabilities — NATO-tier
reposition, export facades, and move/rename — to Rust, as a language mode
of the existing binary. Go behavior is untouched.

## Settled decisions

| Decision | Choice |
|----------|--------|
| Organizational unit | Crates in a cargo workspace. Module-level (within one crate) is explicitly out of scope for v1; design leaves room for it later. |
| Facade style | Glob re-export: `pub use <internal_crate>::*;`. No symbol enumeration, no Rust parsing for export. |
| Toolchain entry | `devenvs/rust` devshell (stable-first nixpkgs); tests build real cargo fixture workspaces and `t.Skip` when `cargo`/`ast-grep` are absent. |
| Language selection | Auto-detect (`go.mod` → go, workspace `Cargo.toml` → rust); `-lang go\|rust` flag overrides; both/neither present without the flag is an error. |
| Rename source rewrites | ast-grep (tree-sitter-rust structural rewrite), pinned via nix; gated by `cargo check --workspace` before and after. |
| Architecture | Approach A: per-language implementation packages behind the existing `DependencyReader`/`LevelMapper`/`PackageMover` seams. Battle-tested Go paths (`*/dagnabit` exporter/mover) are not refactored. |

## FDR flag (recorded limitation)

The CLI language dispatch (if/else over two languages plus a `-lang`
flag) does not scale: a third language should trigger a
registry/backend-interface refactor. Record this as a limitation in the
feature's FDR when written.

## Section 1 — CLI surface & language dispatch

- `cmd/dagnabit/main.go` resolves language once, shared by all
  subcommands: `-lang` flag, else walk up from CWD (`go.mod` → go;
  `Cargo.toml` containing `[workspace]` → rust; both in the same root or
  neither → error instructing `-lang`).
- Go mode is byte-for-byte today's code path. Go-only flags (`-module`,
  `--copy`, …) error in rust mode rather than being silently accepted.
- New dewey package `*/cargo_workspace` (level `0/`, peer of
  `go_module`): walks up to the nearest `[workspace]` Cargo.toml,
  returns root dir + member list. Rust analog of
  `go_module.ResolveModulePath`.

## Section 2 — reposition

`Repositioner` orchestrator unchanged; rust mode supplies
implementations:

- `*/cargo_metadata` (new, level `alfa/`, peer of `go_list`) implements
  `DependencyReader`: shells `cargo metadata --format-version 1
  --no-deps`, parses JSON, emits edges between workspace member crates
  only (path-deps under the workspace root; registry deps ignored). Node
  names are manifest-dir paths relative to the workspace root, honoring
  `ComponentDepth` 2/3 and `--initial` exactly like `go_list`.
- Rust `PackageMover`: `git mv` + workspace-wide rewrites of relative
  `path = "…"` dependency entries (dependents' references and the moved
  crate's own deps) + root `[workspace] members` update. No `.rs` edits —
  crate names don't change in a pure reposition.
- TOML edits are span-based: parse to locate entries, edit raw text
  spans, preserving user comments/formatting (Go TOML libs don't
  round-trip comments).
- Post-move gate: `cargo metadata` must succeed (structural validation
  that all path-deps resolve).

## Section 3 — export

New `*/dagnabit_rust` package (level `echo/`, peer of `*/dagnabit`).

- For internal crate `internal/<level>/<name>`, generate facade crate
  `pkgs/<name>/`:
  - `Cargo.toml`: package name `<name>`, single path-dep on the internal
    crate, version/edition copied from it.
  - `src/lib.rs`: generated header + `pub use <internal_crate_name>::*;`.
  - Appended to `[workspace] members` if absent.
- Naming: internal crates use the `<name>_internal` package-name
  convention so the facade can own the bare `<name>`; collision is a
  hard error at export time.
- Invocation modes: explicit crate dirs; `--library` (everything under
  `internal/`); directive analog is `[package.metadata.dagnabit]
  export = true` in the internal crate's Cargo.toml (idiomatic home for
  tool config; no magic comments); `--check` regenerates into a temp dir
  and diffs against committed `pkgs/`.
- Formatting: existing treefmt-discovery `FormatOutput` (already
  language-agnostic).
- Not supported in rust mode (explicit errors): `--copy`, consumer
  import rewriting.
- Open implementation question: whether `pub use …::*` covers
  `#[macro_export]` macros (it should since Rust 1.32, via crate-root
  re-export) — verify with a fixture; if not, document as a limitation
  rather than building macro handling into v1.

## Section 4 — move / rename

In `*/dagnabit_rust`, mirroring the Go split (move = explicit src→dst;
rename = recompute one crate's NATO level, optional leaf rename).

- Move without leaf rename: identical to the reposition mover
  (Cargo.toml-only rewrites, no `.rs` edits).
- Move/rename with leaf rename:
  1. Pre-flight gate: `cargo check --workspace` (analog of the Go
     `packages.Load` gate); `--force` skips.
  2. Dir move + TOML rewrites, plus `[package] name` rewrite
     (preserving the `_internal` suffix convention) and dependents'
     `[dependencies]` key renames.
  3. Source rewrites via ast-grep in dependent crates: curated pattern
     set for use-declarations, qualified path expressions, and macro
     paths (`old::$$$REST` → `new::$$$REST`). Rewrites use the lib
     target name (underscored — hyphens in package names map to
     underscores in source), read from `cargo metadata`.
  4. `crate::` self-references inside the moved crate need no rewrite.
  5. Post-rewrite gate: `cargo check --workspace`; on failure report and
     leave the tree dirty for inspection (clean-git-start assumption,
     same as Go).
- `rename`'s level computation reuses `computeRequiredLevel` (operates
  on `DependencyReader` edges; language-agnostic once `cargo_metadata`
  feeds it).
- Dry-run prints planned dir move, TOML rewrite counts, and ast-grep
  match counts (match without `--update-all`).
- Pattern-set coverage is validated against fixture workspaces in tests,
  not enumerated in this design.

## Section 5 — build, test, packaging

- `devenvs/rust/`: cargo/rustc from stable nixpkgs, composed into the
  default shell. ast-grep in the devshell and wrapped into the dagnabit
  nix package's PATH (wrapProgram suffix; falls back to user PATH
  outside nix). `cargo` remains a plain runtime PATH expectation
  (dagnabit never vendored the Go toolchain either).
- Go tests in `*/dagnabit_rust` and `*/cargo_metadata`: fixture builders
  write minimal real cargo workspaces to `t.TempDir()`; tests needing
  `cargo`/`ast-grep` use `exec.LookPath` + `t.Skip`. A
  facade-compiles test mirrors `TestExportPackageGeneratedFacadeCompiles`.
- BATS lane: `zz-tests_bats/dagnabit_rust.bats`, end-to-end CLI against
  a fixture workspace (per eng:wiring-bats-tests conventions).
- gomod2nix: only new Go dep is a TOML parser (locate-only; edits are
  span-based). `just build-nix-gomod2nix` after go.mod changes.

## Section 6 — error handling & rollback

- External-tool failures (`cargo metadata`, `cargo check`, `ast-grep`)
  surface stderr verbatim with the failing argv. Missing tools error
  with "required for rust mode" before any tree mutation.
- Mutation ordering: validate → compute full plan → mutate → gate.
- Rollback: purely additive feature; go mode is unchanged, so the dual
  architecture is inherent. Disabling = don't use rust mode. Release
  rollback = previous dagnabit tag (single nix input pin). No promotion
  criteria needed — nothing old is replaced.

## Tuning levers (→ FDR Tuning Levers section)

1. **Language dispatch shape** — current: if/else over two languages +
   `-lang`. Change signal: a third language → registry/backend refactor.
2. **`_internal` naming convention** — current: enforced suffix. Change
   signal: a workspace where the convention fights existing naming →
   configurable via `[package.metadata.dagnabit]`.
3. **`cargo check` gate scope** — current: full workspace, pre+post.
   Change signal: too slow on large workspaces → dependents-only
   (`cargo check -p`).
4. **ast-grep pattern set** — current: curated (use decls, path exprs,
   macro paths). Change signal: coverage gaps → extend; unmanageable
   growth → reconsider rust-analyzer SSR.
