# LSP Cold-Start Progress Tracking — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Gate MCP requests behind LSP work-done-progress tracking so cold-start indexing doesn't cause timeouts or empty results.

**Architecture:** A `ProgressTracker` on each `LSPInstance` intercepts `window/workDoneProgress/create` and `$/progress` messages from the LSP subprocess. MCP callers block on `WaitForReady` before making LSP calls; LSP server callers forward progress to the editor and proceed immediately.

**Tech Stack:** Go, standard `testing` package, `github.com/amarbel-llc/go-lib-mcp/jsonrpc`

---

### Task 1: Config — Duration type and new LSP fields

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Step 1: Write failing tests for new config fields**

Add to `internal/config/config_test.go`:

```go
func TestLSP_ReadinessFields_TOML(t *testing.T) {
	input := `
name = "gopls"
flake = "nixpkgs#gopls"
extensions = ["go"]
wait_for_ready = false
ready_timeout = "5m"
activity_timeout = "15s"
`
	var lsp LSP
	if err := toml.Unmarshal([]byte(input), &lsp); err != nil {
		t.Fatalf("failed to parse TOML: %v", err)
	}

	if lsp.WaitForReady == nil || *lsp.WaitForReady != false {
		t.Errorf("expected WaitForReady=false, got %v", lsp.WaitForReady)
	}
	if lsp.ReadyTimeout != "5m" {
		t.Errorf("expected ReadyTimeout=5m, got %q", lsp.ReadyTimeout)
	}
	if lsp.ActivityTimeout != "15s" {
		t.Errorf("expected ActivityTimeout=15s, got %q", lsp.ActivityTimeout)
	}
}

func TestLSP_ReadinessFields_Defaults(t *testing.T) {
	input := `
name = "gopls"
flake = "nixpkgs#gopls"
extensions = ["go"]
`
	var lsp LSP
	if err := toml.Unmarshal([]byte(input), &lsp); err != nil {
		t.Fatalf("failed to parse TOML: %v", err)
	}

	if lsp.WaitForReady != nil {
		t.Errorf("expected WaitForReady=nil (default), got %v", *lsp.WaitForReady)
	}

	readyTimeout := lsp.ReadyTimeoutDuration()
	if readyTimeout != 10*time.Minute {
		t.Errorf("expected default ReadyTimeout=10m, got %v", readyTimeout)
	}

	activityTimeout := lsp.ActivityTimeoutDuration()
	if activityTimeout != 30*time.Second {
		t.Errorf("expected default ActivityTimeout=30s, got %v", activityTimeout)
	}
}

func TestLSP_ReadinessFields_InvalidDuration(t *testing.T) {
	input := `
name = "gopls"
flake = "nixpkgs#gopls"
extensions = ["go"]
ready_timeout = "not-a-duration"
`
	var lsp LSP
	if err := toml.Unmarshal([]byte(input), &lsp); err != nil {
		t.Fatalf("failed to parse TOML: %v", err)
	}

	// Should fall back to default when parsing fails
	readyTimeout := lsp.ReadyTimeoutDuration()
	if readyTimeout != 10*time.Minute {
		t.Errorf("expected fallback ReadyTimeout=10m, got %v", readyTimeout)
	}
}

func TestLSP_ShouldWaitForReady(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name     string
		lsp      LSP
		expected bool
	}{
		{"nil defaults to true", LSP{WaitForReady: nil}, true},
		{"explicit true", LSP{WaitForReady: &trueVal}, true},
		{"explicit false", LSP{WaitForReady: &falseVal}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.lsp.ShouldWaitForReady(); got != tt.expected {
				t.Errorf("ShouldWaitForReady() = %v, want %v", got, tt.expected)
			}
		})
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `nix develop --command go test -v -run 'TestLSP_Readiness|TestLSP_ShouldWait' ./internal/config/`
Expected: FAIL — fields and methods don't exist yet.

**Step 3: Add fields and methods to config**

In `internal/config/config.go`, add to the `LSP` struct:

```go
type LSP struct {
	// ... existing fields ...
	WaitForReady    *bool  `toml:"wait_for_ready,omitempty"`
	ReadyTimeout    string `toml:"ready_timeout,omitempty"`
	ActivityTimeout string `toml:"activity_timeout,omitempty"`
}
```

Add methods:

```go
func (l *LSP) ShouldWaitForReady() bool {
	if l.WaitForReady == nil {
		return true
	}
	return *l.WaitForReady
}

