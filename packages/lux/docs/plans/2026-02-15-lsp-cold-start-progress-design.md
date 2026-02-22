# LSP Cold-Start Progress Tracking

## Problem

When an LSP starts cold and needs to compile/index an entire project (e.g. gopls loading packages, rust-analyzer indexing), requests made during this window either time out or return empty/error results. Lux currently drops all `$/progress` notifications and doesn't handle `window/workDoneProgress/create` requests, so there is no awareness of indexing state.

## Approach

Add a `ProgressTracker` component at the pool level that intercepts LSP progress protocol messages. Use it to gate MCP requests (block until ready) while letting LSP server requests flow through immediately (editors handle partial results natively).

## Components

### ProgressTracker (`internal/subprocess/progress.go`)

Embedded in `LSPInstance`. Tracks active work-done-progress tokens from the LSP.

**Data model:**

```go
type ProgressTracker struct {
    tokens         map[any]*ProgressToken
    mu             sync.RWMutex
    lastActivity   time.Time
    readyCh        chan struct{}
}

type ProgressToken struct {
    Title   string
    Message string
    Pct     *int
    Created time.Time
}
```

**Token lifecycle:**

1. LSP sends `window/workDoneProgress/create` request -> tracker registers token, handler responds `{}`
2. LSP sends `$/progress` with `kind: "begin"` -> tracker updates token title/message
3. LSP sends `$/progress` with `kind: "report"` -> tracker updates message/percentage, bumps `lastActivity`
4. LSP sends `$/progress` with `kind: "end"` -> tracker removes token; if none remain, closes `readyCh`

**Key methods:**

- `HandleCreate(tokenID)` - register new token
- `HandleProgress(tokenID, value)` - update/remove token based on kind
- `IsReady() bool` - true when no active tokens
- `WaitForReady(ctx, activityTimeout, hardTimeout) error` - blocks until ready or timeout
- `ActiveProgress() []ProgressToken` - snapshot for logging/reporting

**`WaitForReady` behavior:**

- If already ready, returns immediately
- Blocks on `readyCh`
- On each wake/poll: check `lastActivity` against `activityTimeout` (stalled detection), check total elapsed against `hardTimeout`
- Check `inst.State` - if no longer `Running`, return error immediately

**`readyCh` lifecycle:**

- Created when token count goes 0 -> 1
- Closed when token count goes 1 -> 0
- Concurrent waiters all unblock together

### Handler Integration

Both notification handler paths (LSP server and MCP server) intercept progress messages before existing logic.

**`window/workDoneProgress/create` (request from LSP):**
- Extract token ID from params
- Call `inst.Progress.HandleCreate(tokenID)`
- Respond with `jsonrpc.NewResponse(msg.ID, nil)`
- Do not forward to client (lux owns this)

**`$/progress` (notification from LSP):**
- Extract token ID and value from params
- Call `inst.Progress.HandleProgress(tokenID, value)`
- LSP server path: forward to editor client (standard behavior)
- MCP server path: log to stderr

**Handler factory change:**
The handler closure captures the pool reference and looks up the instance by name to access its `ProgressTracker`.

**Client capabilities:**
Set `Window.WorkDoneProgress = true` in the `ClientCapabilities` sent during `initialize` to tell the LSP it may use progress reporting.

**`$/` drop guard (`server/handler.go:97-98`):**
Stays as-is. That guard is for client->lux messages. Progress messages from LSP->lux flow through the notification handler, which is a separate code path.

### MCP Bridge Changes (`internal/mcp/bridge.go`)

Two-phase approach in `withDocument`:

1. **Wait for ready:** After `GetOrStart`, before the LSP call, check `inst.Progress.IsReady()`. If not ready, call `WaitForReady` with the per-LSP timeout config. During the wait, periodically log progress and send MCP `notifications/message` via the transport.

2. **Call with retry:** Once ready, proceed with existing `callWithRetry`. The "no views" retry logic stays as-is for cross-project reference errors.

**Progress reporting to MCP callers:**
Pass a callback `func(lspName, message string)` into the bridge at construction. The MCP server wires this to `transport.Write` with a `notifications/message` notification. Rate-limited to one message per second.

### LSP Server Path

No blocking. Requests forward immediately as today. The `$/progress` forwarding from the handler change lets editors show progress natively.

## Configuration

New optional fields on `[[lsp]]` entries in `lsps.toml`:

```toml
[[lsp]]
name = "gopls"
flake = "..."
extensions = ["go"]
wait_for_ready = true        # default: true
ready_timeout = "10m"        # default: 10m
activity_timeout = "30s"     # default: 30s
```

- `wait_for_ready` (bool) - MCP callers block until progress tokens drain. Default `true`.
- `ready_timeout` (duration) - hard cap on total wait. Default `10m`.
- `activity_timeout` (duration) - no progress within this window = stalled. Default `30s`.

These only affect MCP callers. LSP server path always forwards immediately.

Duration fields parsed via `time.ParseDuration` with a custom TOML type or string fields with a parse method.

## Error Handling

| Scenario | Behavior |
|----------|----------|
| LSP never sends progress tokens | `IsReady()` always true, no wait, existing retry handles transient errors |
| LSP sends create but never end | `activity_timeout` fires, request proceeds |
| Multiple concurrent MCP requests | All block on same `readyCh`, unblock together |
| LSP crashes during indexing | `WaitForReady` checks instance state, returns error immediately |
| New workspace folder triggers re-index | Fresh progress tokens arrive, subsequent `WaitForReady` blocks again |

## Testing

**ProgressTracker unit tests (`internal/subprocess/progress_test.go`):**
- Token lifecycle: create -> begin -> report -> end transitions
- Multiple tokens: not ready until all end
- `WaitForReady` returns immediately when ready
- `WaitForReady` unblocks when last token ends
- Activity timeout returns stalled error
- Hard timeout returns error even with flowing progress
- Instance failure returns error immediately
- No tokens registered: always ready

**Notification handler tests:**
- `window/workDoneProgress/create` -> tracker registers, handler returns success
- `$/progress` begin/report/end -> tracker state updates
- `$/progress` forwarded to client in LSP server path
- `$/progress` logged in MCP server path

**Bridge tests:**
- `wait_for_ready = true` blocks until progress completes
- `wait_for_ready = false` skips wait
- Progress callback invoked during wait
- Activity/hard timeout surface as errors

**Config tests:**
- Duration parsing
- Defaults applied when fields omitted
