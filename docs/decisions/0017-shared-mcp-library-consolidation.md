---
status: accepted
date: 2026-02-25
---

# Consolidate packages onto shared MCP libraries

## Context and Problem Statement

Before MCP V1, packages used different approaches to MCP server implementation. get-hubbed depended on an external `go-lib-mcp` library, chix used manual JSON-RPC request/response types and dispatch logic, while grit and lux already used the shared go-mcp library. This meant protocol upgrades required N separate implementations instead of a single library change.

## Considered Options

* Keep per-package MCP implementations
* Consolidate onto go-mcp (Go) and rust-mcp (Rust) shared libraries
* Use a code generation approach

## Decision Outcome

Chosen option: "Consolidate onto go-mcp and rust-mcp shared libraries", because it ensures protocol changes (like the V1 upgrade) are implemented once in the library and propagate to all packages automatically, accepting the upfront migration effort for get-hubbed and chix.

### Consequences

* Good, because V1 protocol enablement becomes a one-line change per package (register V1 providers instead of V0) after consolidation.
* Good, because tool annotations, structured output, and pagination come free from the library rather than being reimplemented per package.
* Good, because eliminating the external `go-lib-mcp` dependency from get-hubbed removes a fragile external coupling.
* Bad, because chix migration requires wrapping each tool function as a `rust-mcp` `Tool` trait impl, which has less prior art than the Go migration.
* Bad, because BATS tests may need adjustment if exact JSON field ordering or error message wording changes between library implementations.

## Pros and Cons of the Options

### Keep per-package MCP implementations

* Good, because no migration effort required.
* Bad, because every protocol upgrade (V1, future versions) must be implemented separately in get-hubbed's go-lib-mcp integration and chix's manual JSON-RPC.
* Bad, because bug fixes and security patches must be applied in multiple places.

### Consolidate onto go-mcp and rust-mcp shared libraries

* Good, because grit and lux already demonstrate the `command.App` pattern, providing a clear reference for get-hubbed migration.
* Good, because chix can delete all manual JSON-RPC types (`JsonRpcRequest`, `JsonRpcResponse`, `JsonRpcError`, `InitializeResult`, `Capabilities`, `ServerInfo`) and dispatch logic.
* Bad, because parameter type mismatches may surface during get-hubbed's raw JSON schema to `command.Param` conversion.

### Use a code generation approach

* Good, because tool definitions could be declarative with generated server stubs.
* Bad, because it adds a build-time code generation step and new tooling to maintain.
* Bad, because generated code is harder to debug and customize per-package.

## More Information

* Design: `docs/plans/2026-02-22-mcp-library-migration-design.md`
* Migration order: get-hubbed first (mechanical, clear reference in grit), then chix (more fiddly trait wrapping)
* Verification: `nix flake check`, existing BATS tests, manual stdin/stdout initialize handshake
