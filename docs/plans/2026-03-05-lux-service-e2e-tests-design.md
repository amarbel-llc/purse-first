# Lux Service End-to-End Test Design

## Problem

The lux service daemon has substantial implementation (protocol, session
registry, workspace registry, handler, daemon, LSP client proxy, launchd
install) but all integration-level tests are skipped. There is no way to verify
the daemon works end-to-end without manual testing.

## Approach

Three incremental layers, each independently valuable:

### Layer 1: Unskip existing daemon tests

The existing Go tests in `daemon_test.go`, `lspclient_test.go`, and
`integration_test.go` are structurally complete but all have `t.Skip()`. These
tests exercise session management, idle timeout, socket cleanup, and socket
activation detection — none send actual LSP requests, so nil config is
sufficient.

**Changes:** Remove `t.Skip()` from all tests. No production code changes
expected.

**Tests unskipped:**

- `daemon_test.go`: 9 tests (accept/register, deregister on disconnect,
  multiple clients, idle timeout, stale socket removal, 5 socket activation FD
  tests, socket activation listener inheritance)
- `lspclient_test.go`: 2 tests (`WrapRequest`, `ProxyRoundTrip`)
- `integration_test.go`: 1 test (full register → status → deregister round-trip)

### Layer 2: Executor injection for LSP round-trip tests

**Goal:** Test the full daemon path — client connects, registers, sends an LSP
request, daemon routes it to an LSP subprocess, response comes back — without
requiring Nix.

**Injection point:** Add an optional `ExecutorFactory` field to
`WorkspaceRegistry`. When set, `createWorkspace` uses it instead of
`NewNixExecutor()`. Production code never sets it.

```go
type ExecutorFactory func() subprocess.Executor
```

In `createWorkspace`, one nil-check:

```go
executor := subprocess.NewNixExecutor()
if r.executorFactory != nil {
    executor = r.executorFactory()
}
```

**Fake LSP:** A test helper that implements the `Executor` interface. `Build`
returns a path to a test binary. `Execute` spawns a minimal JSON-RPC process
that:

- Responds to `initialize` with empty capabilities
- Responds to `shutdown` with null
- Echoes other requests back with a canned response

**New tests enabled:**

- LSP request round-trip through daemon
- Notification broadcast to multiple sessions
- Workspace config → pool → routing path

### Layer 3: BATS integration tests

New `zz-tests_bats/lux_service.bats` testing the daemon as a user would.

**Isolation:** `setup_test_home` sets `XDG_RUNTIME_DIR` into the test tmpdir.
`config.SocketPath()` reads `XDG_RUNTIME_DIR`, so the daemon socket is
automatically isolated per test.

**Setup/teardown:** Start `lux service run` in background during setup, kill and
wait in teardown.

**Test cases:**

1. `service_run_creates_socket` — start daemon, verify socket exists
2. `service_status_returns_json` — start daemon, run `lux service status`,
   verify JSON with `session_count: 0`
3. `service_idle_shutdown` — start daemon with short idle timeout, verify it
   exits
4. `service_cleans_up_socket_on_shutdown` — kill daemon, verify socket removed

## Files touched

| File | Change |
|------|--------|
| `packages/lux/internal/service/daemon_test.go` | Remove `t.Skip()` from 9 tests |
| `packages/lux/internal/service/lspclient_test.go` | Remove `t.Skip()` from 2 tests |
| `packages/lux/internal/service/integration_test.go` | Remove `t.Skip()`, add LSP round-trip test |
| `packages/lux/internal/service/workspace.go` | Add `ExecutorFactory` field, nil-check in `createWorkspace` |
| `packages/lux/internal/service/fake_lsp_test.go` | New: fake executor + fake LSP process for tests |
| `zz-tests_bats/lux_service.bats` | New: BATS integration tests for daemon CLI |

## Rollback

No existing behavior is changed. The `ExecutorFactory` field defaults to nil,
preserving current production path. All new tests can be reverted by removing
the test files and reverting the one nil-check in `workspace.go`.
