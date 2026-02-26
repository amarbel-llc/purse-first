# mgp Catalog Resource Design

## Problem

mgp's `query` tool requires Claude to spend output tokens writing a GraphQL
query and input tokens reading the result every time it wants to discover
available tools. MCP resources offer a more context-efficient alternative: the
host application can include resource content passively, without a tool
round-trip.

## Solution

Expose the tool catalog as a single MCP resource at `mgp://catalog`. The host
can read it once and include it in context when relevant, giving Claude
awareness of available tools without burning a query round-trip.

## Design Decisions

- **Single resource** — one URI (`mgp://catalog`) returns the entire catalog.
  Per-server resources add complexity without clear benefit at this scale.
- **Coexists with query tool** — the resource provides passive context; the
  query tool provides active filtered lookups. Different use cases.
- **JSON format** — structured, machine-readable, consistent with GraphQL
  responses.
- **Metadata only** — name, title, description, and annotations per tool. No
  inputSchema (Claude gets that from `exec`'s schema when it actually calls a
  tool).
- **Approach 1 (inline in main.go)** — no new packages or abstractions. The
  catalog is already available in main; register the resource there.

## Resource Definition

| Field | Value |
|-------|-------|
| URI | `mgp://catalog` |
| Name | `Tool Catalog` |
| Description | Complete catalog of tools available across all MCP servers |
| MimeType | `application/json` |

## Content Schema

```json
{
  "servers": [
    {
      "name": "grit",
      "tools": [
        {
          "name": "status",
          "title": "Show working tree status",
          "description": "...",
          "readOnly": true,
          "destructive": false,
          "idempotent": true
        }
      ]
    }
  ]
}
```

Annotation fields (`readOnly`, `destructive`, `idempotent`) are omitted when
null.

## Implementation

All changes in `packages/mgp/cmd/mgp/main.go`:

1. Create `server.NewResourceRegistry()`
2. Register one resource (`mgp://catalog`) with a reader that serializes the
   catalog
3. Pass `Resources: resourceRegistry` to `server.Options`

The reader closure captures the catalog. Since the catalog is built once at
startup and never mutated, no synchronization is needed.

## What Doesn't Change

- `query` tool (stays as-is)
- `exec` tool (stays as-is)
- Catalog discovery logic
- go-mcp library
- No new packages or files
