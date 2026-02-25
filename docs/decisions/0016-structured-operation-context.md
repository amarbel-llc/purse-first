---
status: accepted
date: 2026-02-25
---

# Adopt structured operation context for go-mcp

## Context and Problem Statement

MCP tool implementations used ad-hoc error handling and flat text/JSON output. There was no consistent way to express nested operations, stream structured progress, or distinguish between domain failures, external errors, and exceptional control flow (skip, abort). Three problems converged: dodder's expensive `errors.As` type-switching for control flow, tap-dancer requiring explicit `TestPoint` construction, and MCP tool handlers lacking structured lifecycle events.

## Decision Drivers

* Tool handlers need nested operation reporting (subtasks within a tool call)
* Domain code should not think about output format -- writers handle rendering
* Control flow (skip, abort) should be semantically distinct from errors
* Cleanup callbacks need guaranteed execution with clear failure semantics

## Considered Options

* Keep ad-hoc error handling per tool
* Create structured `operation.Context` with Writer interface and panic-based control flow
* Use Go `context.Context` with custom values

## Decision Outcome

Chosen option: "Create structured operation.Context with Writer interface and panic-based control flow", because it provides a unified lifecycle model (Run/After/Must) that streams events to format-agnostic writers while using panics for exceptional control flow (skip, abort, retry), keeping tool implementation code clean and free of error-type boilerplate, accepting the unconventional use of panics in Go.

### Consequences

* Good, because `Run` provides nested operation tracking with automatic lifecycle events (`BeginOperation`/`EndOperation`) streamed to pluggable writers.
* Good, because `ControlFail`, `ControlWrap`, and `ControlSkip` capture file:line automatically via `runtime.Caller`, producing rich diagnostics without manual annotation.
* Good, because `Must` vs `After` gives clear cleanup semantics -- required cleanup that can flip success to failure vs best-effort cleanup.
* Good, because the Writer interface is format-agnostic, supporting TAP-14, JSON, structured logs, and MCP progress notifications from the same operation tree.
* Bad, because panic-based control flow is unconventional in Go and requires all callback boundaries (`Run`, `Must`, `After`) to use `recover`, adding complexity.
* Bad, because the `Context` interface is large (11 methods), which may be intimidating for simple tool implementations.
* Neutral, because this is independent of dodder's `alfa/errors` Context but designed for future migration compatibility.

## More Information

* Design: `docs/plans/2026-02-23-operation-context-design.md`
* Package: `libs/go-mcp/operation/`
* Three-tier error model: domain failure (`ControlFail`), external error (`ControlWrap`), plain `return err`
* TAP writer maps operation events to TAP-14 output via tap-dancer's existing `Writer`
