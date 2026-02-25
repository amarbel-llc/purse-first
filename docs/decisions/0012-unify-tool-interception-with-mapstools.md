---
status: accepted
date: 2026-02-25
---

# Unify tool interception with MapsTools

## Context and Problem Statement

Tool mapping originally only intercepted Bash commands via `MapsBash` on the `Command` struct. However, tools like Read, Grep, and Glob also needed interception -- for example, lux should redirect file reads of `.go` files to the LSP hover tool, and nix file reads should go through the nil language server. The `MapsBash` field could not express these non-Bash interception rules.

## Considered Options

* Keep `MapsBash` only, limiting interception to Bash commands
* Add separate `MapsRead`, `MapsGrep`, `MapsGlob` fields for each tool type
* Replace `MapsBash` with unified `MapsTools` using per-tool-type matchers

## Decision Outcome

Chosen option: "Replace `MapsBash` with unified `MapsTools` using per-tool-type matchers", because a single `ToolMapping` type with a `Replaces` field and appropriate matcher fields (CommandPrefixes for Bash, Extensions for file tools) handles all current and future tool types without proliferating separate fields on the Command struct.

### Consequences

* Good, because lux can declare `{Replaces: "Read", Extensions: [".go", ".py"]}` to intercept file reads for LSP-supported languages.
* Good, because multiple tools sharing the same `(Replaces, Extensions, CommandPrefixes)` key are consolidated into a single mapping entry with multiple tool suggestions.
* Bad, because all existing `MapsBash` declarations must be migrated to `MapsTools` with `Replaces: "Bash"`.

## Pros and Cons of the Options

### Keep `MapsBash` only

* Good, because no migration required.
* Bad, because Read, Grep, and Glob interception is impossible, leaving significant tool routing gaps.

### Add separate fields per tool type

* Good, because each field is self-documenting.
* Bad, because the `Command` struct grows a new field for every tool type Claude Code adds.
* Bad, because consolidation logic must handle multiple separate field types.

### Replace `MapsBash` with unified `MapsTools`

* Good, because the `ToolMapping` struct generalizes cleanly -- `Replaces` identifies the built-in tool, and matcher fields (`Extensions`, `CommandPrefixes`) apply as appropriate for each tool type.
* Good, because `GenerateMappings()` produces a single unified format with both extensions and command prefixes.
* Neutral, because the hook handler's `formatDenyReason` gains a `tool_prefix` field for dev-mode MCP server name resolution.
* Bad, because grit's existing `BashMapping` entries must be rewritten to `ToolMapping` entries.

## More Information

See `docs/plans/2026-02-18-dev-mcp-design.md` for the `ToolMapping` struct definition, `GenerateMappings()` output format, and the `dev-mcp` command that uses these mappings.
