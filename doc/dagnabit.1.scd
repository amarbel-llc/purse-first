dagnabit(1)

# NAME

dagnabit - reposition Go packages and Rust crates by dependency depth and generate export facades

# SYNOPSIS

*dagnabit* [*-n*] [*-v*] [*-depth* _N_] [*--initial*] [*-lang* _go|rust_] [*-module* _path_] _prefix_...

*dagnabit export* [*-n*] [*-output-dir* _dir_] [*--library*] [*--check*] [*--copy*] [*--no-rewrite-consumers*] [*-lang* _go|rust_] [*-module* _path_] [_packages..._]

*dagnabit move* [*-n*] [*-v*] [*--force*] [*-lang* _go|rust_] [*-module* _path_] _src_ _dst_

*dagnabit rename* [*-n*] [*-v*] [*--force*] [*-lang* _go|rust_] [*-module* _path_] _src_ [_new-leaf_]

*dagnabit init-smoke* [*--check*] [*-n*] [*-module* _path_]

*dagnabit init-smoke run* [*-n*] [*-module* _path_]

*dagnabit version*

# DESCRIPTION

*dagnabit* is a code organization tool for Go modules and cargo (Rust) workspaces with five subcommands: repositioning, export facade generation, package relocation (*move*), single-package repositioning (*rename*), and per-arch init-smoke test generation (*init-smoke*, go-only). The operating language is auto-detected; see *LANGUAGES*.

Without a subcommand, *dagnabit* analyzes the dependency graph of packages, computes a topological sort, and moves packages into directories named after NATO phonetic levels based on their dependency depth. Leaf packages (no internal dependencies) go to level 0, their dependents to alfa, and so on up through the NATO alphabet. In go mode the graph comes from *go list*; in rust mode from *cargo metadata*.

