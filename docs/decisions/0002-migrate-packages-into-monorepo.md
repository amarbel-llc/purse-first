---
status: accepted
date: 2026-02-25
---

# Migrate Packages into Monorepo

## Context and Problem Statement

Circular dependencies exist between purse-first and its 6 packages (grit, get-hubbed, lux, chix, batman, tap-dancer). purse-first depends on all packages as flake inputs for marketplace generation, while packages depend back on purse-first for go-mcp, rust-mcp, the purse builder, and the CLI. This makes atomic cross-package changes impossible and complicates flake lock updates.

## Considered Options

* Keep separate repos with pinned versions
* Monorepo with all packages
* Partial consolidation (only Go packages)

## Decision Outcome

Chosen option: "Monorepo with all packages", because it eliminates circular dependencies and enables atomic cross-package changes with a single `nix flake check`, accepting a larger repository and the coordination overhead of a shared Go vendor hash.

All 6 packages moved to `packages/<name>/`. Go modules use a `go.work` workspace for local resolution. Per-package Nix build expressions live in `lib/packages/`. Big-bang migration on a single branch with git history preserved in archived original repos.

### Consequences

* Good, because circular flake dependencies are eliminated entirely.
* Good, because cross-package changes (library API updates, protocol upgrades) become atomic commits.
* Good, because a single `nix flake check` validates the entire ecosystem.
* Bad, because the repository is significantly larger and all Go packages share a single vendor hash.
* Bad, because releases must be coordinated across all packages rather than independently versioned.

## More Information

See [docs/plans/2026-02-21-monorepo-migration-design.md](../plans/2026-02-21-monorepo-migration-design.md) for the full migration design including per-package notes and Nix build structure.
