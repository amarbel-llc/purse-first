# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with this repository.

## Overview

Lux is an LSP multiplexer written in Go that routes LSP requests to multiple language servers based on file type. It also functions as an MCP server, exposing LSP capabilities as tools for AI assistants. Language servers are built and run via Nix flakes, started on-demand, and managed through a subprocess pool.

## Build & Development Commands

```sh
just build            # Nix build (produces ./result)
just build-go         # Quick go build for dev iteration (runs gomod2nix first)
just test             # Run all Go tests
just test-v           # Verbose test output
just fmt              # Format Go + shell code
just lint             # go vet
just deps             # go mod tidy + gomod2nix regeneration
just build-gomod2nix  # Regenerate gomod2nix.toml only
```

Run a single test:
```sh
nix develop --command go test -v -run TestName ./internal/config/
```

After changing `go.mod`, always run `just deps` to regenerate `gomod2nix.toml` (required for Nix builds).

## Architecture

### Request Flow

Client (editor/Claude) sends JSON-RPC to Lux. The **Handler** (`internal/server/handler.go`) receives it, extracts the document URI, and asks the **Router** (`internal/server/router.go`) which LSP owns that file. The **Pool** (`internal/subprocess/pool.go`) starts the LSP subprocess on-demand via **NixExecutor** (`internal/subprocess/nix.go`), which runs `nix build <flake>` and caches the result path. Requests are forwarded over stdin/stdout pipes to the subprocess; responses are relayed back.

### Key Packages

| Package | Role |
|---------|------|
| `cmd/lux` | Cobra CLI: `serve`, `add`, `list`, `status`, `start`, `stop`, `format`, `mcp {stdio,sse,http}`, `genman` |
| `internal/server` | LSP server, handler, and file-type router |
| `internal/subprocess` | LSP process pool, lifecycle state machine (Idle→Starting→Running→Stopping→Stopped), Nix executor |
| `internal/mcp` | MCP server, bridge (adapts LSP ops to MCP tools), tool registry, resources, prompts |
| `internal/config` | TOML config parsing (`lsps.toml`, `formatters.toml`), per-project overrides, config merging |
| `internal/formatter` | External formatter routing and execution (separate from LSP formatting) |
| `internal/capabilities` | Auto-discovery and caching of LSP capabilities during `lux add` |
| `internal/lsp` | LSP protocol types, capability aggregation, URI utilities |
| `internal/transport` | MCP transport layers: stdio, SSE, streamable HTTP |
| `internal/control` | Unix socket for management commands (status/start/stop) |
| `pkg/filematch` | File matching by extension, glob pattern, or language ID (priority: languageID > extension > pattern) |

### Configuration

- User config: `~/.config/lux/lsps.toml` (TOML, `[[lsp]]` entries)
- Formatter config: `~/.config/lux/formatters.toml`
- Cached capabilities: `~/.local/share/lux/capabilities/`
- Control socket: `$XDG_RUNTIME_DIR/lux.sock`
- Per-project overrides load from the project root directory

### LSP Config Fields

Each `[[lsp]]` entry supports: `name`, `flake`, `binary` (optional, for multi-binary flakes), `extensions`, `patterns`, `language_ids`, `args`, `env`, `init_options`, `settings`, `settings_key`, and `capabilities` (with `disable`/`enable` lists). At least one of `extensions`/`patterns`/`language_ids` is required.

## Nix Flake

Uses the `nixpkgs` (stable) + `nixpkgs-master` (unstable) convention. Built with `buildGoApplication` from the Go devenv overlay. The flake also generates man pages (cobra/doc for section 1, scdoc for section 5) and provides an `install-mcp` app that configures `~/.claude/mcp.json`.

## Key Dependencies

- `github.com/amarbel-llc/go-lib-mcp` - MCP protocol library (JSON-RPC handler, transport interfaces)
- `github.com/spf13/cobra` - CLI framework
- `github.com/BurntSushi/toml` - Config parsing
- `github.com/gobwas/glob` - Glob pattern matching for file routing
