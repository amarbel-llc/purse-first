---
status: accepted
date: 2026-02-25
---

# Adopt Unified Command Framework

## Context and Problem Statement

Go packages used a mix of `flag`, cobra, and custom command parsing. Rust packages used clap. Each package maintained tool metadata in three independent places --- `ToolRegistry.Register()` calls, `PluginBuilder` chains, and hand-written help text. Adding a tool required touching all three, and they drifted. There was no single source of truth for a tool's name, parameters, description, and bash mappings.

## Considered Options

* Standardize on cobra for Go + clap for Rust
* Create unified `command` package in go-mcp (and later rust-mcp)
* Keep per-package approach

## Decision Outcome

Chosen option: "Create unified `command` package in go-mcp", because a single `Command` struct declaration produces CLI parsing, MCP tool registration, plugin.json, mappings.json, manpages, and shell completions from one source of truth, accepting the upfront cost of migrating all existing packages and the open design questions around handler ergonomics.

Go implementation first (grit, then lux, then get-hubbed), then mirror to Rust with a struct-based API followed by `#[derive(Command)]` proc macros. The `purse` builder package is deprecated once all Go consumers migrate. Nix integration via `postInstall` calling the built-in `generate-all` subcommand.

### Consequences

* Good, because tool metadata is defined once and cannot drift between CLI help, MCP schema, plugin.json, and manpages.
* Good, because adding a new tool is a single struct declaration instead of touching 3+ files.
* Good, because manpages and shell completions are generated automatically for every package.
* Bad, because all existing packages must be migrated from their current CLI frameworks.
* Bad, because the unified handler return type (`*ToolCallResult`) may not be natural for CLI-only commands --- handler ergonomics remain an open design question.

## More Information

See [docs/plans/2026-02-16-unified-command-framework-design.md](../plans/2026-02-16-unified-command-framework-design.md) for the full data model, generation outputs, Rust derive macro design, and phased migration plan.
