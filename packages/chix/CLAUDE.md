# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
just build          # Build with nix (preferred)
just dev            # Build with cargo (dev mode)
just test           # Run tests
just check          # Run cargo check + clippy
just fmt            # Format code
nix flake check     # Validate flake and run all checks
```

## Architecture

chix is a Claude Code plugin that combines a Nix MCP server with skills for working with Nix-backed codebases.

### Components

- **MCP server** (`src/`) - Rust binary exposing Nix and FlakeHub CLI operations as tools via JSON-RPC 2.0 over stdin/stdout
- **Skills** (`skills/`) - Claude Code skills for Nix codebase workflows
- **Plugin manifest** (`.claude-plugin/plugin.json`) - Registers the MCP server and hooks with Claude Code
- **Hooks** (`.claude-plugin/hooks/`) - PostToolUse hook that auto-formats `.nix` files after Edit/Write

### Request Flow

1. `main.rs` - Entry point, reads JSON-RPC requests line-by-line from stdin
2. `server.rs` - Dispatches requests to handlers (`initialize`, `tools/list`, `tools/call`)
3. `tools/mod.rs` - Tool registry with `list_tools()` returning all tool definitions and parameter structs
4. `tools/*.rs` - Individual tool implementations (build, flake, run, eval, log, flakehub, nil LSP)
5. `nix_runner.rs` - Executes `nix` and `fh` CLI commands with timeout (300s default) and process cleanup
6. `validators.rs` - Input validation: flake refs, installables, shell metacharacter blocking

### Adding a New Tool

1. Add implementation in `src/tools/` (new file or existing)
2. Add `ToolInfo` entry in `tools/mod.rs::list_tools()` with name, description, and JSON schema
3. Add parameter struct in `tools/mod.rs`
4. Add match arm in `server.rs::call_tool()`
5. Export from `tools/mod.rs`

### Security

All inputs are validated before execution:
- `validate_installable()` / `validate_flake_ref()` - Whitelist regex for flake references
- `validate_no_shell_metacharacters()` - Blocks `;|&`$(){}\\<>!`
- `validate_args()` - Validates argument arrays

Commands use `kill_on_drop(true)` to ensure cleanup on timeout.

### Nix Build

The wrapped package (`packages.default`) includes `fh`, `cachix`, and `nil` in PATH via `makeWrapper`. System `nix` is used directly for compatibility with Determinate Nix settings. The build uses crane for Rust compilation.
