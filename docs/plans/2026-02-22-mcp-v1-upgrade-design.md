# MCP V1 Upgrade Design

Date: 2026-02-22

## Goal

Enable V1 (2025-11-25) protocol negotiation across all three MCP packages
(chix, get-hubbed, grit). When a V1-capable client connects, the server
responds with V1 protocol version and server instructions. Tools continue
returning standard content --- annotations and output schemas are future phases.

## Background

Both shared libraries (go-mcp, rust-mcp) already have full V1 support:
version constants, negotiation logic, V1 types, V1 registries. The packages
just need to opt in.

## Changes by Layer

### go-mcp library (`libs/go-mcp/`)

- Add `RegisterMCPToolsV1(*server.ToolRegistryV1)` to `command.App` ---
  converts commands to `protocol.ToolV1` structs (Name, Description,
  InputSchema, no annotations)
- Add `resultToMCPV1(*Result) *protocol.ToolCallResultV1` helper alongside
  existing `resultToMCP`
- No negotiation changes needed --- `ToolRegistryV1` implements
  `ToolProviderV1`, so `hasV1Providers()` returns true automatically

### get-hubbed (`packages/get-hubbed/`)

- Switch `main.go` from `NewToolRegistry()` to `NewToolRegistryV1()`
- Switch `app.RegisterMCPTools(registry)` to `app.RegisterMCPToolsV1(registry)`
- Convert `RegisterAPITools` to accept `*server.ToolRegistryV1` and return
  `*protocol.ToolCallResultV1`
- Add `Options.Instructions` describing the server

### grit (`packages/grit/`)

- Same pattern as get-hubbed: `NewToolRegistryV1()`,
  `RegisterMCPToolsV1()`, `Options.Instructions`

### chix (`packages/chix/`)

- Add `.instructions("...")` to `McpServerBuilder` --- automatically enables
  V1 negotiation
- No other changes needed (V0 tools are auto-wrapped for V1 clients)

## Out of Scope

- Tool annotations (read-only, destructive, idempotent, open-world hints) ---
  phase 2
- Output schemas and structured content --- phase 3
- New V1-only capabilities (completions, tasks, elicitation, logging)
- V0 backward compatibility is maintained --- V0 clients still get V0 responses
