---
status: dormant
date: 2026-05-29
promotion-criteria: n/a — shipped, then parked
---

# Lux Service Daemon

> **Dormant:** `lux` moved out of purse-first in commit `e1d6632`
> ("slim purse-first to framework-only") and is currently dormant in its
> entirety — it is not published in `amarbel-llc/moxy` or any other active
> repo. This FDR is retained for historical context. Paths in **More
> Information** are relative to the former `packages/lux/`.

## Motivation

Lux routes LSP requests from multiple clients (editors, AI assistants) to
language server subprocesses. In the simple model, each client spawns its own
`lux serve` process, which starts its own set of LSP subprocesses. With three
clients (editor + two Claude Code sessions), three copies of gopls and nil are
running, each consuming hundreds of MB of RAM and seconds of startup time.

The service daemon is a persistent background process that multiplexes many
clients onto a shared pool of LSP subprocesses. Clients connect via a Unix
socket; the daemon routes their requests to the appropriate LSP and fans out
LSP notifications back to all interested sessions.

## Architecture

Three registry layers, each with a distinct scope:

```
Daemon
  ├── SessionRegistry        — one entry per connected client
  └── WorkspaceRegistry      — one entry per workspace root
        └── Pool             — one LSPInstance per registered LSP
              └── LSPInstance — subprocess + jsonrpc.Conn + state machine
```

### Session Layer

A session represents one connected client. Sessions are created by a
`lux/session.register` request and destroyed when the connection closes.

Sessions carry:
- `WorkspaceRoot` — the project root this client is working in
- `ClientType` — `lsp` (editor), `mcp` (AI assistant), or `control` (management)
- `OpenDocs` — set of document URIs currently open in this session

**Document ref counting**: `SessionRegistry` tracks how many sessions have each
document open. `didOpen` is forwarded to the LSP only when the ref count goes
from 0 to 1. `didClose` is forwarded only when the ref count drops to 0. This
prevents redundant open/close churn when multiple sessions work on the same
file.

### Workspace Layer

A workspace is created on demand when the first session for a given root
registers. It loads the lux config for that root, creates a file-type router,
and initializes a subprocess pool.

Each workspace has its own pool of LSP instances, independent of other
workspaces. This means two projects with different Go module paths each get
their own gopls.

### Pool / LSP Instance State Machine

```
Idle → Starting → Running → Stopping → Stopped
                          → Failed
```

`GetOrStart` transitions an instance through this machine:

- **Idle/Failed**: build the LSP binary via `NixExecutor.Build()`, then
  `Execute()` to start the subprocess, then LSP `initialize` handshake, then
  `initialized` notification, then optional `workspace/didChangeConfiguration`
  for settings. Transitions to Running on success.
- **Starting**: poll-wait (50 ms intervals) until Running or Failed.
- **Running**: return immediately.

`Stop` sends LSP `shutdown` + `exit`, waits up to 5 seconds, then kills.

### Notification Broadcasting

LSP subprocesses send server-initiated messages (diagnostics, progress, etc.)
to the daemon. The daemon fans these out to all sessions registered in the
same workspace:

```
LSP subprocess
    │ textDocument/publishDiagnostics
    ▼
Pool handler (per lsp)
    │
    ▼
broadcastNotification(workspace, lspName, msg)
    │
    ├── SessionsForWorkspace(workspace) → [session1, session2, ...]
    └── rc.Notify(msg.Method, msg.Params) → each connected client
```

Server-to-client **requests** (e.g. `window/workDoneProgress/create`) are
acknowledged directly by the daemon rather than forwarded. This prevents
multiple clients from each sending a response to the same LSP request.

## Interface

### Unix Socket

Default path: `$XDG_RUNTIME_DIR/lux.sock`.

If started with socket activation (LISTEN_PID + LISTEN_FDS env vars matching
systemd's fd-passing protocol), the daemon inherits the listener fd and does not
create its own socket.

### Protocol Methods

All messages are JSON-RPC 2.0.

| Method | Direction | Purpose |
|--------|-----------|---------|
| `lux/session.register` | client→daemon | Register a new session; returns `session_id` |
| `lux/session.deregister` | client→daemon | Explicitly deregister a session |
| `lux/lsp.request` | client→daemon | Forward an LSP request to the workspace's pool |
| `lux/lsp.notification` | client→daemon | Forward an LSP notification |
| `lux/pool.status` | client→daemon | Query LSP subprocess states |
| `lux/pool.start` | client→daemon | Explicitly start a named LSP |
| `lux/pool.stop` | client→daemon | Explicitly stop a named LSP |
| `lux/warmup` | client→daemon | Pre-warm LSPs for a directory |

LSP notifications from the daemon to clients are forwarded using the original
LSP method names (e.g. `textDocument/publishDiagnostics`).

### Idle Shutdown

When configured with an idle timeout, the daemon shuts down after the timeout
duration with no active sessions. This supports on-demand startup: the daemon
is started when needed and exits when idle, with a service manager (or socket
activation) restarting it on next connection.

## Examples

**Session register:**

```json
→ {"jsonrpc":"2.0","id":1,"method":"lux/session.register","params":{"workspace_root":"/home/user/myproject","client_type":"mcp"}}
← {"jsonrpc":"2.0","id":1,"result":{"session_id":"a3f8c1b2"}}
```

**LSP request (hover):**

```json
→ {"jsonrpc":"2.0","id":2,"method":"lux/lsp.request","params":{"session_id":"a3f8c1b2","lsp_method":"textDocument/hover","lsp_params":{"textDocument":{"uri":"file:///home/user/myproject/main.go"},"position":{"line":10,"character":5}}}}
← {"jsonrpc":"2.0","id":2,"result":{"contents":{"kind":"markdown","value":"..."}}}
```

**Notification pushed from LSP to client:**

```json
← {"jsonrpc":"2.0","method":"textDocument/publishDiagnostics","params":{"uri":"file:///...","diagnostics":[...]}}
```

## Limitations

- **No health-check polling.** A TODO in `pool.go` notes that running LSPs are
  not probed. A process that crashes without updating its state machine entry
  will appear as Running until the next request fails.
- **Workspace config is loaded once.** Config is read when the workspace is
  first created. Changes to `lsps.toml` require restarting the affected workspace
  (stop all LSPs, then next connection recreates the workspace).
- **No per-session LSP routing.** All sessions in a workspace share the same
  pool. There is no mechanism to give one session access to an LSP that others
  do not have.
- **No connection authentication.** Any process that can reach the Unix socket
  path can register as a session. The socket permissions are the only access
  control.

## More Information

- Daemon: `packages/lux/internal/service/daemon.go`
- Session registry: `packages/lux/internal/service/session.go`
- Workspace registry: `packages/lux/internal/service/workspace.go`
- Protocol constants: `packages/lux/internal/service/protocol.go`
- Subprocess pool and state machine: `packages/lux/internal/subprocess/pool.go`
- Socket activation: `socketActivationFD()` in `daemon.go`
