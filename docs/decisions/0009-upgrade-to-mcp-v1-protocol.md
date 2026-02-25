---
status: accepted
date: 2026-02-25
---

# Upgrade to MCP V1 protocol

## Context and Problem Statement

All MCP packages (chix, get-hubbed, grit) used MCP V0 exclusively. The MCP V1 specification (2025-11-25) introduced protocol negotiation, server instructions, annotations, and structured metadata. Both shared libraries (go-mcp, rust-mcp) already had full V1 support, but the packages had not opted in.

## Considered Options

* Stay on V0 only
* Support V1 with V0 backward compatibility
* V1 only, dropping V0

## Decision Outcome

Chosen option: "Support V1 with V0 backward compatibility", because V1 enables richer tool descriptions and server instructions for capable clients while preserving compatibility with existing V0 clients, and dropping V0 would break current deployments.

### Consequences

* Good, because V1-capable clients get protocol negotiation and server instructions automatically.
* Good, because the upgrade path is minimal -- Go packages switch to `NewToolRegistryV1()` and `RegisterMCPToolsV1()`, Rust packages add `.instructions()` to the builder.
* Bad, because two protocol paths must be maintained and tested going forward.

## Pros and Cons of the Options

### Stay on V0 only

* Good, because no code changes required.
* Bad, because packages cannot use V1 features like annotations, instructions, or structured metadata.
* Bad, because clients that expect V1 negotiation get a degraded experience.

### Support V1 with V0 backward compatibility

* Good, because V0 clients continue to work unchanged.
* Good, because annotations and output schemas can be added incrementally in later phases.
* Neutral, because chix requires no tool changes -- V0 tools are auto-wrapped for V1 clients by rust-mcp.
* Bad, because both V0 and V1 response paths must be maintained.

### V1 only, dropping V0

* Good, because only one protocol path to maintain.
* Bad, because existing V0-only clients would break immediately.
* Bad, because premature -- V1 adoption is not yet universal.

## More Information

See `docs/plans/2026-02-22-mcp-v1-upgrade-design.md` for per-layer changes. Annotations (phase 2) are covered in ADR 0010.