func (l *LSP) ReadyTimeoutDuration() time.Duration {
	if l.ReadyTimeout == "" {
		return 10 * time.Minute
	}
	d, err := time.ParseDuration(l.ReadyTimeout)
	if err != nil {
		return 10 * time.Minute
	}
	return d
}

func (l *LSP) ActivityTimeoutDuration() time.Duration {
	if l.ActivityTimeout == "" {
		return 30 * time.Second
	}
	d, err := time.ParseDuration(l.ActivityTimeout)
	if err != nil {
		return 30 * time.Second
	}
	return d
}
```

Add `"time"` to the imports.

**Step 4: Run tests to verify they pass**

Run: `nix develop --command go test -v -run 'TestLSP_Readiness|TestLSP_ShouldWait' ./internal/config/`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add wait_for_ready, ready_timeout, activity_timeout config fields

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 2: ProgressTracker — Core data structure and token lifecycle

**Files:**
- Create: `internal/subprocess/progress.go`
- Create: `internal/subprocess/progress_test.go`

**Step 1: Write failing tests for token lifecycle**

Create `internal/subprocess/progress_test.go`:

```go
package subprocess

import (
	"encoding/json"
	"testing"
	"time"
)

func TestProgressTracker_NewIsReady(t *testing.T) {
	pt := NewProgressTracker()
	if !pt.IsReady() {
		t.Error("new tracker should be ready")
	}
}

func TestProgressTracker_CreateMakesNotReady(t *testing.T) {
	pt := NewProgressTracker()
	pt.HandleCreate("token-1")
	if pt.IsReady() {
		t.Error("tracker with active token should not be ready")
	}
}

func TestProgressTracker_FullLifecycle(t *testing.T) {
	pt := NewProgressTracker()

	pt.HandleCreate("token-1")
	if pt.IsReady() {
		t.Error("should not be ready after create")
	}

	pt.HandleProgress("token-1", progressValue(t, "begin", "Loading packages...", "", nil))
	if pt.IsReady() {
		t.Error("should not be ready after begin")
	}

	pct := 50
	pt.HandleProgress("token-1", progressValue(t, "report", "", "50% done", &pct))
	if pt.IsReady() {
		t.Error("should not be ready after report")
	}

	pt.HandleProgress("token-1", progressValue(t, "end", "", "", nil))
	if !pt.IsReady() {
		t.Error("should be ready after end")
	}
}

func TestProgressTracker_MultipleTokens(t *testing.T) {
	pt := NewProgressTracker()

	pt.HandleCreate("token-1")
	pt.HandleCreate("token-2")

	pt.HandleProgress("token-1", progressValue(t, "begin", "Task 1", "", nil))
	pt.HandleProgress("token-2", progressValue(t, "begin", "Task 2", "", nil))

	pt.HandleProgress("token-1", progressValue(t, "end", "", "", nil))
	if pt.IsReady() {
		t.Error("should not be ready with one token still active")
	}

	pt.HandleProgress("token-2", progressValue(t, "end", "", "", nil))
	if !pt.IsReady() {
		t.Error("should be ready after all tokens end")
	}
}

func TestProgressTracker_ActiveProgress(t *testing.T) {
	pt := NewProgressTracker()

	pt.HandleCreate("token-1")
	pt.HandleProgress("token-1", progressValue(t, "begin", "Indexing", "", nil))

	pct := 42
	pt.HandleProgress("token-1", progressValue(t, "report", "", "Processing files", &pct))

	active := pt.ActiveProgress()
	if len(active) != 1 {
		t.Fatalf("expected 1 active token, got %d", len(active))
	}
	if active[0].Title != "Indexing" {
		t.Errorf("expected title 'Indexing', got %q", active[0].Title)
	}
	if active[0].Message != "Processing files" {
		t.Errorf("expected message 'Processing files', got %q", active[0].Message)
	}
	if active[0].Pct == nil || *active[0].Pct != 42 {
		t.Errorf("expected pct 42, got %v", active[0].Pct)
	}
}

func TestProgressTracker_LastActivity(t *testing.T) {
	pt := NewProgressTracker()
	pt.HandleCreate("token-1")
	pt.HandleProgress("token-1", progressValue(t, "begin", "Task", "", nil))

	before := pt.LastActivity()

	time.Sleep(5 * time.Millisecond)
	pt.HandleProgress("token-1", progressValue(t, "report", "", "working", nil))

	after := pt.LastActivity()
	if !after.After(before) {
		t.Error("lastActivity should advance after report")
	}
}

