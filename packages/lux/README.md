# Lux

Lux is an LSP (Language Server Protocol) multiplexer that routes requests to multiple language servers based on file type. It also functions as an MCP (Model Context Protocol) server, exposing LSP capabilities as tools for AI assistants like Claude Code.

## Features

- **LSP Multiplexing**: Routes LSP requests to the correct language server based on file extension, glob patterns, or language IDs
- **MCP Server**: Exposes LSP capabilities (hover, go-to-definition, completions, etc.) as MCP tools
- **Nix-based**: Uses Nix flakes to build and run language servers reproducibly
- **On-demand startup**: Language servers start lazily when first needed
- **Multiple transports**: Supports stdio, SSE, and streamable HTTP

## Installation

### Using Nix

```bash
nix build github:amarbel-llc/lux
```

### Install as MCP Server for Claude Code

```bash
nix run github:amarbel-llc/lux#install-mcp
```

This adds lux to your `~/.claude/mcp.json` configuration.

## Configuration

Lux reads its configuration from `~/.config/lux/lsps.toml`.

### Configuration Structure

```toml
# Optional: custom socket path for control commands
socket = "/tmp/lux.sock"

[[lsp]]
name = "gopls"                    # Unique identifier
flake = "nixpkgs#gopls"           # Nix flake reference
extensions = ["go"]               # File extensions (without dot)
patterns = ["*.go", "go.mod"]     # Glob patterns
language_ids = ["go"]             # LSP language identifiers
args = []                         # Additional command-line arguments
```

### Configuration Fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Unique identifier for this LSP |
| `flake` | Yes | Nix flake reference (e.g., `nixpkgs#gopls`) |
| `extensions` | * | File extensions to match (without leading `.`) |
| `patterns` | * | Glob patterns for filenames |
| `language_ids` | * | LSP language identifiers |
| `args` | No | Additional arguments to pass to the LSP |

\* At least one of `extensions`, `patterns`, or `language_ids` is required.

## Adding a New LSP

There are two ways to add a new language server to lux:

### Method 1: Using `lux add` (Recommended)

The `add` command automatically discovers LSP capabilities and configures file type matching:

```bash
lux add nixpkgs#gopls
```

This will:
1. Build the flake
2. Start the LSP to discover its capabilities
3. Cache the capabilities for faster startup
4. Add the configuration to `~/.config/lux/lsps.toml`

#### Examples

```bash
# Go
lux add nixpkgs#gopls

# Python
lux add nixpkgs#pyright

# TypeScript/JavaScript
lux add nixpkgs#nodePackages.typescript-language-server

# Nix
lux add nixpkgs#nil

# Rust
lux add nixpkgs#rust-analyzer

# Lua
lux add nixpkgs#lua-language-server
```

After adding, you may need to edit the config to adjust file extensions or patterns if they weren't auto-detected.

### Method 2: Manual Configuration

Edit `~/.config/lux/lsps.toml` directly:

```toml
[[lsp]]
name = "gopls"
flake = "nixpkgs#gopls"
extensions = ["go"]
language_ids = ["go"]

[[lsp]]
name = "pyright"
flake = "nixpkgs#pyright"
extensions = ["py", "pyi"]
language_ids = ["python"]

[[lsp]]
name = "typescript-language-server"
flake = "nixpkgs#nodePackages.typescript-language-server"
extensions = ["js", "ts", "jsx", "tsx", "mjs", "cjs"]
language_ids = ["javascript", "typescript", "javascriptreact", "typescriptreact"]

[[lsp]]
name = "nil"
flake = "nixpkgs#nil"
extensions = ["nix"]
language_ids = ["nix"]

[[lsp]]
name = "rust-analyzer"
flake = "nixpkgs#rust-analyzer"
extensions = ["rs"]
language_ids = ["rust"]

[[lsp]]
name = "lua-language-server"
flake = "nixpkgs#lua-language-server"
extensions = ["lua"]
language_ids = ["lua"]
```

### Using Custom Flakes

You can reference any Nix flake that provides an LSP:

```toml
[[lsp]]
name = "my-lsp"
flake = "github:owner/repo#my-lsp"
extensions = ["xyz"]
```

Or a local flake:

```toml
[[lsp]]
name = "dev-lsp"
flake = "/path/to/flake#lsp"
extensions = ["xyz"]
```

## Usage

### LSP Server Mode

Start lux as an LSP server (reads from stdin, writes to stdout):

```bash
lux serve
```

This mode is used by editors that support LSP.

### MCP Server Mode

Run lux as an MCP server to expose LSP capabilities to Claude:

```bash
# Over stdio (for Claude Code)
lux mcp stdio

# Over SSE
lux mcp sse --addr :8080

# Over streamable HTTP
lux mcp http --addr :8081
```

### Management Commands

```bash
# List configured LSPs
lux list

# Check status of running LSPs
lux status

# Start an LSP eagerly
lux start gopls

# Stop a running LSP
lux stop gopls
```

## MCP Tools

When running as an MCP server, lux exposes these tools:

| Tool | Description |
|------|-------------|
| `hover` | Get type information and documentation at a position |
| `definition` | Go to the definition of a symbol |
| `references` | Find all references to a symbol |
| `completion` | Get code completions at a position |
| `format` | Format a document |
| `document_symbols` | List all symbols in a document |
| `code_action` | Get available code actions at a position |
| `rename` | Rename a symbol across the codebase |

## Development

### Prerequisites

- Nix with flakes enabled
- direnv (optional but recommended)

### Building

```bash
# Full nix build
just build

# Quick go build for development
just build-go
```

### Testing

```bash
just test
just test-v  # verbose
```

### Formatting

```bash
just fmt
```

### Running Locally

```bash
# LSP server
just serve

# MCP server
just mcp-stdio
```

## File Locations

| Path | Description |
|------|-------------|
| `~/.config/lux/lsps.toml` | Configuration file |
| `~/.local/share/lux/capabilities/` | Cached LSP capabilities |
| `$XDG_RUNTIME_DIR/lux.sock` | Control socket |

## Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Client    │────▶│     Lux     │────▶│    gopls    │
│  (Editor/   │     │ (Multiplexer)│     └─────────────┘
│   Claude)   │     │             │     ┌─────────────┐
└─────────────┘     │   Router    │────▶│   pyright   │
                    │             │     └─────────────┘
                    │   Pool      │     ┌─────────────┐
                    │             │────▶│     nil     │
                    └─────────────┘     └─────────────┘
```

1. Client sends LSP request to lux
2. Router determines which LSP handles the file type
3. Pool starts the LSP on-demand if not running
4. Request is forwarded to the appropriate LSP
5. Response is returned to the client

## License

MIT
