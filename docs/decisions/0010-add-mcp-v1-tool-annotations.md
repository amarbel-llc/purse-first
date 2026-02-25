---
status: accepted
date: 2026-02-25
---

# Add MCP V1 annotations to all tools

## Context and Problem Statement

MCP V1 introduced tool annotations (Title, ReadOnlyHint, DestructiveHint, IdempotentHint, OpenWorldHint) for tool metadata. With approximately 80 tools across chix (37), get-hubbed (20), grit (17), and lux, these annotations needed to be added systematically to give clients accurate behavioral hints about each tool.

## Considered Options

* No annotations -- minimal V1 support only
* Annotate all tools with Title and behavior hints
* Annotate only a subset of high-impact tools

## Decision Outcome

Chosen option: "Annotate all tools with Title and behavior hints", because consistent annotations across the entire tool surface give clients reliable behavioral metadata, and partial annotation would create an inconsistent experience where some tools have hints and others do not.

### Consequences

* Good, because clients can use ReadOnlyHint and DestructiveHint to surface appropriate confirmation UX or auto-approve safe operations.
* Good, because Title fields provide human-readable display names distinct from the machine-oriented tool names.
* Bad, because all ~80 tools must be individually audited for correct annotation values, and maintaining accuracy as tools evolve adds ongoing burden.

## Pros and Cons of the Options

### No annotations -- minimal V1 support only

* Good, because no per-tool audit or maintenance needed.
* Bad, because the primary user-facing benefit of V1 (behavioral hints) is not delivered.

### Annotate all tools with Title and behavior hints

* Good, because Go packages get `Title` and `Annotations` fields on `command.Command`, propagated through `RegisterMCPToolsV1` automatically.
* Good, because Rust packages implement the `ToolV1` trait with `title()` and `annotations()` methods, switching registration from `.with_tool()` to `.with_tool_v1()`.
* Bad, because annotation values require careful judgment (e.g., whether `nix build` is destructive) and may need revision as understanding evolves.

### Annotate only a subset of high-impact tools

* Good, because less upfront work.
* Bad, because an inconsistent annotation surface is worse than none -- clients cannot reliably distinguish "no annotation" from "not destructive."

## More Information

See `docs/plans/2026-02-22-mcp-v1-annotations-plan.md` for the complete annotation table covering all 80 tools and the per-task implementation plan.
