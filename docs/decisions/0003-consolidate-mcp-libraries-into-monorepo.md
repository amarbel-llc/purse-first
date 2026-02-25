---
status: accepted
date: 2026-02-25
---

# Consolidate MCP Libraries into Monorepo

## Context and Problem Statement

go-lib-mcp and rust-lib-mcp were separate repositories. Packages depended on them via git URLs, creating version coordination problems --- updating a library API required publishing to git first, then updating every consumer's go.sum or Cargo.lock. This was the root of a circular dependency chain: purse-first depended on packages that depended on libraries that lived outside purse-first.

## Considered Options

* Keep as separate repos with versioned releases
* Move into purse-first as `libs/`
* Use git submodules

## Decision Outcome

Chosen option: "Move into purse-first as `libs/`", because it enables same-commit library and consumer changes without version coordination, accepting that the libraries can no longer be independently versioned or consumed outside purse-first.

go-lib-mcp moved to `libs/go-mcp/` with Go workspace (`go.work`) for local cross-module resolution. rust-lib-mcp moved to `libs/rust-mcp/` with Cargo path dependencies. Clean copy without git history preservation.

### Consequences

* Good, because library API changes and consumer migrations happen in the same commit.
* Good, because the Go workspace resolves cross-references locally during development without publishing.
* Good, because Rust path dependencies eliminate Cargo git fetch overhead.
* Bad, because external consumers can no longer depend on the libraries independently.
* Bad, because library changes are not independently versioned --- a library bug fix requires a purse-first release.

## More Information

See [docs/plans/2026-02-16-eat-libs-design.md](../plans/2026-02-16-eat-libs-design.md) for the original migration plan.