The *export* subcommand generates facade packages that re-export symbols from _internal/_ packages, creating a public API surface in _pkgs/_. In go mode, types are re-exported as aliases, non-generic functions and variables as var bindings, and constants as const bindings. Generic functions are wrapped because Go does not support assigning them to variables without instantiation (see golang/go#52654, https://github.com/golang/go/issues/52654). Functions whose signatures reference unexported types are skipped. Go code generation uses *jennifer* (github.com/dave/jennifer) for proper import management. In rust mode, facades are one-line glob re-export crates; see the *export* subsection under *SUBCOMMANDS*.

# LANGUAGES

*dagnabit* operates on Go modules (go mode) and cargo workspaces (rust mode). The mode is auto-detected by walking up from the current directory; the first directory containing either marker wins:

*go.mod*
	go mode.

*Cargo.toml containing a [workspace] table*
	rust mode. A crate-only *Cargo.toml* (a *[package]* table without *[workspace]*) does not count; the walk continues upward past it.

Finding both markers in the same directory, or neither anywhere up to the filesystem root, is an error. Pass *-lang go* or *-lang rust* to disambiguate; *-lang* also restricts the upward walk to the named language's marker.

Both modes operate on the detected root: the directory containing _go.mod_ (go) or the workspace _Cargo.toml_ (rust). Either mode may be invoked from any subdirectory beneath its root. Prefix arguments and package paths are always interpreted relative to that root, not the invocation directory.

The *-module* flag is go-only; combining it with rust mode is an error.

## Runtime requirements (rust mode)

Rust mode shells out to *cargo*(1), which must be on *PATH*. Crate renames (*move* or *rename* with a differing leaf) additionally require *ast-grep* for dependent-source rewrites. The Nix package wraps the *dagnabit* binary with ast-grep appended to *PATH* as a fallback; a user-provided ast-grep earlier on *PATH* wins.

# SUBCOMMANDS

## reposition (default)

Analyze and reposition packages based on dependency depth. In go mode, must be run from a directory containing _go.mod_.

In rust mode, crates are the unit of repositioning. The dependency graph is read from *cargo metadata --format-version 1 --no-deps*; only path-dependencies that resolve to workspace members contribute edges (registry dependencies are ignored). Moving a crate rewrites the relative path-dependencies the move invalidated in every member *Cargo.toml* plus the *[workspace]* members entry in the root manifest; no _.rs_ file is touched (crate names do not change in a pure move). Each move is gated by re-running *cargo metadata* to verify the workspace still resolves.

## move

Relocate a single package from _src_ to _dst_ (both relative to the module or workspace root).

In go mode, when the leaf directory name differs between _src_ and _dst_, *move* performs type-aware rewrites of the package's *package* declaration and of qualified references (e.g., _oldLeaf_.X → _newLeaf_.X) in every caller, using *golang.org/x/tools/go/packages* to discriminate package uses from local variables, aliased imports, and dot-imports. A pre-move analysis aborts if *packages.Load* reports any type errors in the module. Pass *--force* to proceed anyway.

In rust mode, a move whose leaf is unchanged is a pure directory move with Cargo.toml-only rewrites, exactly as in *reposition*. When the leaf differs, the move is a crate rename: in addition to the directory move and manifest path-dependency rewrites, *dagnabit* rewrites the crate's *[package]* name (preserving the _<name>_\_internal convention: _oldLeaf_\_internal → _newLeaf_\_internal), renames the dependency key in every directly dependent member's *Cargo.toml*, and rewrites source references in those dependents via *ast-grep* using a single whole-identifier pattern (the old lib-target identifier → the new one), which matches use declarations, qualified expressions, and type positions without substring false positives.

Rust renames are gated by *cargo check --workspace* both before and after the rewrite; *--force* skips only the pre-flight check. Independent of *--force*, a rename always verifies ast-grep is on *PATH* and fails fast if a generated facade *pkgs/*_oldLeaf_ exists, before any tree mutation: facades are generated, not renamed; remove it and re-run *dagnabit export* after the rename. On a post-flight failure the tree is left dirty for inspection.

## rename

Reposition ONE package (go) or crate (rust) to the NATO level dictated by its transitive internal dependencies, optionally renaming its leaf. Other packages are NOT moved, even if they sit at the wrong level; use the default reposition subcommand to rebalance the whole tree.

The destination is computed, not given: the leaf is kept (or replaced by _new-leaf_, when given) and the level segment is replaced by the computed one. The actual move delegates to *move*, so the per-language rewrite and gating behavior described there applies unchanged. In go mode, _src_ is a two-component _<level>/<leaf>_ path; in rust mode it must be a three-component _<prefix>/<level>/<leaf>_ path.

## export

Generate facades from _internal/_ packages into the output directory (default _pkgs/_).

Three invocation modes:

*dagnabit export*
	Scan _internal/_ for opted-in packages and generate facades for each. In go mode, a package opts in with a *//go:generate dagnabit export* directive in a source file; in rust mode, with *[package.metadata.dagnabit] export = true* in the crate manifest.

*dagnabit export* _packages..._
	Generate facades for the specified packages (go: package patterns such as *./internal/alfa/...*; rust: crate directories relative to the workspace root, such as *internal/alfa/store*).

*dagnabit export --library*
	Export every package under _internal/_. In go mode this fails if any *//go:generate dagnabit export* directives exist; in rust mode it exports every crate under _internal/_.

In go mode, after generation, *dagnabit* runs the project formatter on the output directory when a *conformist*(1) configuration (*conformist.toml* or *.conformist.toml*) is present. The config is found by walking up from the module root, bounded by *DAGNABIT\_CEILING\_DIRECTORIES* (see *ENVIRONMENT*): the walk never ascends into a ceiling directory, so a repo whose own config is *Nix-generated* (no *conformist.toml* on disk) does not escalate to a stray ancestor config. For such a repo, set *DAGNABIT\_CONFORMIST\_CONFIG* to the generated config path; this short-circuits the search and runs *conformist --config-file* with that config. When a config is found but *conformist* is not on *PATH*, *dagnabit* fails rather than silently emitting unformatted facades. This is skipped under *--dry-run* and when no config is found.

In rust mode, facades are glob re-export crates: for an internal crate at *internal/*_<level>/name_ whose *[package]* name is _name_\_internal, *export* generates *pkgs/*_name_*/Cargo.toml* (a single path-dependency on the internal crate) and *pkgs/*_name_*/src/lib.rs* containing *pub use* _name_\_internal::\*; and idempotently adds the facade to the root *[workspace]* members. Internal crates MUST follow the _<name>_\_internal package-name convention; an internal crate whose *[package]* name equals its facade name is an error. *No* formatter pass runs on rust facades: the generated output is byte-canonical (rustfmt-clean) by construction, keeping *export* and *--check* environment-independent.

## init-smoke

Generate per-arch blank-import tests that catch package *init()* functions which fail on a particular _GOOS_/_GOARCH_. Such an init compiles clean on every arch and detonates only at module _instantiation_ on the arch that cannot satisfy it (e.g. a _/dev/null_ open under the js/wasm strict filesystem), so it is invisible to *go build* and caught only by running the module. Go-only.

Target arches are declared in _dagnabit.toml_ at the module root, as *[[init-smoke.arch]]* entries with *goos*, *goarch*, *loader*, and *skip* keys. For each arch, *dagnabit* type-checks the module's package graph under that _GOOS_/_GOARCH_ (via *golang.org/x/tools/go/packages*) and writes _initsmoke/initsmoke\_goos\_goarch\_test.go_, a *//go:build*-constrained file that blank-imports every package which _builds_ for the arch. Packages that do not build for the arch are auto-excluded; *skip* lists packages that build but should be excluded anyway (a *skip* entry matching no package is an error). Instantiating the generated test binary runs every imported package's *init()*, so a per-arch init hazard surfaces as a load-time panic naming the offender.

Like *export*, *init-smoke* runs the project formatter on the generated files (honoring *DAGNABIT\_CONFORMIST\_CONFIG* / *DAGNABIT\_CEILING\_DIRECTORIES*; see *ENVIRONMENT*), and *--check* regenerates into a temporary directory and byte-compares against the committed files, exiting nonzero and naming the out-of-sync arch files on drift.

*dagnabit init-smoke run* builds and instantiates each generated test under its arch's declared *loader*, so a package *init()* that fails on the arch surfaces as a load-time panic naming the offender. Loader values are goos-dependent. *strict* (the default) for js/wasm is Go's generic *wasm\_exec.js* run under *bun* (stub filesystem, so filesystem ops fail with ENOSYS -- the browser-strict behavior that catches the hazard), and for wasip1/wasm is *wasmtime* with no directory preopens. *node* (js only) is Go's stock *go\_js\_wasm\_exec* (real filesystem -- the false-confidence path, opt-in only). A repo-relative or absolute path names a repo-provided executable loader taking the test binary as its first argument. The required runtime (*bun*, *wasmtime*) must be on *PATH*.

## version

Print the build identity -- version and commit -- of the *dagnabit* binary, then exit. Output is a single self-identification line per *eng-versioning*(7):

```
dagnabit VERSION+COMMIT
```

The values are injected at link time by the Nix build (from the repo-root _version.env_). A plain *go build* without those ldflags reports *dev+unknown*. The same identity is stamped into the *Code generated by dagnabit (VERSION+COMMIT)* header of every exported facade, so a stale binary run against a newer library surfaces as facade drift rather than a silent compile error.

# POSITIONAL ARGUMENTS

_prefix_...
	One or more package tree prefixes to analyze (e.g. *lib*, *internal*, *src*). At least one prefix is required for the reposition subcommand.

# OPTIONS

## Reposition options

*-n*, *--dry-run*
	Show what would be moved without moving.

*-v*, *--verbose*
	Enable verbose output.

*-depth* _N_
	Path component depth: 3 for _prefix/level/package_ (default), 2 for _level/package_. Use depth 2 for repos where NATO levels are top-level directories. Implied (and overridden) by *--initial*.

*--initial*
	Initial tiering of a flat _<prefix>/<pkg>_ layout. Inserts a NATO level segment into every package path based on its dependency height, producing _<prefix>/<level>/<pkg>_. Implies *-depth 2*.

*-lang* _go|rust_
	Operating language. Auto-detected when empty (see *LANGUAGES*).

*-module* _path_
	Go module path. Read from _go.mod_ if empty. Go-only.

## Move options

*-n*, *--dry-run*
	Show what would be moved without moving: a *would-move* event is emitted for the planned move. In rust mode, a leaf rename additionally prints the rename plan with per-pattern ast-grep match counts (same-leaf moves print only the planned move).

*-v*, *--verbose*
	Enable verbose output on stderr.

*--force*
	Go: proceed even if *packages.Load* reports type errors. Rewrites may miss call sites in packages with type errors; use only when the errors are unrelated to the moved package. Rust: skip the *cargo check --workspace* pre-flight gate (the post-flight gate still runs).

*-lang* _go|rust_
	Operating language. Auto-detected when empty (see *LANGUAGES*).

*-module* _path_
	Go module path. Read from _go.mod_ if empty. Go-only.

## Rename options

Same flags as *move*: *-n*/*--dry-run*, *-v*/*--verbose*, *--force*, *-lang* _go|rust_ and the go-only *-module* _path_ all behave identically.

## Export options

*-n*, *--dry-run*
	Show what would be generated without writing files.

*-output-dir* _dir_
	Output directory for generated facades, relative to the module or workspace root (default: *pkgs*).

*--library*
	Export facades for every package under _internal/_. Does not accept package arguments. Go mode fails if any *//go:generate dagnabit export* directives exist.

*-c*, *--check*
	Verify the committed facades match a fresh export without writing: regenerate into a temporary directory, byte-compare against the committed tree, and exit nonzero on drift. Works with *--library*, explicit packages, or the directive scan.

*--copy*
	Copy internal source files into _pkgs/_, rewriting only intra-module imports, instead of emitting thin re-export aliases. Go-only.

*--no-rewrite-consumers*
	Skip rewriting external workspace consumers' imports to the new facade path. Go-only.

*-lang* _go|rust_
	Operating language. Auto-detected when empty (see *LANGUAGES*).

*-module* _path_
	Go module path. Read from _go.mod_ if empty. Go-only.

## Init-smoke options

*-c*, *--check*
	Verify the committed init-smoke tests match a fresh generation without writing: regenerate into a temporary directory, byte-compare against the committed _initsmoke/_ files, and exit nonzero (naming the out-of-sync arch files) on drift.

*-n*, *--dry-run*
	Enumerate and render without writing files.

*-module* _path_
	Go module path. Read from _go.mod_ if empty.

# EXAMPLES

Reposition packages in a repo with _lib/_ and _internal/_ prefixes:

```
cd myproject
dagnabit -n -v lib internal
```

Reposition packages in a repo where NATO levels are top-level:

```
dagnabit -n -v -depth 2 0 alfa bravo charlie
```

Initial tiering of a flat _internal/<pkg>_ layout (produces _internal/0/<pkg>_, _internal/alfa/<pkg>_, '.' etc.):

```
dagnabit -n -v --initial internal
```

Relocate a package with a leaf rename (rewrites package decl, imports, and qualified references in callers):

```
dagnabit move internal/foo internal/bar
```

Relocate without renaming the leaf (same leaf, imports-only rewrite):

```
dagnabit move internal/alfa/foo internal/bravo/foo
```

Dry-run a move to preview what would change:

```
dagnabit move -n internal/foo internal/bar
```

Generate export facades for all marked internal packages:

```
dagnabit export
```

Generate facades for specific packages:

```
dagnabit export ./internal/alfa/blob_store_id ./internal/bravo/...
```

Dry-run to see what would be generated:

```
dagnabit export -n ./internal/...
```

Dry-run a repositioning of cargo workspace crates under _internal/_ (runs from any subdirectory of the workspace):

```
dagnabit -n -v -lang rust internal
```

Generate glob facades for every internal crate of a cargo workspace:

```
dagnabit export --library
```

Move a crate with a rename (rewrites the [package] name, dependents' dependency keys, and dependents' sources via ast-grep, gated by *cargo check*):

```
dagnabit move internal/alfa/store internal/alfa/blob_store
```

Verify the committed rust facades are in sync without writing:

```
dagnabit export --check --library
```

Regenerate the per-arch init-smoke tests, then verify they are in sync:

```
dagnabit init-smoke
dagnabit init-smoke --check   # exit 1 + arch file list on drift
```

# GENERATED OUTPUT

In go mode, for an internal package at _internal/alfa/blob\_store\_id_, dagnabit generates _pkgs/blob\_store\_id/main.go_:

```
// Code generated by dagnabit (dev+unknown); DO NOT EDIT.

package blob_store_id

import internal "example.com/mod/internal/alfa/blob_store_id"

type (
    Id           = internal.Id
    LocationType = internal.LocationType
)

var (
    Make             = internal.Make
    MakeWithLocation = internal.MakeWithLocation
)
```

In rust mode, for an internal crate at _internal/alfa/blob\_store\_id_ whose *[package]* name is *blob\_store\_id\_internal*, dagnabit generates _pkgs/blob\_store\_id/Cargo.toml_ and _pkgs/blob\_store\_id/src/lib.rs_:

```
[package]
name = "blob_store_id"
version = "0.1.0"
edition = "2021"

[dependencies]
blob_store_id_internal = { path = "../../internal/alfa/blob_store_id" }
```

```
// Code generated by dagnabit (dev+unknown); DO NOT EDIT.

pub use blob_store_id_internal::*;
```

# DIRECTIVES

In go mode, place the following comment in any _.go_ file within an internal package to mark it for export:

```
//go:generate dagnabit export
```

In rust mode, the analog is a manifest key in the internal crate's *Cargo.toml*:

```
[package.metadata.dagnabit]
export = true
```

When *dagnabit export* is run without arguments, it scans _internal/_ for these markers.

# ENVIRONMENT

*DAGNABIT\_CEILING\_DIRECTORIES*
	Colon-separated list of absolute directories above which *dagnabit* will not walk when searching upward from the module root for a formatter configuration. Mirrors *GIT\_CEILING\_DIRECTORIES*: a ceiling entry is the last directory not searched, so directories at and below the ceiling (including the module root) are still checked. Use it to stop a repo with a Nix-generated conformist config from escalating to a stray ancestor *conformist.toml* (such as an eng-root config).

*DAGNABIT\_CONFORMIST\_CONFIG*
	Absolute path to an explicit *conformist* config file. When set, the upward configuration search is skipped entirely and *dagnabit* runs *conformist --config-file* with this path. Intended for repositories whose conformist config is Nix-generated and therefore not present on disk as a *conformist.toml* (see *DESCRIPTION*).

# EXIT STATUS

*0*
	Success.

*1*
	Error (dependency cycle, missing _go.mod_ or *[workspace]* _Cargo.toml_, generation failure, failed cargo gate, etc.).

# SEE ALSO

*go-generate*(1), *go-list*(1), *conformist*(1), *cargo*(1), *cargo-metadata*(1), *cargo-check*(1), *ast-grep*(1)

# BUGS

Report bugs at https://code.linenisgreat.com/purse-first/issues.
