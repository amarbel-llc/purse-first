# MCP Library Migration Design

## Goal

Consolidate all packages onto the shared MCP libraries (`go-mcp`, `rust-mcp`)
before enabling V1 protocol features. Clean foundation first.

## Current State

| Package    | Library           | Protocol | Issue                                      |
|------------|-------------------|----------|---------------------------------------------|
| grit       | go-mcp            | V0       | Already on shared library                   |
| lux        | go-mcp            | V0       | Already on shared library                   |
| get-hubbed | go-lib-mcp (old)  | V0       | External dependency, no command.App         |
| chix       | Manual JSON-RPC   | V0       | rust-mcp in Cargo.toml but unused           |
| tap-dancer | N/A               | N/A      | Not an MCP server                           |

Libraries define V1 as `2025-11-25` (matches latest official MCP spec). Version
negotiation and V1 provider interfaces are built. No package uses V1 yet.

## Workstream 1: get-hubbed -> go-mcp + command.App

Replace `go-lib-mcp` with `go-mcp` and adopt the `command.App` pattern used by
grit and lux.

### Changes

- Replace `go-lib-mcp` import with `github.com/amarbel-llc/purse-first/libs/go-mcp`
- Rewrite `internal/tools/registry.go` to return `*command.App`
- Convert each tool registration function from `r.Register(name, desc, schema,
  handler)` to `app.AddCommand(&command.Command{...})` with typed `Params`
- Add `MapsTools` declarations where appropriate
- Update `main.go` to use `app.RegisterMCPTools(registry)`
- Regenerate `gomod2nix.toml`
- Update `lib/packages/get-hubbed.nix` to remove special `go-lib-mcp` handling

### Preserved

- All tool handler logic (GitHub API calls, response formatting)
- Same tool names, parameters, and behavior
- BATS tests

### Risk

Parameter type mismatches during raw JSON schema to `command.Param` conversion.
Mitigated by running BATS tests after.

## Workstream 2: chix -> rust-mcp

Replace manual JSON-RPC implementation with `rust-mcp`'s `McpServerBuilder` +
`Tool` trait.

### Changes

- Delete manual types: `JsonRpcRequest`, `JsonRpcResponse`, `JsonRpcError`,
  `InitializeResult`, `Capabilities`, `ServerInfo`
- Delete manual dispatch: `handle_request()`, `dispatch()`,
  `handle_initialize()`, `handle_tools_list()`, `handle_resources_list()`,
  `handle_resources_read()`
- Replace `main.rs` IO loop with
  `McpServer::builder("chix", version).with_tool(...).build().run_stdio().await`
- Wrap each tool function as a `Tool` trait impl using existing `ToolInfo`
  entries for `name()`, `description()`, `input_schema()`

### Preserved

- All tool logic (nix command execution, output formatting, pagination)
- Resource implementations
- Skill/plugin structure
- BATS tests

### Risk

BATS tests may assert on exact JSON field ordering or error message wording.
Need to verify rust-mcp produces compatible output.

## Ordering

The workstreams are fully independent. Recommended order: **get-hubbed first,
then chix**.

- get-hubbed is more mechanical with a clear reference (grit)
- Eliminates the last external `go-lib-mcp` dependency as a clean milestone
- chix has more fiddly trait wrapping with less prior art

## Verification

For both workstreams:

1. `nix flake check`
2. Existing BATS tests
3. Manual smoke test: initialize handshake via stdin/stdout

## What This Unlocks

Once both packages are on the shared libraries:

- V1 is a one-line change per package (register V1 providers instead of V0)
- Tool annotations, structured output, pagination come free from the library
- Protocol upgrades happen in `libs/` instead of per-package
