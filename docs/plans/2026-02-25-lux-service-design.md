# Lux Service: Server-Client Architecture Design

## Status

Proposed

## Context

Lux is currently a monolithic LSP multiplexer. Each invocation of `lux serve`
or `lux mcp` is an independent process that owns its own LSP subprocess pool,
config, and state. This means:

- Multiple editors/AI agents each spawn their own LSP processes (wasted memory,
  redundant Nix builds, duplicated startup time)
- LSP state (indexing, diagnostics, warmup) is lost on editor restart
- No way to have multiple simultaneous clients (editor + MCP + CLI) backed by
  the same LSP pool

## Decision

Transition lux to a server-client model where a persistent service owns the LSP
pool and clients are thin proxies.

## Design

### Component Architecture

Four components:

**1. `lux service run`** — The always-on daemon. Owns:

- LSP subprocess pool (shared across all clients)
- Session registry (per-client document state, capability negotiation)
- Workspace registry (per-workspace LSP pools and config)
- Config loading and watching
- Nix build cache
- MCP HTTP/SSE endpoint (direct access for non-stdio clients)
- Unix socket listener for JSON-RPC clients

**2. `lux lsp`** (renamed from `lux serve`) — Thin LSP proxy:

- Connects to the service over Unix socket
- Registers a session
- Proxies JSON-RPC between stdin/stdout and the service
- On disconnect, deregisters its session

**3. `lux mcp stdio`** — Thin MCP proxy:

- Connects to the service over Unix socket
- Registers a session
- Proxies MCP JSON-RPC between stdin/stdout and the service

**4. `lux` CLI** (status, start, stop, warmup, list) — Management commands:

- Connect to the service over Unix socket
- Issue control commands
- No session needed — fire-and-forget queries

### Service Lifecycle

**Socket-activated via launchd (macOS) / systemd (Linux):**

- The OS service manager owns the socket at `$XDG_RUNTIME_DIR/lux.sock`
- First client connection triggers service launch
- Service exits gracefully after 30 min idle (no active sessions)
- OS relaunches on next socket connection
- Zero overhead when unused, instant availability on demand

**Single global service** — one process manages all workspaces. Workspace pools
are keyed by the workspace root path provided during session registration.

**Management commands:**

- `lux service install` — Generate + load launchd plist / systemd units
- `lux service uninstall` — Unload + remove
- `lux service status` — Show workspaces, sessions, pools
- `lux service logs` — Tail service log

### Session Model

Each client registers a session on connect:

```
Client connects → lux/session.register(workspace_root, client_type) → session_id
Client disconnects → lux/session.deregister(session_id) → cleanup
```

Per-session state:

- Open documents (which files this client has opened)
- Client capabilities (negotiated during initialize)
- Pending request IDs (for routing responses back)

**Document sharing:** Last-writer-wins with reference-counted open/close.

- First session to `didOpen` a file sends it to the LSP
- Last session to `didClose` sends it to the LSP
- Any session can send `didChange` — LSP sees the latest version
- Works naturally for the common editor-writes/AI-reads pattern

**Response routing:** Each request gets a service-internal ID. The service maps
service↔LSP request IDs and routes responses back to the originating session.

**Notifications:** LSP notifications (diagnostics, progress) are broadcast to
all sessions that have the relevant file open.

**Cleanup:** On disconnect, deregister session, decrement doc ref counts, close
docs that reach zero refs.

### Protocol

JSON-RPC over Unix socket with `lux/*` method namespace:

#### Session Management

```jsonc
// Register
{"jsonrpc": "2.0", "method": "lux/session.register", "id": 1,
 "params": {"workspace_root": "/path/to/project", "client_type": "lsp"}}
// → {"jsonrpc": "2.0", "id": 1, "result": {"session_id": "abc123"}}

// Deregister
{"jsonrpc": "2.0", "method": "lux/session.deregister", "id": 2,
 "params": {"session_id": "abc123"}}
```

#### LSP Forwarding

```jsonc
// Request (client → service → LSP)
{"jsonrpc": "2.0", "method": "lux/lsp.request", "id": 3,
 "params": {"session_id": "abc123",
            "lsp_method": "textDocument/completion",
            "lsp_params": { /* standard LSP params */ }}}

// Notification (service → client, broadcast)
{"jsonrpc": "2.0", "method": "lux/lsp.notification",
 "params": {"session_id": "abc123",
            "lsp_method": "textDocument/publishDiagnostics",
            "lsp_params": { /* standard LSP params */ }}}
```

#### Control Commands

```jsonc
{"jsonrpc": "2.0", "method": "lux/pool.status", "id": 4, "params": {}}
{"jsonrpc": "2.0", "method": "lux/pool.start", "id": 5,
 "params": {"name": "gopls"}}
{"jsonrpc": "2.0", "method": "lux/pool.stop", "id": 6,
 "params": {"name": "gopls"}}
{"jsonrpc": "2.0", "method": "lux/warmup", "id": 7,
 "params": {"dir": "/path"}}
```

### MCP Access

Two paths to the same backing state:

1. **`lux mcp stdio`** — Thin proxy for Claude Code CLI. Proxies MCP protocol
   through the service.
2. **Service HTTP/SSE endpoint** — Direct MCP access for other clients. Reuses
   existing `internal/transport/` code (http.go, sse.go).

### CLI Commands (Revised)

| Command | Role |
|---------|------|
| `lux lsp` | Thin proxy: stdin/stdout LSP ↔ service |
| `lux mcp stdio` | Thin proxy: stdin/stdout MCP ↔ service |
| `lux service install` | Generate + load OS service config |
| `lux service uninstall` | Unload + remove OS service config |
| `lux service run` | Daemon entrypoint (OS calls this) |
| `lux service status` | Show workspaces, sessions, pools |
| `lux service logs` | Tail service log |
| `lux status` | Query LSP pool status |
| `lux start <name>` | Start an LSP |
| `lux stop <name>` | Stop an LSP |
| `lux warmup <dir>` | Pre-start LSPs for a directory |
| `lux list` | Show filetype routing table |
| `lux add` | Add LSP/formatter/filetype config |
| `lux fmt <file>` | External formatter |

### Error Handling

- **Service not running** → client gets connection refused → clear error:
  "run `lux service install`"
- **LSP crash** → service restarts it (existing pool behavior) → sessions
  get re-initialized
- **Service crash** → OS restarts it → clients reconnect → sessions
  re-register
- **Idle timeout** → service exits → OS relaunches on next connection →
  transparent to clients

### Future Extension: Multi-Workspace

The protocol already supports multi-workspace via the `workspace_root` param in
session registration. The service manages per-workspace pools internally. Future
work could add cross-workspace operations (e.g., find references across
projects) without protocol changes.

## Consequences

- LSP processes are shared across editors and AI agents — reduced memory and
  startup time
- LSP state (indexing, diagnostics) persists across editor restarts
- Multiple simultaneous clients (editor + MCP + CLI) backed by the same pool
- Socket activation means zero overhead when not in use
- Added complexity: session management, request routing, reference counting
- `lux serve` renamed to `lux lsp` — breaking change for editor configs
