---
status: superseded
date: 2026-02-25
---

# Refactor purse-first as a reusable marketplace framework

> _Superseded by #110 (marketplace aggregation removed)._

## Context and Problem Statement

purse-first was a monolithic tool that built a single hardcoded marketplace. Other teams and organizations wanted to create their own Claude plugin marketplaces with different package sets, but the build logic was tightly coupled to purse-first's specific package list. There was no way to reuse the marketplace assembly machinery without forking the entire repository.

## Considered Options

* Keep purse-first as a single marketplace
* Extract `lib.mkMarketplace` as a reusable Nix function with a `nix flake init` template
* Fork purse-first for each new marketplace

## Decision Outcome

Chosen option: "Extract `lib.mkMarketplace` as a reusable Nix function with a `nix flake init` template", because it enables downstream consumers to create their own marketplaces with minimal configuration while purse-first dogfoods its own framework, and forking would create maintenance burden across diverging codebases.

### Consequences

* Good, because downstream marketplaces can scaffold with `nix flake init -t purse-first#marketplace` and customize only the package list and metadata.
* Good, because purse-first itself becomes the first consumer of `mkMarketplace`, ensuring the framework stays battle-tested.
* Bad, because the `mkMarketplace` API surface becomes a public contract that must be maintained with backward compatibility.

## Pros and Cons of the Options

### Keep purse-first as a single marketplace

* Good, because no API design or abstraction work required.
* Bad, because other teams must fork the entire repo and maintain their own diverging copy.
* Bad, because marketplace assembly logic cannot be shared or improved centrally.

### Extract `lib.mkMarketplace` as a reusable Nix function with a `nix flake init` template

* Good, because downstream consumers get a turnkey scaffold with CI, justfile, and devShell.
* Good, because the meta plugin (skills carrier) is generated automatically when skills are provided.
* Neutral, because the no-hooks variant is always generated alongside the main marketplace.
* Bad, because the framework function must handle both purse-first's own build (from source) and downstream builds (from flake input).

### Fork purse-first for each new marketplace

* Good, because each fork has full control with no abstractions.
* Bad, because fixes and improvements must be manually cherry-picked across forks.
* Bad, because marketplace assembly logic diverges quickly.

## More Information

See `docs/plans/2026-02-18-marketplace-framework-design.md` for the full `mkMarketplace` API, template contents, and implementation details.
