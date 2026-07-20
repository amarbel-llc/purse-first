# Lux Service E2E Tests Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to
> implement this plan task-by-task.

**Goal:** Make the lux service daemon testable end-to-end across three
incremental layers: unskip existing Go tests, add executor injection with a fake
LSP for round-trip tests, add BATS CLI integration tests.

**Architecture:** Layer 1 removes `t.Skip()` from existing daemon tests that
exercise session management without LSP requests. Layer 2 adds a single
`ExecutorFactory` injection point to `WorkspaceRegistry` so tests can substitute
a fake LSP. Layer 3 adds BATS tests that exercise the real binary.

**Tech Stack:** Go testing, `jsonrpc` package, BATS + bats-island

**Rollback:** Purely additive — revert test files and one nil-check in
`workspace.go`.

---

### Task 1: Unskip socket activation FD tests

These are the simplest — pure env-var logic, no daemon startup.

**Files:**
- Modify: `packages/lux/internal/service/daemon_test.go:285-350`

**Step 1: Remove `t.Skip()` from 5 socket activation tests**

Remove the `t.Skip()` line from each of:
- `TestSocketActivationFD_NotSet`
- `TestSocketActivationFD_WrongPID`
- `TestSocketActivationFD_Detected`
- `TestSocketActivationFD_ZeroFDs`
- `TestSocketActivationFD_InvalidPID`
- `TestSocketActivationFD_InvalidFDs`

**Step 2: Run tests to verify they pass**

Run: `nix develop --command go test -v -run TestSocketActivationFD ./packages/lux/internal/service/`
Expected: All 6 tests PASS

**Step 3: Commit**

```
test(lux): unskip socket activation FD detection tests
```

---

### Task 2: Unskip daemon socket lifecycle tests

These start a real daemon on a temp Unix socket — test accept, disconnect
cleanup, stale socket removal, and socket activation listener inheritance.

**Files:**
- Modify: `packages/lux/internal/service/daemon_test.go:41-88,90-150,152-218,251-283,352-415`

**Step 1: Remove `t.Skip()` from daemon lifecycle tests**

Remove `t.Skip()` from:
- `TestDaemon_AcceptAndRegister`
- `TestDaemon_DeregisterOnDisconnect`
- `TestDaemon_MultipleClients`
- `TestDaemon_RemovesStaleSocket`
- `TestDaemon_SocketActivationInheritsListener`

**Step 2: Run tests to verify they pass**

Run: `nix develop --command go test -v -run "TestDaemon_(Accept|Deregister|Multiple|RemovesStale|SocketActivation)" ./packages/lux/internal/service/`
Expected: All 5 tests PASS

If any test fails due to timing, adjust `waitForSocket` timeout or add
`waitForListeningSocket` where socket file existence precedes accept readiness.

**Step 3: Commit**

```
test(lux): unskip daemon socket lifecycle tests
```

---

### Task 3: Unskip idle timeout test

This test uses short timeouts (200ms idle, 30ms check interval) and verifies
the daemon exits and cleans up.

**Files:**
- Modify: `packages/lux/internal/service/daemon_test.go:220-249`

**Step 1: Remove `t.Skip()` from `TestDaemon_IdleTimeout`**

**Step 2: Run test to verify it passes**

Run: `nix develop --command go test -v -run TestDaemon_IdleTimeout ./packages/lux/internal/service/`
Expected: PASS — daemon exits within 5s, socket removed

If flaky, increase `idleCheckInterval` from 30ms to 50ms.

**Step 3: Commit**

```
test(lux): unskip daemon idle timeout test
```

---

### Task 4: Unskip LSP client and integration tests

These test register/deregister over the socket and request wrapping — no LSP
requests involved.

**Files:**
- Modify: `packages/lux/internal/service/lspclient_test.go:13-45,79-140`
- Modify: `packages/lux/internal/service/integration_test.go:13-97`

**Step 1: Remove `t.Skip()` from 3 tests**

- `TestLSPClient_WrapRequest` (lspclient_test.go:14)
- `TestLSPClient_ProxyRoundTrip` (lspclient_test.go:80)
- `TestIntegration_FullRoundTrip` (integration_test.go:14)

**Step 2: Run tests to verify they pass**

Run: `nix develop --command go test -v -run "TestLSPClient_Wrap|TestLSPClient_Proxy|TestIntegration" ./packages/lux/internal/service/`
Expected: All 3 PASS

**Step 3: Run all service tests together**

Run: `nix develop --command go test -v ./packages/lux/internal/service/`
Expected: All service tests PASS (no more `t.Skip` in the package)

**Step 4: Commit**

```
test(lux): unskip LSP client and integration round-trip tests
```

---

### Task 5: Add executor factory injection to WorkspaceRegistry

One production code change: let tests inject a custom executor.

**Files:**
- Modify: `packages/lux/internal/service/workspace.go:29-34,62-68`

**Step 1: Add the ExecutorFactory type and field**

In `workspace.go`, add after the `WorkspaceRegistry` struct definition:

