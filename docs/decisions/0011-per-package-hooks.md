---
status: accepted
date: 2026-02-25
---

# Move from central to per-package hooks

## Context and Problem Statement

Tool routing hooks were managed centrally by purse-first, which meant the hook router had to be the marketplace build to discover mappings from `share/purse-first/`. The standalone CLI build had no plugin directory, so hooks silently stopped intercepting. More fundamentally, each package knows best which built-in tools it should intercept -- grit knows about git commands, lux knows about LSP-supported file reads -- but purse-first had to know about all of them.

## Considered Options

* Keep central hook routing in purse-first
* Per-package hooks where each package generates its own hooks
* Hybrid with a central registry and per-package matchers

## Decision Outcome

Chosen option: "Per-package hooks where each package generates its own hooks", because it eliminates the coupling between the router and the marketplace build, and each package can self-match its own tool interceptions without purse-first needing knowledge of any specific package.

### Consequences

* Good, because adding a new package no longer requires updating central hook infrastructure.
* Good, because Claude Code auto-discovers per-package `hooks/hooks.json` at session start, using the native plugin hook system.
* Bad, because each installed package with PreToolUse hooks spawns a separate process per tool call -- with 4 MCP packages, that is 4 process spawns per tool use (mitigated by Claude Code running hooks in parallel).

## Pros and Cons of the Options

### Keep central hook routing in purse-first

* Good, because single process handles all tool routing.
* Bad, because the router requires the marketplace build to discover mappings, breaking standalone CLI builds.
* Bad, because every new package requires changes to central hook code.

### Per-package hooks where each package generates its own hooks

* Good, because `generate-plugin` emits `hooks/hooks.json` and `hooks/pre-tool-use` alongside `plugin.json` at build time.
* Good, because matching logic moves into go-mcp's `command` package, so each package binary can self-match via its `hook` subcommand.
* Neutral, because `symlinkJoin` collects hooks directories automatically with no marketplace build changes.
* Bad, because purse-first loses `hook`, `post-hook`, and `session-end` subcommands, and `internal/hook/` and `internal/mapping/` are deleted.

### Hybrid with central registry and per-package matchers

* Good, because single process spawn per tool use.
* Bad, because still requires a central component that must know about all packages.
* Bad, because more complex than either pure approach.

## More Information

See `docs/plans/2026-02-23-per-package-hooks-design.md` for the generated file formats, plugin cache layout, and list of removed functionality.
