# dewey

A multi-tier Go library inside `purse-first`, organized under NATO-phonetic
levels (`0/`, `alfa/`, `bravo/`, …, `golf/`) by dependency depth. Packages
at level N may only depend on packages at lower levels within dewey.

## Package naming convention

Packages are referenced by their **leaf** name throughout this document,
written as `*/<leaf>` (e.g. `*/dagnabit`, `*/go_list`). The branch (NATO
level) is intentionally left as a wildcard because packages get
repositioned across levels as their dependencies change — pinning the
level in docs creates lies the moment `dagnabit rename` runs.

**Leaf names MUST be unique across the entire dewey tree.** This is what
makes `*/<leaf>` an unambiguous reference. `dagnabit` enforces this:
attempting to introduce a second package with a leaf name already in use
elsewhere in the tree is a build-time error.

## Stability tiers

Not every package under this module is at the same maturity. When editing,
lean on the labels below. If you find behavior that contradicts them, update
this file.

### Battle-tested (safe to build on)

- `*/dagnabit` — Go package mover (reposition / move / rename / export),
  plus the type-aware leaf-rename analysis via `golang.org/x/tools/go/packages`.
  Validated end-to-end on maneater.
- `*/go_list` — shells `go list` and returns per-prefix dependency edges.
  Powers `dagnabit reposition`.
- `*/go_module` — reads the `module` directive out of `go.mod`.
- `*/nato_levels` — the NATO phonetic level table and the `LevelMapper`
  implementation.
- `*/topological_sort` — Kahn's algorithm on directed string edges.

### Experimental (treat as a contract in flux)

- `*/command` — CLI framework (`Utility` + `Command` + param/flag
  definitions + MCP/hook integration). Was mainlined before the interface
  settled; expect rough edges and missing features. `cmd/dagnabit` is
  planned as the tracer bullet to mature it (see purse-first#45 and the
  follow-up tracer-bullet plan it links to).

## cmd/

`libs/dewey/cmd/` hosts binaries that belong to the dewey module itself
(e.g. static analyzers: `defererr`, `repool`, `seqerror`). External binaries
that consume dewey — notably `cmd/dagnabit` at the repo root — remain in
their own locations for now. Don't move `cmd/dagnabit` in here without a
matching issue update; the decomposition as of purse-first#45 intentionally
kept the public tool separate from reusable primitives.

## gclplugin/

`libs/dewey/gclplugin/` registers the three static analyzers (`defererr`,
`seqerror`, `repool`) as a golangci-lint **module plugin** so downstream
repos that already gate on `golangci-lint` can fold them into that run
instead of a separate `go vet -vettool` pass (see purse-first#120). It
imports the stable `pkgs/analyzer_<name>.Analyzer` facades and calls
`register.Plugin("dewey", New)`; `Settings` lets a consumer opt out per
analyzer (all enabled by default; `repool` is a no-op outside pool-owning
packages). This is additive — the `cmd/<name>` singlechecker + vettool
path stays for non-golangci consumers. Consumers wire it via
`.custom-gcl.yml` + `golangci-lint custom`; the package doc comment carries
the full snippet.

## Testing

Use `just test-dewey` from the repo root. It runs `go test -tags test
./libs/dewey/...`. The `test` build tag gates helpers that must not ship to
consumers. `just vet-dewey` is known to flag one residual issue in
`*/catgut` (the `noescape` helper trips the `unsafeptr` check by
design — see the comment on the function for why this is intentional);
treat that as prior art, not a regression.

## Related issues

- purse-first#45 — dagnabit integration status (decomposition done; CLI
  port to `golf/command` still pending).
- purse-first#46 — dagnabit adoption tracker across repos.
