# Design: `purse-first validate-mcp`

## Problem

MCP server packages built on purse-first have no framework-level way to validate
that they start, respond to protocol calls, and return well-formed results.
Chrest rolled its own `test-mcp` justfile recipe using
`@modelcontextprotocol/inspector --cli`; this pattern should be reusable without
requiring Node.js.

Resolves: https://github.com/amarbel-llc/purse-first/issues/1

## Approach

Implement validation directly in Go using the existing `jsonrpc.Conn` and
`protocol` types from go-mcp. No external dependencies. The validator spawns the
MCP binary as a subprocess, communicates over stdio JSON-RPC, runs a fixed set of
checks, and reports results.

## CLI Interface

```
purse-first validate-mcp <binary> [args...]
purse-first validate --type mcp <binary> [args...]
```

Both forms are equivalent. The existing `validate` command gains `mcp` as a new
document type that spawns a process instead of reading a JSON file.

## Checks

1. **Initialization** — send `initialize` request with `InitializeParamsV1`,
   wait for `InitializeResultV1`, send `initialized` notification. Verify the
   response deserializes correctly and contains a protocol version.

2. **tools/list** — call `tools/list`, verify:
   - Response deserializes into `ToolsListResultV1`
   - At least one tool is returned
   - Every tool has a non-nil `annotations` field

3. **resources/list** — call `resources/list`, verify:
   - Response deserializes into `ResourcesListResultV1`
   - No assertion on count (not all servers have resources)

4. **resources/templates/list** — call `resources/templates/list`, verify:
   - Response deserializes into `ResourceTemplatesListResultV1`
   - No assertion on count

5. **Clean shutdown** — close stdin, verify the process exits within the timeout.

## Implementation

- **`internal/validate/mcp.go`** — core validation logic. Spawns subprocess,
  creates `jsonrpc.Conn` over stdin/stdout pipes, runs checks sequentially,
  collects warnings/errors.
- **`cmd/purse-first/main.go`** — wire up `validate-mcp` subcommand and extend
  `validate --type mcp` to delegate to it.
- **Timeout** — hardcoded 10s for the entire validation sequence.
- **Output** — follows existing validate pattern: per-check status lines,
  exit 0 on success, non-zero on any error.

## Not in Scope

- Recursing into individual resources or templates
- Asserting specific annotation values
- Configuration file for expected capabilities
- Custom timeout flag
- Testing tool invocations (tools/call)

These can be added later as usage stabilizes.

## Rollback

This is purely additive — new subcommand and new `--type` value. No existing
behavior changes. Rollback is removing the code.