func TestProgressTracker_EndUnknownTokenIsNoop(t *testing.T) {
	pt := NewProgressTracker()
	// Should not panic
	pt.HandleProgress("unknown", progressValue(t, "end", "", "", nil))
	if !pt.IsReady() {
		t.Error("should still be ready after ending unknown token")
	}
}

func TestProgressTracker_NumericTokenID(t *testing.T) {
	pt := NewProgressTracker()
	pt.HandleCreate(json.Number("42"))
	if pt.IsReady() {
		t.Error("should not be ready after create with numeric token")
	}
	pt.HandleProgress(json.Number("42"), progressValue(t, "begin", "Task", "", nil))
	pt.HandleProgress(json.Number("42"), progressValue(t, "end", "", "", nil))
	if !pt.IsReady() {
		t.Error("should be ready after end with numeric token")
	}
}

// progressValue builds a json.RawMessage for $/progress value field.
func progressValue(t *testing.T, kind, title, message string, pct *int) json.RawMessage {
	t.Helper()
	v := map[string]any{"kind": kind}
	if title != "" {
		v["title"] = title
	}
	if message != "" {
		v["message"] = message
	}
	if pct != nil {
		v["percentage"] = *pct
	}
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshaling progress value: %v", err)
	}
	return data
}
```

**Step 2: Run tests to verify they fail**

Run: `nix develop --command go test -v -run 'TestProgressTracker' ./internal/subprocess/`
Expected: FAIL — `ProgressTracker` not defined.

**Step 3: Implement ProgressTracker**

Create `internal/subprocess/progress.go`:

```go
package subprocess

import (
	"encoding/json"
	"sync"
	"time"
)

type ProgressTracker struct {
	tokens       map[string]*ProgressToken
	mu           sync.RWMutex
	lastActivity time.Time
	readyCh      chan struct{}
}

type ProgressToken struct {
	Title   string
	Message string
	Pct     *int
	Created time.Time
}

func NewProgressTracker() *ProgressTracker {
	return &ProgressTracker{
		tokens:       make(map[string]*ProgressToken),
		lastActivity: time.Now(),
	}
}

func tokenKey(id any) string {
	switch v := id.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case float64:
		return json.Number(fmt.Sprintf("%v", v)).String()
	default:
		data, _ := json.Marshal(v)
		return string(data)
	}
}

func (pt *ProgressTracker) HandleCreate(tokenID any) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	key := tokenKey(tokenID)
	wasEmpty := len(pt.tokens) == 0

	pt.tokens[key] = &ProgressToken{
		Created: time.Now(),
	}
	pt.lastActivity = time.Now()

	if wasEmpty {
		pt.readyCh = make(chan struct{})
	}
}

func (pt *ProgressTracker) HandleProgress(tokenID any, value json.RawMessage) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	key := tokenKey(tokenID)

	var v struct {
		Kind       string `json:"kind"`
		Title      string `json:"title"`
		Message    string `json:"message"`
		Percentage *int   `json:"percentage"`
	}
	if err := json.Unmarshal(value, &v); err != nil {
		return
	}

	pt.lastActivity = time.Now()

	switch v.Kind {
	case "begin":
		tok, ok := pt.tokens[key]
		if !ok {
			// Token not created via window/workDoneProgress/create;
			// some LSPs skip the create and go straight to begin.
			tok = &ProgressToken{Created: time.Now()}
			pt.tokens[key] = tok
			if len(pt.tokens) == 1 {
				pt.readyCh = make(chan struct{})
			}
		}
		tok.Title = v.Title
		tok.Message = v.Message
		tok.Pct = v.Percentage

	case "report":
		tok, ok := pt.tokens[key]
		if !ok {
			return
		}
		if v.Message != "" {
			tok.Message = v.Message
		}
		tok.Pct = v.Percentage

	case "end":
		if _, ok := pt.tokens[key]; !ok {
			return
		}
		delete(pt.tokens, key)
		if len(pt.tokens) == 0 && pt.readyCh != nil {
			close(pt.readyCh)
			pt.readyCh = nil
		}
	}
}

func (pt *ProgressTracker) IsReady() bool {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return len(pt.tokens) == 0
}