```go
type WorkspaceRegistry struct {
	workspaces      map[string]*Workspace
	baseCfg         *config.Config
	broadcaster     NotificationBroadcaster
	executorFactory func() subprocess.Executor // nil = use NewNixExecutor
	mu              sync.RWMutex
}
```

**Step 2: Use the factory in createWorkspace**

In `createWorkspace`, replace:

```go
executor := subprocess.NewNixExecutor()
```

with:

```go
var executor subprocess.Executor
if r.executorFactory != nil {
    executor = r.executorFactory()
} else {
    executor = subprocess.NewNixExecutor()
}
```

**Step 3: Verify existing tests still pass**

Run: `nix develop --command go test -v ./packages/lux/internal/service/`
Expected: All tests PASS (executorFactory is nil, so production path unchanged)

**Step 4: Commit**

```
feat(lux): add executor factory injection to WorkspaceRegistry
```

---

### Task 6: Write fake executor and fake LSP test helper

Create a fake executor that spawns a minimal JSON-RPC process using `io.Pipe`
pairs — no external binary needed.

**Files:**
- Create: `packages/lux/internal/service/fake_lsp_test.go`

**Step 1: Write the fake executor**

```go
package service

import (
	"context"
	"encoding/json"
	"io"

	"github.com/amarbel-llc/lux/internal/subprocess"
	"code.linenisgreat.com/purse-first/libs/go-mcp/jsonrpc"
)

// fakeExecutor implements subprocess.Executor using in-process pipes.
// Build is a no-op; Execute spawns a goroutine that speaks JSON-RPC.
type fakeExecutor struct{}

func (e *fakeExecutor) Build(_ context.Context, _, _ string) (string, error) {
	return "/fake/lsp", nil
}

func (e *fakeExecutor) Execute(_ context.Context, _ string, _ []string, _ map[string]string, _ string) (*subprocess.Process, error) {
	// Create pipe pairs: what the pool writes to stdin, the fake reads;
	// what the fake writes to stdout, the pool reads.
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	done := make(chan struct{})

	// Spawn fake LSP handler
	go func() {
		defer close(done)
		conn := jsonrpc.NewConn(stdinR, stdoutW, func(_ context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
			return handleFakeLSP(msg)
		})
		conn.Run(context.Background())
	}()

	return &subprocess.Process{
		Stdin:  stdinW,
		Stdout: stdoutR,
		Stderr: io.NopCloser(&discardReader{}),
		Wait: func() error {
			<-done
			return nil
		},
		Kill: func() error {
			stdinR.Close()
			stdoutW.Close()
			return nil
		},
	}, nil
}

func handleFakeLSP(msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	if msg.ID == nil {
		// Notification — ignore
		return nil, nil
	}
	switch msg.Method {
	case "initialize":
		result := map[string]any{
			"capabilities": map[string]any{},
		}
		return jsonrpc.NewResponse(*msg.ID, result)
	case "shutdown":
		return jsonrpc.NewResponse(*msg.ID, nil)
	default:
		// Echo the method back as a text result
		result := map[string]any{
			"echo": msg.Method,
		}
		return jsonrpc.NewResponse(*msg.ID, result)
	}
}

type discardReader struct{}

func (d *discardReader) Read(_ []byte) (int, error) {
	select {} // Block forever; stderr logger will be stopped by Kill
}
```

**Step 2: Verify it compiles**

