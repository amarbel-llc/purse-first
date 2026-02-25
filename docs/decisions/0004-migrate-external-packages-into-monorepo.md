---
status: accepted
date: 2026-02-25
---

# Migrate External Packages into Monorepo

## Context and Problem Statement

After the initial 6-package migration (ADR-0002), 4 more external packages (sandcastle, and-so-can-you-repo, potato, spinclass) remained as separate repositories with the same circular dependency problems. These packages depended on purse-first for CLI tooling and library access, while purse-first consumed them as flake inputs for marketplace generation.

## Considered Options

* Keep as external repos
* Migrate into monorepo following ADR-0002 pattern

## Decision Outcome

Chosen option: "Migrate into monorepo following ADR-0002 pattern", because the same circular dependency problems that motivated ADR-0002 apply equally to these packages, accepting the further increase in repository size.

All 4 packages moved to `packages/<name>/` with per-package Nix build expressions in `lib/packages/`. Go packages (potato, spinclass) added to `go.work`. spinclass was renamed from sweatshop during migration (binary, module path, and imports updated; `sweatfile` config name preserved).

### Consequences

* Good, because all purse-first packages are now in a single repository with no remaining circular flake dependencies.
* Good, because the monorepo pattern is proven from ADR-0002 and requires no new infrastructure.
* Bad, because the repository now contains packages in 4 languages (Go, Rust, TypeScript, Bash), increasing build complexity.

## More Information

See [docs/plans/2026-02-22-package-migration-design.md](../plans/2026-02-22-package-migration-design.md) for per-package migration details and the spinclass rename scope.
