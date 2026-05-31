---
status: superseded
date: 2026-02-25
---

# Use relative directory paths for self-contained marketplace

> _Superseded by #110 (marketplace aggregation removed)._

## Context and Problem Statement

Marketplace.json referenced packages via GitHub repository URLs. Since all packages now live in the monorepo, this caused every package to resolve to the same repo root, losing individual MCP identity -- Claude Code would clone the monorepo and find only the root `bob` plugin.json instead of each package's own manifest. Network access during builds was also an unnecessary fragile dependency.

## Considered Options

* Keep GitHub URL references
* Switch to relative directory paths
* Use Nix store paths

## Decision Outcome

Chosen option: "Switch to relative directory paths", because packages are already output to `share/purse-first/<name>/` by the Nix build, so the marketplace can reference them as `./share/purse-first/<name>` without network access or external coupling, accepting that this pattern only works when the marketplace and packages are co-located in the same output tree.

### Consequences

* Good, because each package resolves to its own `plugin.json`, restoring correct MCP identity for all packages.
* Good, because builds no longer require network access to resolve package sources.
* Good, because the pattern matches the official Claude marketplace (`claude-plugins-official`) convention for local plugins.
* Bad, because marketplace entries are only valid relative to the build output root -- they cannot be consumed standalone without the full output tree.
* Neutral, because `marketplace-config.json` per-plugin `repo` fields are removed, but the top-level repo field remains for metadata.

## More Information

* Design: `docs/plans/2026-02-22-self-contained-marketplace-design.md`
* Implementation: `internal/marketplace/generate.go` -- replace GitHub source resolution with relative path computation.
* The relative path pattern is builder-agnostic; any build system producing the standard `$out/share/purse-first/<name>/plugin.json` layout will work.