Run: `nix develop --command go build ./packages/lux/internal/service/`
Expected: No errors (test file won't be included in build, but vet it):

Run: `nix develop --command go vet ./packages/lux/internal/service/`
Expected: No errors

**Step 3: Commit**

```
test(lux): add fake executor and fake LSP for service integration tests
```

---

### Task 7: Write LSP round-trip integration test

Test the full path: daemon → register → configure workspace with fake LSP →
send LSP request → get response.

**Files:**
- Modify: `packages/lux/internal/service/integration_test.go`

**Step 1: Write the failing test**

Append to `integration_test.go`:

```go
func TestIntegration_LSPRequestRoundTrip(t *testing.T) {
	socketPath := t.TempDir() + "/lux.sock"

	cfg := &config.Config{
		LSPs: []config.LSP{
			{
				Name:       "fake",
				Flake:      "fake#lsp",
				Extensions: []string{"go"},
			},
		},
	}

	d := NewDaemon(socketPath, cfg, 0)
	d.workspaces.executorFactory = func() subprocess.Executor {
		return &fakeExecutor{}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	waitForListeningSocket(t, socketPath, 2*time.Second)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dialing socket: %v", err)
	}
	defer conn.Close()

	client := jsonrpc.NewConn(conn, conn, nil)
	go client.Run(ctx)

	// Register session
	regResult, err := client.Call(ctx, MethodSessionRegister, RegisterParams{
		WorkspaceRoot: t.TempDir(),
		ClientType:    ClientTypeLSP,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	var reg RegisterResult
	json.Unmarshal(regResult, &reg)

	// Send LSP request through the daemon
	lspResult, err := client.Call(ctx, MethodLSPRequest, LSPRequestParams{
		SessionID: reg.SessionID,
		LSPMethod: "textDocument/hover",
		LSPParams: json.RawMessage(`{"textDocument":{"uri":"file:///test.go"},"position":{"line":0,"character":0}}`),
	})
	if err != nil {
		t.Fatalf("LSP request: %v", err)
	}

	// Verify we got a response from the fake LSP
	var lspResp map[string]any
	if err := json.Unmarshal(lspResult, &lspResp); err != nil {
		t.Fatalf("unmarshal LSP response: %v", err)
	}

	if lspResp["echo"] != "textDocument/hover" {
		t.Errorf("expected echo of method, got: %v", lspResp)
	}

	cancel()
	<-errCh
}
```

Add imports at top of file: `"net"`, `"time"`,
`"github.com/amarbel-llc/lux/internal/config"`,
`"github.com/amarbel-llc/lux/internal/subprocess"`.

**Step 2: Run test to verify it passes**

Run: `nix develop --command go test -v -run TestIntegration_LSPRequestRoundTrip ./packages/lux/internal/service/`
Expected: PASS

This test exercises: daemon socket → handler → workspace registry (with fake
executor) → pool → fake LSP → response back through handler.

If routing fails ("no LSP matched for method"), the issue is that the Router
needs filetype configs to match `file:///test.go` → `.go` extension → `fake`
LSP. The `config.LSP` has `Extensions: []string{"go"}` but the Router is built
from `filetype.Config` not `config.LSP`. If this mismatch occurs, create a
filetype config in a temp dir and set `XDG_CONFIG_HOME` to point there, OR
adjust the test to register the fake LSP and set up a filetype config directly.

**Step 3: Commit**

```
test(lux): add LSP round-trip integration test through daemon
```

---

### Task 8: Run full lux test suite

Verify nothing is broken across the entire lux package.

**Files:** None — validation only

**Step 1: Run all lux tests**

Run: `nix develop --command go test -v ./packages/lux/...`
Expected: All tests PASS

**Step 2: Run all Go tests**

Run: `nix develop --command go test ./...`
Expected: No regressions

---

### Task 9: Write BATS integration tests for daemon CLI

**Files:**
- Create: `zz-tests_bats/lux_service.bats`

**Step 1: Write the BATS test file**

```bash
#!/usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  setup_test_home
  # lux uses XDG_RUNTIME_DIR for socket path; set_xdg sets XDG_RUNTIME_HOME
  export XDG_RUNTIME_DIR="$BATS_TEST_TMPDIR/.xdg/runtime"
  mkdir -p "$XDG_RUNTIME_DIR"
  result_path="${PURSE_FIRST_RESULT:-$BATS_CWD/result}"
  lux="$result_path/bin/lux"
}

teardown() {
  if [[ -n "${daemon_pid:-}" ]]; then
    kill "$daemon_pid" 2>/dev/null || true
    wait "$daemon_pid" 2>/dev/null || true
  fi
  teardown_test_home
}

start_daemon() {
  "$lux" service run &
  daemon_pid=$!
  # Wait for socket to appear
  local socket="$XDG_RUNTIME_DIR/lux.sock"
  local deadline=$((SECONDS + 5))
  while [[ ! -S "$socket" ]] && [[ $SECONDS -lt $deadline ]]; do
    sleep 0.05
  done
  [[ -S "$socket" ]]
}

function service_run_creates_socket { # @test
  start_daemon
  [[ -S "$XDG_RUNTIME_DIR/lux.sock" ]]
}

function service_status_returns_json { # @test
  start_daemon
  run "$lux" service status
  assert_success
  assert_output --partial '"session_count"'
}

function service_cleans_up_socket_on_shutdown { # @test
  start_daemon
  local socket="$XDG_RUNTIME_DIR/lux.sock"
  [[ -S "$socket" ]]
  kill "$daemon_pid"
  wait "$daemon_pid" 2>/dev/null || true
  daemon_pid=""
  # Socket should be removed
  [[ ! -e "$socket" ]]
}
```

**Step 2: Run the BATS tests**

Requires `nix build` first so `result/bin/lux` exists.

Run:
```
nix build
nix develop --command result-batman/bin/bats --tap zz-tests_bats/lux_service.bats
```

Expected: All 3 tests PASS

**Step 3: Commit**

```
test(lux): add BATS integration tests for service daemon CLI
```

---

### Task 10: Add lux service BATS to justfile

**Files:**
- Modify: `justfile`

**Step 1: Add a `test-lux-service` target**

Add after the existing `test-lifecycle` target:

```just
test-lux-service: build-batman
    nix build
    {{cmd_nix_dev}} {{cmd_batman_bats}} zz-tests_bats/lux_service.bats
```

And add `test-lux-service` to the `test` target's dependency list alongside
`test-lifecycle` and `test-integration`.

**Step 2: Run the new target**

Run: `just test-lux-service`
Expected: PASS

**Step 3: Commit**

```
build: add test-lux-service justfile target
```
