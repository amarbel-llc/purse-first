# Echo MCP Dummy Package Design

## Purpose

A minimal MCP server for testing and development that echoes tool arguments
back with metadata and logs all JSON-RPC traffic to a file.

## Location

`dummies/go/` — top-level directory, separate from `packages/`. Added to
`go.work` only (no flake.nix, no marketplace registration).

## Architecture

Single binary using the raw go-mcp server layer (`server.NewToolRegistryV1` +
`transport.NewStdio`). No `command.App` — unnecessary for a dummy package.

### Components

1. **Echo tool** — A single tool called `echo` that accepts arbitrary JSON
   arguments and returns them along with metadata (tool name, timestamp,
   argument count).

2. **Logging transport** — A wrapper around `transport.NewStdio` that writes
   every JSON-RPC message (reads and writes) to `echo-mcp.jsonl` in the
   current directory, one JSON object per line with direction metadata.

### Data Flow

```
Claude -> stdin -> LoggingTransport.Read() -> Server -> echo handler -> Server -> LoggingTransport.Write() -> stdout
                        |                                                             |
                   echo-mcp.jsonl                                                echo-mcp.jsonl
```

## File Structure

```
dummies/go/
  cmd/echo-mcp/
    main.go          # Entry point, server setup, echo handler, logging transport
  go.mod
  go.sum
```

## Build

- Added to `go.work`
- Build with `go build ./dummies/go/cmd/echo-mcp`
- No Nix build, no marketplace registration