func (pt *ProgressTracker) ReadyCh() <-chan struct{} {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	if pt.readyCh == nil {
		// Already ready — return a closed channel
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return pt.readyCh
}

func (pt *ProgressTracker) LastActivity() time.Time {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.lastActivity
}

func (pt *ProgressTracker) ActiveProgress() []ProgressToken {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	result := make([]ProgressToken, 0, len(pt.tokens))
	for _, tok := range pt.tokens {
		cp := *tok
		result = append(result, cp)
	}
	return result
}
```

Add `"fmt"` to imports for the `tokenKey` function.

**Step 4: Run tests to verify they pass**

Run: `nix develop --command go test -v -run 'TestProgressTracker' ./internal/subprocess/`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/subprocess/progress.go internal/subprocess/progress_test.go
git commit -m "feat: add ProgressTracker for LSP work-done-progress tokens

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 3: ProgressTracker — WaitForReady

**Files:**
- Modify: `internal/subprocess/progress.go`
- Modify: `internal/subprocess/progress_test.go`

**Step 1: Write failing tests for WaitForReady**

Add to `internal/subprocess/progress_test.go`:

```go
import (
	"context"
	// ... existing imports ...
	"errors"
)

func TestProgressTracker_WaitForReady_AlreadyReady(t *testing.T) {
	pt := NewProgressTracker()
	ctx := context.Background()
	err := pt.WaitForReady(ctx, 30*time.Second, 10*time.Minute, nil)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestProgressTracker_WaitForReady_UnblocksOnEnd(t *testing.T) {
	pt := NewProgressTracker()
	pt.HandleCreate("token-1")
	pt.HandleProgress("token-1", progressValue(t, "begin", "Loading", "", nil))

	ctx := context.Background()
	done := make(chan error, 1)
	go func() {
		done <- pt.WaitForReady(ctx, 5*time.Second, 10*time.Second, nil)
	}()

	time.Sleep(50 * time.Millisecond)
	pt.HandleProgress("token-1", progressValue(t, "end", "", "", nil))

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForReady did not return after token ended")
	}
}

func TestProgressTracker_WaitForReady_ActivityTimeout(t *testing.T) {
	pt := NewProgressTracker()
	pt.HandleCreate("token-1")
	pt.HandleProgress("token-1", progressValue(t, "begin", "Loading", "", nil))

	ctx := context.Background()
	done := make(chan error, 1)
	go func() {
		done <- pt.WaitForReady(ctx, 100*time.Millisecond, 10*time.Second, nil)
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected activity timeout error")
		}
		if !errors.Is(err, ErrActivityTimeout) {
			t.Errorf("expected ErrActivityTimeout, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForReady did not return on activity timeout")
	}
}

func TestProgressTracker_WaitForReady_HardTimeout(t *testing.T) {
	pt := NewProgressTracker()
	pt.HandleCreate("token-1")
	pt.HandleProgress("token-1", progressValue(t, "begin", "Loading", "", nil))

	ctx := context.Background()
	done := make(chan error, 1)

	// Keep sending reports to prevent activity timeout
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-time.After(20 * time.Millisecond):
				pt.HandleProgress("token-1", progressValue(t, "report", "", "working", nil))
			}
		}
	}()

	go func() {
		done <- pt.WaitForReady(ctx, 5*time.Second, 200*time.Millisecond, nil)
	}()

	select {
	case err := <-done:
		close(stop)
		if err == nil {
			t.Fatal("expected hard timeout error")
		}
		if !errors.Is(err, ErrHardTimeout) {
			t.Errorf("expected ErrHardTimeout, got %v", err)
		}
	case <-time.After(2 * time.Second):
		close(stop)
		t.Fatal("WaitForReady did not return on hard timeout")
	}
}

func TestProgressTracker_WaitForReady_ContextCancelled(t *testing.T) {
	pt := NewProgressTracker()
	pt.HandleCreate("token-1")
	pt.HandleProgress("token-1", progressValue(t, "begin", "Loading", "", nil))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- pt.WaitForReady(ctx, 5*time.Second, 10*time.Second, nil)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForReady did not return on context cancel")
	}
}

func TestProgressTracker_WaitForReady_InstanceStateCheck(t *testing.T) {
	pt := NewProgressTracker()
	pt.HandleCreate("token-1")
	pt.HandleProgress("token-1", progressValue(t, "begin", "Loading", "", nil))

	ctx := context.Background()
	failed := false
	stateCheck := func() bool { return failed }

	done := make(chan error, 1)
	go func() {
		done <- pt.WaitForReady(ctx, 5*time.Second, 10*time.Second, stateCheck)
	}()

	time.Sleep(50 * time.Millisecond)
	failed = true

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected error when instance failed")
		}
		if !errors.Is(err, ErrInstanceFailed) {
			t.Errorf("expected ErrInstanceFailed, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForReady did not return on instance failure")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `nix develop --command go test -v -run 'TestProgressTracker_WaitForReady' ./internal/subprocess/`
Expected: FAIL — `WaitForReady`, `ErrActivityTimeout`, `ErrHardTimeout`, `ErrInstanceFailed` not defined.

**Step 3: Implement WaitForReady**

Add to `internal/subprocess/progress.go`:

```go
import (
	"context"
	// ... existing imports ...
)

var (
	ErrActivityTimeout = errors.New("LSP progress stalled: no activity within timeout")
	ErrHardTimeout     = errors.New("LSP progress exceeded maximum wait time")
	ErrInstanceFailed  = errors.New("LSP instance failed during progress wait")
)

// WaitForReady blocks until all progress tokens complete, or a timeout fires.
// isFailedFn is called periodically to check if the LSP instance has failed;
// pass nil if no external state check is needed.
func (pt *ProgressTracker) WaitForReady(ctx context.Context, activityTimeout, hardTimeout time.Duration, isFailedFn func() bool) error {
	if pt.IsReady() {
		return nil
	}

	deadline := time.Now().Add(hardTimeout)
	pollInterval := 250 * time.Millisecond

	for {
		readyCh := pt.ReadyCh()

		select {
		case <-readyCh:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}

		if isFailedFn != nil && isFailedFn() {
			return ErrInstanceFailed
		}

		if time.Now().After(deadline) {
			return ErrHardTimeout
		}

		if time.Since(pt.LastActivity()) > activityTimeout {
			return ErrActivityTimeout
		}
	}
}
```

Add `"errors"` to imports.

**Step 4: Run tests to verify they pass**

Run: `nix develop --command go test -v -run 'TestProgressTracker_WaitForReady' ./internal/subprocess/`
Expected: PASS

**Step 5: Run all ProgressTracker tests together**

Run: `nix develop --command go test -v -run 'TestProgressTracker' ./internal/subprocess/`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/subprocess/progress.go internal/subprocess/progress_test.go
git commit -m "feat: add WaitForReady with activity and hard timeouts

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 4: Wire ProgressTracker into LSPInstance and Pool

**Files:**
- Modify: `internal/subprocess/pool.go`

**Step 1: Add Progress field to LSPInstance**

In `internal/subprocess/pool.go`, add to the `LSPInstance` struct (after `Error error`):

```go
Progress *ProgressTracker
```

**Step 2: Initialize tracker in GetOrStart**

In `GetOrStart`, right after `inst.State = LSPStateStarting` (line 149), add:

```go
inst.Progress = NewProgressTracker()
```

**Step 3: Advertise workDoneProgress in client capabilities**

This happens in the callers (`bridge.go` and `server/server.go`) that construct `InitializeParams`. We'll handle this in later tasks — no change to `pool.go` needed here.

**Step 4: Add readiness config fields to LSPInstance**

Add to `LSPInstance`:

```go
WaitForReady    bool
ReadyTimeout    time.Duration
ActivityTimeout time.Duration
```

Update `Register` to accept these from config. Add three parameters after `capOverrides`:

```go
func (p *Pool) Register(name, flake, binary string, args []string, env map[string]string, initOpts map[string]any, settings map[string]any, settingsKey string, capOverrides *CapabilityOverride, waitForReady bool, readyTimeout, activityTimeout time.Duration) {
```

And set them in the instance:

```go
p.instances[name] = &LSPInstance{
	// ... existing fields ...
	WaitForReady:    waitForReady,
	ReadyTimeout:    readyTimeout,
	ActivityTimeout: activityTimeout,
}
```

**Step 5: Update all callers of Register**

In `internal/server/server.go` (lines 51-61 and 134-143) and `internal/mcp/server.go` (lines 54-64), update `Register` calls to pass the new config fields:

```go
s.pool.Register(
	l.Name, l.Flake, l.Binary, l.Args, l.Env, l.InitOptions, l.Settings, l.SettingsWireKey(), capOverrides,
	l.ShouldWaitForReady(), l.ReadyTimeoutDuration(), l.ActivityTimeoutDuration(),
)
```

There are **four** call sites to update:
1. `internal/server/server.go:60` (initial registration in `New`)
2. `internal/server/server.go:143` (re-registration in `reloadPool`)
3. `internal/mcp/server.go:63` (registration in MCP `New`)

**Step 6: Verify it compiles**

Run: `nix develop --command go build ./...`
Expected: success

**Step 7: Run all tests**

Run: `nix develop --command go test ./...`
Expected: PASS

**Step 8: Commit**

```bash
git add internal/subprocess/pool.go internal/server/server.go internal/mcp/server.go
git commit -m "feat: wire ProgressTracker and readiness config into LSPInstance

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 5: Handler integration — intercept progress messages

**Files:**
- Modify: `internal/server/handler.go` — `serverNotificationHandler`
- Modify: `internal/mcp/server.go` — `lspNotificationHandler`
- Modify: `internal/lsp/protocol.go` — add progress param types

**Step 1: Add LSP protocol types for progress params**

In `internal/lsp/protocol.go`, add at the bottom:

```go
// WorkDoneProgressCreateParams is sent by the server to create a progress token.
type WorkDoneProgressCreateParams struct {
	Token any `json:"token"` // string | number
}

// ProgressParams wraps the $/progress notification.
type ProgressParams struct {
	Token any             `json:"token"` // string | number
	Value json.RawMessage `json:"value"`
}
```

**Step 2: Update serverNotificationHandler (LSP server path)**

In `internal/server/handler.go`, modify `serverNotificationHandler` to intercept progress messages **before** forwarding. The handler closure needs access to the pool to look up the instance's tracker:

```go
func serverNotificationHandler(s *Server, lspName string) jsonrpc.Handler {
	return func(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
		// Intercept window/workDoneProgress/create requests
		if msg.IsRequest() && msg.Method == lsp.MethodWindowWorkDoneProgressCreate {
			if inst, ok := s.pool.Get(lspName); ok && inst.Progress != nil {
				var params lsp.WorkDoneProgressCreateParams
				if err := json.Unmarshal(msg.Params, &params); err == nil {
					inst.Progress.HandleCreate(params.Token)
				}
			}
			return jsonrpc.NewResponse(*msg.ID, nil)
		}

		// Intercept $/progress notifications — update tracker, then forward
		if msg.IsNotification() && msg.Method == lsp.MethodProgress {
			if inst, ok := s.pool.Get(lspName); ok && inst.Progress != nil {
				var params lsp.ProgressParams
				if err := json.Unmarshal(msg.Params, &params); err == nil {
					inst.Progress.HandleProgress(params.Token, params.Value)
				}
			}
			// Fall through to forward to client
		}

		if msg.IsNotification() {
			if s.clientConn != nil {
				s.clientConn.Notify(msg.Method, msg.Params)
			}
		}

		if msg.IsRequest() {
			if msg.Method == lsp.MethodWorkspaceConfiguration {
				return handleWorkspaceConfiguration(s, lspName, msg)
			}

			if s.clientConn != nil {
				result, err := s.clientConn.Call(ctx, msg.Method, msg.Params)
				if err != nil {
					return nil, err
				}
				resp, _ := jsonrpc.NewResponse(*msg.ID, nil)
				resp.Result = result
				return resp, nil
			}
		}

		return nil, nil
	}
}
```

**Step 3: Update lspNotificationHandler (MCP server path)**

In `internal/mcp/server.go`, modify `lspNotificationHandler`. The MCP handler also needs the pool and lspName. Change the method signature to accept lspName:

First, update the handler factory in `New()` (line 50-52):

```go
s.pool = subprocess.NewPool(executor, func(lspName string) jsonrpc.Handler {
	return s.lspNotificationHandler(lspName)
})
```

Then update the method:

```go
func (s *Server) lspNotificationHandler(lspName string) jsonrpc.Handler {
	return func(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
		// Intercept window/workDoneProgress/create requests
		if msg.IsRequest() && msg.Method == lsp.MethodWindowWorkDoneProgressCreate {
			if inst, ok := s.pool.Get(lspName); ok && inst.Progress != nil {
				var params lsp.WorkDoneProgressCreateParams
				if err := json.Unmarshal(msg.Params, &params); err == nil {
					inst.Progress.HandleCreate(params.Token)
				}
			}
			return jsonrpc.NewResponse(*msg.ID, nil)
		}

		// Intercept $/progress notifications — update tracker, log to stderr
		if msg.IsNotification() && msg.Method == lsp.MethodProgress {
			if inst, ok := s.pool.Get(lspName); ok && inst.Progress != nil {
				var params lsp.ProgressParams
				if err := json.Unmarshal(msg.Params, &params); err == nil {
					inst.Progress.HandleProgress(params.Token, params.Value)

					// Log progress to stderr
					active := inst.Progress.ActiveProgress()
					for _, tok := range active {
						msg := tok.Title
						if tok.Message != "" {
							msg += ": " + tok.Message
						}
						if tok.Pct != nil {
							msg += fmt.Sprintf(" (%d%%)", *tok.Pct)
						}
						fmt.Fprintf(os.Stderr, "[lux] %s: %s\n", lspName, msg)
					}
				}
			}
			return nil, nil
		}

		if msg.Method == "textDocument/publishDiagnostics" && msg.Params != nil {
			var params lsp.PublishDiagnosticsParams
			if err := json.Unmarshal(msg.Params, &params); err != nil {
				return nil, nil
			}

			s.diagStore.Update(params)

			resourceURI := DiagnosticsResourceURI(params.URI)
			notification, err := jsonrpc.NewNotification("notifications/resources/updated", map[string]string{
				"uri": resourceURI,
			})
			if err == nil {
				s.transport.Write(notification)
			}
		}

		return nil, nil
	}
}
```

**Step 4: Advertise workDoneProgress in client capabilities**

In `internal/mcp/bridge.go`, update `defaultInitParams` (line 480-495) to add `Window`:

```go
Capabilities: lsp.ClientCapabilities{
	Workspace: &lsp.WorkspaceClientCapabilities{
		WorkspaceFolders: true,
	},
	TextDocument: &lsp.TextDocumentClientCapabilities{
		// ... existing fields ...
	},
	Window: &lsp.WindowClientCapabilities{
		WorkDoneProgress: true,
	},
},
```

In `internal/server/handler.go`, the init params come from the client editor — they may or may not already advertise `workDoneProgress`. Since the LSP server path just forwards everything, this is fine. But during `GetOrStart`, we should ensure the init params sent to the LSP include `workDoneProgress: true`. In `internal/subprocess/pool.go`, in `GetOrStart` around line 186 where `customParams` is created, add:

```go
if customParams.Capabilities.Window == nil {
	customParams.Capabilities.Window = &lsp.WindowClientCapabilities{}
}
customParams.Capabilities.Window.WorkDoneProgress = true
```

**Step 5: Verify it compiles**

Run: `nix develop --command go build ./...`
Expected: success

**Step 6: Run all tests**

Run: `nix develop --command go test ./...`
Expected: PASS

**Step 7: Commit**

```bash
git add internal/lsp/protocol.go internal/server/handler.go internal/mcp/server.go internal/mcp/bridge.go internal/subprocess/pool.go
git commit -m "feat: intercept LSP progress messages in notification handlers

Handle window/workDoneProgress/create and $/progress in both LSP and
MCP server paths. Forward progress to editor clients; log to stderr
for MCP. Advertise workDoneProgress in client capabilities.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 6: MCP Bridge — wait for ready before LSP calls

**Files:**
- Modify: `internal/mcp/bridge.go`
- Modify: `internal/mcp/server.go`

**Step 1: Add progress reporter callback to Bridge**

In `internal/mcp/bridge.go`, add a callback field and update constructor:

```go
type Bridge struct {
	pool             *subprocess.Pool
	router           *server.Router
	fmtRouter        *formatter.Router
	executor         subprocess.Executor
	docMgr           *DocumentManager
	progressReporter func(lspName, message string)
}

func NewBridge(pool *subprocess.Pool, router *server.Router, fmtRouter *formatter.Router, executor subprocess.Executor, progressReporter func(lspName, message string)) *Bridge {
	return &Bridge{
		pool:             pool,
		router:           router,
		fmtRouter:        fmtRouter,
		executor:         executor,
		progressReporter: progressReporter,
	}
}
```

**Step 2: Add waitForLSPReady method to Bridge**

Add to `internal/mcp/bridge.go`:

```go
func (b *Bridge) waitForLSPReady(ctx context.Context, inst *subprocess.LSPInstance) error {
	if !inst.WaitForReady || inst.Progress == nil || inst.Progress.IsReady() {
		return nil
	}

	fmt.Fprintf(os.Stderr, "[lux] %s: waiting for LSP to finish indexing...\n", inst.Name)

	// Report progress periodically while waiting
	done := make(chan error, 1)
	go func() {
		done <- inst.Progress.WaitForReady(ctx, inst.ActivityTimeout, inst.ReadyTimeout, func() bool {
			inst.mu.RLock()
			defer inst.mu.RUnlock()
			return inst.State == subprocess.LSPStateFailed
		})
	}()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case err := <-done:
			if err == nil {
				fmt.Fprintf(os.Stderr, "[lux] %s: LSP ready\n", inst.Name)
			}
			return err
		case <-ticker.C:
			active := inst.Progress.ActiveProgress()
			for _, tok := range active {
				msg := tok.Title
				if tok.Message != "" {
					msg += ": " + tok.Message
				}
				if tok.Pct != nil {
					msg += fmt.Sprintf(" (%d%%)", *tok.Pct)
				}
				fmt.Fprintf(os.Stderr, "[lux] %s: %s\n", inst.Name, msg)
				if b.progressReporter != nil {
					b.progressReporter(inst.Name, msg)
				}
			}
		}
	}
}
```

Note: `inst.mu` access needs `LSPStateFailed` to be accessible. `inst.State` is already a public field and `LSPStateFailed` is already exported. The `mu` field is unexported, so the `isFailedFn` closure needs to be created in a context where `inst` is accessible. Since `LSPInstance.mu` is unexported, add a public method instead.

Add to `internal/subprocess/pool.go`:

```go
func (inst *LSPInstance) IsFailed() bool {
	inst.mu.RLock()
	defer inst.mu.RUnlock()
	return inst.State == LSPStateFailed
}
```

Then in `waitForLSPReady`, use `inst.IsFailed` directly:

```go
done <- inst.Progress.WaitForReady(ctx, inst.ActivityTimeout, inst.ReadyTimeout, inst.IsFailed)
```

**Step 3: Insert wait in withDocument**

In `internal/mcp/bridge.go`, in `withDocument` (after getting the instance and before the document open/retry), add:

```go
inst, err := b.pool.GetOrStart(ctx, lspName, initParams)
if err != nil {
	return nil, fmt.Errorf("starting LSP %s: %w", lspName, err)
}

// Wait for LSP to finish indexing before making calls
if err := b.waitForLSPReady(ctx, inst); err != nil {
	return nil, fmt.Errorf("waiting for LSP %s readiness: %w", lspName, err)
}

projectRoot := b.projectRootForPath(uri.Path())
// ... rest unchanged
```

**Step 4: Wire progressReporter in MCP server**

In `internal/mcp/server.go`, update the `NewBridge` call (line 78) to pass a progress reporter that sends MCP logging notifications:

```go
s.bridge = NewBridge(s.pool, s.router, fmtRouter, executor, func(lspName, message string) {
	notification, err := jsonrpc.NewNotification("notifications/message", map[string]any{
		"level": "info",
		"data":  fmt.Sprintf("%s: %s", lspName, message),
	})
	if err == nil {
		s.transport.Write(notification)
	}
})
```

**Step 5: Verify it compiles**

Run: `nix develop --command go build ./...`
Expected: success

**Step 6: Run all tests**

Run: `nix develop --command go test ./...`
Expected: PASS

**Step 7: Commit**

```bash
git add internal/mcp/bridge.go internal/mcp/server.go internal/subprocess/pool.go
git commit -m "feat: MCP bridge waits for LSP readiness before calls

Block MCP tool calls until LSP progress tokens drain. Report progress
to stderr and via MCP notifications/message. Configurable per-LSP via
wait_for_ready, ready_timeout, activity_timeout.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 7: Full integration test

Run the full test suite and verify the nix build still works.

**Step 1: Run all Go tests**

Run: `nix develop --command go test -v ./...`
Expected: PASS

**Step 2: Run go vet**

Run: `nix develop --command go vet ./...`
Expected: no issues

**Step 3: Run nix build**

Run: `nix build`
Expected: success (may need `just deps` first if imports changed)

**Step 4: Verify gomod2nix is up to date**

No new dependencies were added (all types are from existing packages), so `gomod2nix.toml` should not need updating. Confirm:

Run: `nix develop --command go mod tidy && git diff go.mod go.sum`
Expected: no changes

**Step 5: Commit (if any formatting/cleanup needed)**

Only commit if there are changes from formatting.
