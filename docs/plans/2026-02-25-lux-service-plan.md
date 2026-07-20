# Lux Service Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Transition lux from a monolithic LSP multiplexer to a server-client
model with a socket-activated service daemon and thin LSP/MCP proxy clients.

**Architecture:** A single global service process owns all LSP subprocess pools
(keyed by workspace), accepts JSON-RPC over a Unix socket from thin clients
(`lux lsp`, `lux mcp stdio`), and exposes MCP directly over HTTP/SSE. The
service is socket-activated via launchd/systemd — zero overhead when unused.

**Tech Stack:** Go, JSON-RPC (existing `go-mcp/jsonrpc`), Unix sockets
(existing `net` package), launchd plists / systemd units.

**Design Doc:** `docs/plans/2026-02-25-lux-service-design.md`

---

## Task 1: Define the Service Protocol Types

**Files:**
- Create: `packages/lux/internal/service/protocol.go`
- Test: `packages/lux/internal/service/protocol_test.go`

This task defines the JSON-RPC method names and parameter/result types for the
service protocol. No behavior — just types.

**Step 1: Write the failing test**

```go
package service

import (
	"encoding/json"
	"testing"
)

func TestRegisterParams_Marshal(t *testing.T) {
	p := RegisterParams{
		WorkspaceRoot: "/home/user/project",
		ClientType:    ClientTypeLSP,
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var decoded RegisterParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.WorkspaceRoot != p.WorkspaceRoot {
		t.Errorf("got %q, want %q", decoded.WorkspaceRoot, p.WorkspaceRoot)
	}
	if decoded.ClientType != ClientTypeLSP {
		t.Errorf("got %q, want %q", decoded.ClientType, ClientTypeLSP)
	}
}

func TestRegisterResult_Marshal(t *testing.T) {
	r := RegisterResult{SessionID: "abc123"}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var decoded RegisterResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SessionID != "abc123" {
		t.Errorf("got %q, want %q", decoded.SessionID, "abc123")
	}
}

func TestLSPRequestParams_Marshal(t *testing.T) {
	p := LSPRequestParams{
		SessionID: "abc123",
		LSPMethod: "textDocument/completion",
		LSPParams: json.RawMessage(`{"textDocument":{"uri":"file:///main.go"}}`),
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var decoded LSPRequestParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SessionID != "abc123" {
		t.Errorf("got %q, want %q", decoded.SessionID, "abc123")
	}
	if decoded.LSPMethod != "textDocument/completion" {
		t.Errorf("got %q, want %q", decoded.LSPMethod, "textDocument/completion")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test -v -run TestRegister ./packages/lux/internal/service/`
Expected: FAIL — package does not exist

**Step 3: Write minimal implementation**

```go
package service

import "encoding/json"

// JSON-RPC method names for the service protocol.
const (
	MethodSessionRegister   = "lux/session.register"
	MethodSessionDeregister = "lux/session.deregister"
	MethodLSPRequest        = "lux/lsp.request"
	MethodLSPNotification   = "lux/lsp.notification"
	MethodPoolStatus        = "lux/pool.status"
	MethodPoolStart         = "lux/pool.start"
	MethodPoolStop          = "lux/pool.stop"
	MethodWarmup            = "lux/warmup"
)

// ClientType identifies the kind of client connecting.
type ClientType string

const (
	ClientTypeLSP     ClientType = "lsp"
	ClientTypeMCP     ClientType = "mcp"
	ClientTypeControl ClientType = "control"
)

// RegisterParams is sent by a client to register a session.
type RegisterParams struct {
	WorkspaceRoot string     `json:"workspace_root"`
	ClientType    ClientType `json:"client_type"`
}

// RegisterResult is returned after successful session registration.
type RegisterResult struct {
	SessionID string `json:"session_id"`
}

// DeregisterParams is sent by a client to deregister its session.
type DeregisterParams struct {
	SessionID string `json:"session_id"`
}

// LSPRequestParams wraps an LSP request with session context.
type LSPRequestParams struct {
	SessionID string          `json:"session_id"`
	LSPMethod string          `json:"lsp_method"`
	LSPParams json.RawMessage `json:"lsp_params"`
}

// LSPNotificationParams wraps an LSP notification with session context.
type LSPNotificationParams struct {
	SessionID string          `json:"session_id"`
	LSPMethod string          `json:"lsp_method"`
	LSPParams json.RawMessage `json:"lsp_params"`
}

// PoolStartParams identifies an LSP to start.
type PoolStartParams struct {
	Name string `json:"name"`
}

// PoolStopParams identifies an LSP to stop.
type PoolStopParams struct {
	Name string `json:"name"`
}

// WarmupParams identifies a directory to pre-start LSPs for.
type WarmupParams struct {
	Dir string `json:"dir"`
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test -v -run TestRegister ./packages/lux/internal/service/`
Expected: PASS

**Step 5: Commit**

```
git add packages/lux/internal/service/protocol.go packages/lux/internal/service/protocol_test.go
git commit -m "feat(lux): add service protocol types"
```

---

## Task 2: Implement Session Registry

**Files:**
- Create: `packages/lux/internal/service/session.go`
- Test: `packages/lux/internal/service/session_test.go`

The session registry tracks connected clients, their workspace roots, open
documents, and pending request IDs. It handles reference-counted document
open/close.

**Step 1: Write the failing test**

```go
package service

import (
	"testing"
)

func TestSessionRegistry_RegisterDeregister(t *testing.T) {
	r := NewSessionRegistry()

	id := r.Register("/proj/a", ClientTypeLSP)
	if id == "" {
		t.Fatal("expected non-empty session ID")
	}

	s, ok := r.Get(id)
	if !ok {
		t.Fatal("expected session to exist")
	}
	if s.WorkspaceRoot != "/proj/a" {
		t.Errorf("got %q, want %q", s.WorkspaceRoot, "/proj/a")
	}
	if s.ClientType != ClientTypeLSP {
		t.Errorf("got %q, want %q", s.ClientType, ClientTypeLSP)
	}

	r.Deregister(id)
	_, ok = r.Get(id)
	if ok {
		t.Fatal("expected session to be removed")
	}
}

func TestSessionRegistry_ActiveSessions(t *testing.T) {
	r := NewSessionRegistry()
	r.Register("/proj/a", ClientTypeLSP)
	r.Register("/proj/a", ClientTypeMCP)
	r.Register("/proj/b", ClientTypeLSP)

	if n := r.ActiveCount(); n != 3 {
		t.Errorf("got %d active sessions, want 3", n)
	}

	sessions := r.SessionsForWorkspace("/proj/a")
	if len(sessions) != 2 {
		t.Errorf("got %d sessions for /proj/a, want 2", len(sessions))
	}
}

func TestSessionRegistry_DocumentRefCounting(t *testing.T) {
	r := NewSessionRegistry()
	id1 := r.Register("/proj/a", ClientTypeLSP)
	id2 := r.Register("/proj/a", ClientTypeMCP)

	// First open should return true (send didOpen to LSP)
	shouldOpen := r.OpenDocument(id1, "file:///proj/a/main.go")
	if !shouldOpen {
		t.Error("first open should return true")
	}

	// Second open should return false (already open)
	shouldOpen = r.OpenDocument(id2, "file:///proj/a/main.go")
	if shouldOpen {
		t.Error("second open should return false")
	}

	// First close should return false (still has refs)
	shouldClose := r.CloseDocument(id1, "file:///proj/a/main.go")
	if shouldClose {
		t.Error("first close should return false (still has refs)")
	}

	// Second close should return true (last ref, send didClose)
	shouldClose = r.CloseDocument(id2, "file:///proj/a/main.go")
	if !shouldClose {
		t.Error("second close should return true (last ref)")
	}
}

func TestSessionRegistry_DeregisterCleansUpDocs(t *testing.T) {
	r := NewSessionRegistry()
	id1 := r.Register("/proj/a", ClientTypeLSP)
	id2 := r.Register("/proj/a", ClientTypeMCP)

	r.OpenDocument(id1, "file:///proj/a/main.go")
	r.OpenDocument(id2, "file:///proj/a/main.go")

	// Deregistering id1 should not trigger close (id2 still has it open)
	closeDocs := r.Deregister(id1)
	if len(closeDocs) != 0 {
		t.Errorf("expected no docs to close, got %v", closeDocs)
	}

	// Deregistering id2 should trigger close (last ref)
	closeDocs = r.Deregister(id2)
	if len(closeDocs) != 1 || closeDocs[0] != "file:///proj/a/main.go" {
		t.Errorf("expected [file:///proj/a/main.go], got %v", closeDocs)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test -v -run TestSessionRegistry ./packages/lux/internal/service/`
Expected: FAIL — `NewSessionRegistry` undefined

**Step 3: Write minimal implementation**

```go
package service

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
)

// Session represents a connected client's state.
type Session struct {
	ID            string
	WorkspaceRoot string
	ClientType    ClientType
	OpenDocs      map[string]bool // URIs this session has open
}

// SessionRegistry manages connected client sessions.
type SessionRegistry struct {
	sessions map[string]*Session
	// docRefs tracks per-workspace, per-URI reference counts.
	// Key: workspace_root, Value: map[uri]refcount
	docRefs map[string]map[string]int
	mu      sync.RWMutex
}

func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{
		sessions: make(map[string]*Session),
		docRefs:  make(map[string]map[string]int),
	}
}

func (r *SessionRegistry) Register(workspaceRoot string, clientType ClientType) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := generateSessionID()
	r.sessions[id] = &Session{
		ID:            id,
		WorkspaceRoot: workspaceRoot,
		ClientType:    clientType,
		OpenDocs:      make(map[string]bool),
	}

	if _, ok := r.docRefs[workspaceRoot]; !ok {
		r.docRefs[workspaceRoot] = make(map[string]int)
	}

	return id
}

// Deregister removes a session and returns URIs that should be closed
// (those whose ref count dropped to zero).
func (r *SessionRegistry) Deregister(id string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.sessions[id]
	if !ok {
		return nil
	}

	var closeDocs []string
	refs := r.docRefs[s.WorkspaceRoot]
	for uri := range s.OpenDocs {
		refs[uri]--
		if refs[uri] <= 0 {
			delete(refs, uri)
			closeDocs = append(closeDocs, uri)
		}
	}

	delete(r.sessions, id)
	return closeDocs
}

func (r *SessionRegistry) Get(id string) (*Session, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[id]
	return s, ok
}

func (r *SessionRegistry) ActiveCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sessions)
}

func (r *SessionRegistry) SessionsForWorkspace(workspaceRoot string) []*Session {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*Session
	for _, s := range r.sessions {
		if s.WorkspaceRoot == workspaceRoot {
			result = append(result, s)
		}
	}
	return result
}

// OpenDocument marks a document as open for this session. Returns true if
// this is the first session to open it (caller should send didOpen to LSP).
func (r *SessionRegistry) OpenDocument(sessionID, uri string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.sessions[sessionID]
	if !ok {
		return false
	}

	s.OpenDocs[uri] = true
	refs := r.docRefs[s.WorkspaceRoot]
	refs[uri]++
	return refs[uri] == 1
}

// CloseDocument marks a document as closed for this session. Returns true if
// this was the last session with it open (caller should send didClose to LSP).
func (r *SessionRegistry) CloseDocument(sessionID, uri string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.sessions[sessionID]
	if !ok {
		return false
	}

	delete(s.OpenDocs, uri)
	refs := r.docRefs[s.WorkspaceRoot]
	refs[uri]--
	if refs[uri] <= 0 {
		delete(refs, uri)
		return true
	}
	return false
}

func generateSessionID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test -v -run TestSessionRegistry ./packages/lux/internal/service/`
Expected: PASS

**Step 5: Commit**

```
git add packages/lux/internal/service/session.go packages/lux/internal/service/session_test.go
git commit -m "feat(lux): add session registry with ref-counted documents"
```

---

## Task 3: Implement Workspace Registry

**Files:**
- Create: `packages/lux/internal/service/workspace.go`
- Test: `packages/lux/internal/service/workspace_test.go`

The workspace registry manages per-workspace LSP pools and config. It creates
pools on-demand when the first session for a workspace registers.

**Step 1: Write the failing test**

```go
package service

import (
	"testing"

	"github.com/amarbel-llc/lux/internal/config"
)

func TestWorkspaceRegistry_GetOrCreate(t *testing.T) {
	cfg, _ := config.Load()
	r := NewWorkspaceRegistry(cfg)

	ws1 := r.GetOrCreate("/proj/a")
	if ws1 == nil {
		t.Fatal("expected workspace to be created")
	}
	if ws1.Root != "/proj/a" {
		t.Errorf("got %q, want %q", ws1.Root, "/proj/a")
	}
	if ws1.Pool == nil {
		t.Fatal("expected pool to be created")
	}

	// Second call should return the same workspace
	ws2 := r.GetOrCreate("/proj/a")
	if ws1 != ws2 {
		t.Error("expected same workspace instance")
	}
}

func TestWorkspaceRegistry_Remove(t *testing.T) {
	cfg, _ := config.Load()
	r := NewWorkspaceRegistry(cfg)

	r.GetOrCreate("/proj/a")
	r.Remove("/proj/a")

	if r.Count() != 0 {
		t.Errorf("expected 0 workspaces, got %d", r.Count())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test -v -run TestWorkspaceRegistry ./packages/lux/internal/service/`
Expected: FAIL — `NewWorkspaceRegistry` undefined

**Step 3: Write minimal implementation**

```go
package service

import (
	"sync"

	"code.linenisgreat.com/purse-first/libs/go-mcp/jsonrpc"
	"github.com/amarbel-llc/lux/internal/config"
	"github.com/amarbel-llc/lux/internal/config/filetype"
	"github.com/amarbel-llc/lux/internal/server"
	"github.com/amarbel-llc/lux/internal/subprocess"
)

// Workspace holds the LSP pool and config for a single workspace root.
type Workspace struct {
	Root      string
	Pool      *subprocess.Pool
	Router    *server.Router
	Config    *config.Config
	Filetypes []*filetype.Config
	Executor  subprocess.Executor
}

// WorkspaceRegistry manages per-workspace LSP pools.
type WorkspaceRegistry struct {
	workspaces map[string]*Workspace
	baseCfg    *config.Config
	mu         sync.RWMutex
}

func NewWorkspaceRegistry(baseCfg *config.Config) *WorkspaceRegistry {
	return &WorkspaceRegistry{
		workspaces: make(map[string]*Workspace),
		baseCfg:    baseCfg,
	}
}

func (r *WorkspaceRegistry) GetOrCreate(root string) *Workspace {
	r.mu.Lock()
	defer r.mu.Unlock()

	if ws, ok := r.workspaces[root]; ok {
		return ws
	}

	cfg := r.baseCfg
	projectCfg, err := config.LoadWithProject(root)
	if err == nil {
		cfg = projectCfg
	}

	ftConfigs, _ := filetype.LoadMerged()
	if ftConfigs == nil {
		ftConfigs = []*filetype.Config{}
	}

	router, _ := server.NewRouter(ftConfigs)
	executor := subprocess.NewNixExecutor()

	pool := subprocess.NewPool(executor, func(lspName string) jsonrpc.Handler {
		// Notification handler will be wired in by the service handler
		return nil
	})

	for _, l := range cfg.LSPs {
		var capOverrides *subprocess.CapabilityOverride
		if l.Capabilities != nil {
			capOverrides = &subprocess.CapabilityOverride{
				Disable: l.Capabilities.Disable,
				Enable:  l.Capabilities.Enable,
			}
		}
		pool.Register(l.Name, l.Flake, l.Binary, l.Args, l.Env,
			l.InitOptions, l.Settings, l.SettingsWireKey(),
			capOverrides, l.ShouldWaitForReady(),
			l.ReadyTimeoutDuration(), l.ActivityTimeoutDuration())
	}

	ws := &Workspace{
		Root:      root,
		Pool:      pool,
		Router:    router,
		Config:    cfg,
		Filetypes: ftConfigs,
		Executor:  executor,
	}

	r.workspaces[root] = ws
	return ws
}

func (r *WorkspaceRegistry) Get(root string) (*Workspace, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ws, ok := r.workspaces[root]
	return ws, ok
}

func (r *WorkspaceRegistry) Remove(root string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if ws, ok := r.workspaces[root]; ok {
		ws.Pool.StopAll()
		delete(r.workspaces, root)
	}
}

func (r *WorkspaceRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.workspaces)
}

func (r *WorkspaceRegistry) StopAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, ws := range r.workspaces {
		ws.Pool.StopAll()
	}
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test -v -run TestWorkspaceRegistry ./packages/lux/internal/service/`
Expected: PASS

**Step 5: Commit**

```
git add packages/lux/internal/service/workspace.go packages/lux/internal/service/workspace_test.go
git commit -m "feat(lux): add workspace registry with per-workspace LSP pools"
```

---

## Task 4: Implement Service Handler

**Files:**
- Create: `packages/lux/internal/service/handler.go`
- Test: `packages/lux/internal/service/handler_test.go`

The service handler dispatches incoming JSON-RPC messages on the service
protocol. It routes `lux/session.*` to the session registry, `lux/lsp.*` to
the appropriate workspace pool, and `lux/pool.*` to control operations.

**Step 1: Write the failing test**

```go
package service

import (
	"context"
	"encoding/json"
	"testing"

	"code.linenisgreat.com/purse-first/libs/go-mcp/jsonrpc"
)

func TestHandler_SessionRegister(t *testing.T) {
	h := newTestHandler(t)

	params := RegisterParams{
		WorkspaceRoot: "/proj/a",
		ClientType:    ClientTypeLSP,
	}
	paramsJSON, _ := json.Marshal(params)

	id := json.RawMessage(`1`)
	msg := &jsonrpc.Message{
		ID:     &id,
		Method: MethodSessionRegister,
		Params: paramsJSON,
	}

	resp, err := h.Handle(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}

	var result RegisterResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.SessionID == "" {
		t.Error("expected non-empty session ID")
	}
}

func TestHandler_SessionDeregister(t *testing.T) {
	h := newTestHandler(t)

	// Register first
	sessionID := registerTestSession(t, h, "/proj/a", ClientTypeLSP)

	// Deregister
	params := DeregisterParams{SessionID: sessionID}
	paramsJSON, _ := json.Marshal(params)
	id := json.RawMessage(`2`)
	msg := &jsonrpc.Message{
		ID:     &id,
		Method: MethodSessionDeregister,
		Params: paramsJSON,
	}

	resp, err := h.Handle(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}

	// Verify session is gone
	if h.sessions.ActiveCount() != 0 {
		t.Error("expected 0 active sessions")
	}
}

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	return NewHandler(NewSessionRegistry(), NewWorkspaceRegistry(nil))
}

func registerTestSession(t *testing.T, h *Handler, root string, ct ClientType) string {
	t.Helper()
	params := RegisterParams{WorkspaceRoot: root, ClientType: ct}
	paramsJSON, _ := json.Marshal(params)
	id := json.RawMessage(`99`)
	msg := &jsonrpc.Message{
		ID:     &id,
		Method: MethodSessionRegister,
		Params: paramsJSON,
	}
	resp, err := h.Handle(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	var result RegisterResult
	json.Unmarshal(resp.Result, &result)
	return result.SessionID
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test -v -run TestHandler ./packages/lux/internal/service/`
Expected: FAIL — `Handler` undefined

**Step 3: Write minimal implementation**

```go
package service

import (
	"context"
	"encoding/json"
	"fmt"

	"code.linenisgreat.com/purse-first/libs/go-mcp/jsonrpc"
)

// Handler dispatches incoming service protocol messages.
type Handler struct {
	sessions   *SessionRegistry
	workspaces *WorkspaceRegistry
}

func NewHandler(sessions *SessionRegistry, workspaces *WorkspaceRegistry) *Handler {
	return &Handler{
		sessions:   sessions,
		workspaces: workspaces,
	}
}

func (h *Handler) Handle(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	switch msg.Method {
	case MethodSessionRegister:
		return h.handleRegister(ctx, msg)
	case MethodSessionDeregister:
		return h.handleDeregister(ctx, msg)
	case MethodLSPRequest:
		return h.handleLSPRequest(ctx, msg)
	case MethodLSPNotification:
		return h.handleLSPNotification(ctx, msg)
	case MethodPoolStatus:
		return h.handlePoolStatus(ctx, msg)
	case MethodPoolStart:
		return h.handlePoolStart(ctx, msg)
	case MethodPoolStop:
		return h.handlePoolStop(ctx, msg)
	case MethodWarmup:
		return h.handleWarmup(ctx, msg)
	default:
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.MethodNotFound,
			fmt.Sprintf("unknown method: %s", msg.Method), nil)
	}
}

func (h *Handler) handleRegister(_ context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	var params RegisterParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InvalidParams, err.Error(), nil)
	}

	sessionID := h.sessions.Register(params.WorkspaceRoot, params.ClientType)

	// Ensure workspace pool exists
	h.workspaces.GetOrCreate(params.WorkspaceRoot)

	return jsonrpc.NewResponse(*msg.ID, RegisterResult{SessionID: sessionID})
}

func (h *Handler) handleDeregister(_ context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	var params DeregisterParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InvalidParams, err.Error(), nil)
	}

	closeDocs := h.sessions.Deregister(params.SessionID)
	_ = closeDocs // TODO: send didClose for these URIs to the workspace pool

	return jsonrpc.NewResponse(*msg.ID, map[string]bool{"ok": true})
}

func (h *Handler) handleLSPRequest(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	var params LSPRequestParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InvalidParams, err.Error(), nil)
	}

	session, ok := h.sessions.Get(params.SessionID)
	if !ok {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InvalidParams,
			"unknown session", nil)
	}

	ws, ok := h.workspaces.Get(session.WorkspaceRoot)
	if !ok {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError,
			"no workspace for session", nil)
	}

	lspName := ws.Router.Route(params.LSPMethod, params.LSPParams)
	if lspName == "" {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.MethodNotFound,
			"no LSP configured for this file type", nil)
	}

	inst, err := ws.Pool.GetOrStart(ctx, lspName, nil)
	if err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError,
			fmt.Sprintf("starting LSP %s: %v", lspName, err), nil)
	}

	result, err := inst.Call(ctx, params.LSPMethod, params.LSPParams)
	if err != nil {
		if rpcErr, ok := err.(*jsonrpc.Error); ok {
			return jsonrpc.NewErrorResponse(*msg.ID, rpcErr.Code, rpcErr.Message, rpcErr.Data)
		}
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, err.Error(), nil)
	}

	resp, _ := jsonrpc.NewResponse(*msg.ID, nil)
	resp.Result = result
	return resp, nil
}

func (h *Handler) handleLSPNotification(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	var params LSPNotificationParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return nil, nil
	}

	session, ok := h.sessions.Get(params.SessionID)
	if !ok {
		return nil, nil
	}

	ws, ok := h.workspaces.Get(session.WorkspaceRoot)
	if !ok {
		return nil, nil
	}

	lspName := ws.Router.Route(params.LSPMethod, params.LSPParams)
	if lspName == "" {
		return nil, nil
	}

	inst, err := ws.Pool.GetOrStart(ctx, lspName, nil)
	if err != nil {
		return nil, nil
	}

	inst.Notify(params.LSPMethod, params.LSPParams)
	return nil, nil
}

func (h *Handler) handlePoolStatus(_ context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	// Aggregate status from all workspaces
	type wsStatus struct {
		Workspace string `json:"workspace"`
		LSPs      any    `json:"lsps"`
	}

	var statuses []wsStatus
	h.workspaces.mu.RLock()
	for root, ws := range h.workspaces.workspaces {
		statuses = append(statuses, wsStatus{
			Workspace: root,
			LSPs:      ws.Pool.Status(),
		})
	}
	h.workspaces.mu.RUnlock()

	return jsonrpc.NewResponse(*msg.ID, map[string]any{
		"workspaces": statuses,
		"sessions":   h.sessions.ActiveCount(),
	})
}

func (h *Handler) handlePoolStart(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	var params PoolStartParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InvalidParams, err.Error(), nil)
	}

	// Start in all workspaces that have this LSP registered
	h.workspaces.mu.RLock()
	defer h.workspaces.mu.RUnlock()

	for _, ws := range h.workspaces.workspaces {
		ws.Pool.GetOrStart(ctx, params.Name, nil)
	}

	return jsonrpc.NewResponse(*msg.ID, map[string]bool{"ok": true})
}

func (h *Handler) handlePoolStop(_ context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	var params PoolStopParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InvalidParams, err.Error(), nil)
	}

	h.workspaces.mu.RLock()
	defer h.workspaces.mu.RUnlock()

	for _, ws := range h.workspaces.workspaces {
		ws.Pool.Stop(params.Name)
	}

	return jsonrpc.NewResponse(*msg.ID, map[string]bool{"ok": true})
}

func (h *Handler) handleWarmup(_ context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	var params WarmupParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InvalidParams, err.Error(), nil)
	}

	// Warmup is fire-and-forget
	ws := h.workspaces.GetOrCreate(params.Dir)
	_ = ws // TODO: trigger warmup in background

	return jsonrpc.NewResponse(*msg.ID, map[string]bool{"ok": true})
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test -v -run TestHandler ./packages/lux/internal/service/`
Expected: PASS

**Step 5: Commit**

```
git add packages/lux/internal/service/handler.go packages/lux/internal/service/handler_test.go
git commit -m "feat(lux): add service handler dispatching session and LSP requests"
```

---

## Task 5: Implement Service Daemon

**Files:**
- Create: `packages/lux/internal/service/daemon.go`
- Test: `packages/lux/internal/service/daemon_test.go`

The daemon is the top-level process that listens on a Unix socket, accepts
client connections, and runs a JSON-RPC connection per client. It also manages
idle timeout (exits after 30 min with no active sessions).

**Step 1: Write the failing test**

```go
package service

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"code.linenisgreat.com/purse-first/libs/go-mcp/jsonrpc"
)

func TestDaemon_AcceptAndRegister(t *testing.T) {
	socketPath := t.TempDir() + "/lux.sock"
	d := NewDaemon(socketPath, nil, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Give the daemon time to start listening
	time.Sleep(50 * time.Millisecond)

	// Connect as a client
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Send a register request
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  MethodSessionRegister,
		"params": RegisterParams{
			WorkspaceRoot: "/proj/a",
			ClientType:    ClientTypeLSP,
		},
	}
	data, _ := json.Marshal(req)
	data = append(data, '\n')
	conn.Write(data)

	// Read response
	buf := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatal(err)
	}

	var resp jsonrpc.Message
	if err := json.Unmarshal(buf[:n], &resp); err != nil {
		t.Fatalf("failed to parse response %q: %v", buf[:n], err)
	}

	var result RegisterResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.SessionID == "" {
		t.Error("expected non-empty session ID")
	}

	cancel()
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test -v -run TestDaemon ./packages/lux/internal/service/`
Expected: FAIL — `NewDaemon` undefined

**Step 3: Write minimal implementation**

```go
package service

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"code.linenisgreat.com/purse-first/libs/go-mcp/jsonrpc"
	"github.com/amarbel-llc/lux/internal/config"
)

// Daemon is the top-level service process.
type Daemon struct {
	socketPath  string
	handler     *Handler
	sessions    *SessionRegistry
	workspaces  *WorkspaceRegistry
	listener    net.Listener
	idleTimeout time.Duration
	conns       map[net.Conn]string // conn → session ID
	mu          sync.Mutex
}

func NewDaemon(socketPath string, cfg *config.Config, idleTimeout time.Duration) *Daemon {
	sessions := NewSessionRegistry()
	workspaces := NewWorkspaceRegistry(cfg)
	handler := NewHandler(sessions, workspaces)

	return &Daemon{
		socketPath:  socketPath,
		handler:     handler,
		sessions:    sessions,
		workspaces:  workspaces,
		idleTimeout: idleTimeout,
		conns:       make(map[net.Conn]string),
	}
}

func (d *Daemon) Run(ctx context.Context) error {
	os.Remove(d.socketPath)

	listener, err := net.Listen("unix", d.socketPath)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", d.socketPath, err)
	}
	d.listener = listener

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	if d.idleTimeout > 0 {
		go d.idleWatcher(ctx)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				d.shutdown()
				return ctx.Err()
			default:
				continue
			}
		}

		go d.handleConn(ctx, conn)
	}
}

func (d *Daemon) handleConn(ctx context.Context, conn net.Conn) {
	defer func() {
		d.mu.Lock()
		sessionID := d.conns[conn]
		delete(d.conns, conn)
		d.mu.Unlock()

		if sessionID != "" {
			d.sessions.Deregister(sessionID)
		}
		conn.Close()
	}()

	rpcConn := jsonrpc.NewConn(conn, conn, func(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
		resp, err := d.handler.Handle(ctx, msg)

		// Track session ID for cleanup on disconnect
		if msg.Method == MethodSessionRegister && resp != nil && resp.Result != nil {
			var result RegisterResult
			if jsonErr := json.Unmarshal(resp.Result, &result); jsonErr == nil {
				d.mu.Lock()
				d.conns[conn] = result.SessionID
				d.mu.Unlock()
			}
		}

		return resp, err
	})

	rpcConn.Run(ctx)
}

func (d *Daemon) idleWatcher(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	idleSince := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if d.sessions.ActiveCount() > 0 {
				idleSince = time.Now()
				continue
			}

			if time.Since(idleSince) >= d.idleTimeout {
				fmt.Fprintf(os.Stderr, "lux service: idle timeout reached, shutting down\n")
				d.shutdown()
				return
			}
		}
	}
}

func (d *Daemon) shutdown() {
	d.workspaces.StopAll()
	if d.listener != nil {
		d.listener.Close()
	}
	os.Remove(d.socketPath)
}
```

Note: The test in step 1 uses a simple newline-delimited JSON approach. The
actual `jsonrpc.NewConn` from `go-mcp` may use Content-Length framing. Adjust
the test's write/read to match the framing protocol used by `jsonrpc.Conn`. If
`jsonrpc.Conn` requires `Content-Length` headers, wrap the test client
accordingly. Check `libs/go-mcp/jsonrpc/conn.go` for the exact framing format.

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test -v -run TestDaemon ./packages/lux/internal/service/`
Expected: PASS (adjust framing if needed)

**Step 5: Commit**

```
git add packages/lux/internal/service/daemon.go packages/lux/internal/service/daemon_test.go
git commit -m "feat(lux): add service daemon with idle timeout"
```

---

## Task 6: Implement LSP Proxy Client (`lux lsp`)

**Files:**
- Create: `packages/lux/internal/service/lspclient.go`
- Modify: `packages/lux/cmd/lux/app.go` (rename `serve` → `lsp`, wire proxy)

The LSP proxy client connects to the service, registers a session, and
proxies JSON-RPC between stdin/stdout and the service socket. Editors launch
this binary and don't know about the service.

**Step 1: Write the failing test**

```go
package service

import (
	"testing"
)

func TestLSPClient_WrapMessage(t *testing.T) {
	c := &LSPClient{sessionID: "abc123"}

	wrapped := c.wrapRequest("textDocument/completion", []byte(`{"position":{"line":1}}`))
	if wrapped.LSPMethod != "textDocument/completion" {
		t.Errorf("got %q, want textDocument/completion", wrapped.LSPMethod)
	}
	if wrapped.SessionID != "abc123" {
		t.Errorf("got %q, want abc123", wrapped.SessionID)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test -v -run TestLSPClient ./packages/lux/internal/service/`
Expected: FAIL — `LSPClient` undefined

**Step 3: Write minimal implementation**

```go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"

	"code.linenisgreat.com/purse-first/libs/go-mcp/jsonrpc"
)

// LSPClient proxies LSP JSON-RPC from stdin/stdout to the service socket.
type LSPClient struct {
	socketPath    string
	workspaceRoot string
	sessionID     string
	serviceConn   *jsonrpc.Conn
	clientConn    *jsonrpc.Conn
}

func NewLSPClient(socketPath, workspaceRoot string) *LSPClient {
	return &LSPClient{
		socketPath:    socketPath,
		workspaceRoot: workspaceRoot,
	}
}

func (c *LSPClient) Run(ctx context.Context) error {
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return fmt.Errorf("connecting to service: %w (is lux service running?)", err)
	}
	defer conn.Close()

	// Register session
	c.serviceConn = jsonrpc.NewConn(conn, conn, c.handleServiceMessage)

	go c.serviceConn.Run(ctx)

	result, err := c.serviceConn.Call(ctx, MethodSessionRegister, RegisterParams{
		WorkspaceRoot: c.workspaceRoot,
		ClientType:    ClientTypeLSP,
	})
	if err != nil {
		return fmt.Errorf("registering session: %w", err)
	}

	var reg RegisterResult
	if err := json.Unmarshal(result, &reg); err != nil {
		return fmt.Errorf("parsing register result: %w", err)
	}
	c.sessionID = reg.SessionID

	// Set up stdin/stdout JSON-RPC connection for the editor
	c.clientConn = jsonrpc.NewConn(os.Stdin, os.Stdout, c.handleClientMessage)

	return c.clientConn.Run(ctx)
}

// handleClientMessage proxies editor LSP messages to the service.
func (c *LSPClient) handleClientMessage(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	if msg.IsRequest() {
		wrapped := c.wrapRequest(msg.Method, msg.Params)
		result, err := c.serviceConn.Call(ctx, MethodLSPRequest, wrapped)
		if err != nil {
			if rpcErr, ok := err.(*jsonrpc.Error); ok {
				return jsonrpc.NewErrorResponse(*msg.ID, rpcErr.Code, rpcErr.Message, rpcErr.Data)
			}
			return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, err.Error(), nil)
		}
		resp, _ := jsonrpc.NewResponse(*msg.ID, nil)
		resp.Result = result
		return resp, nil
	}

	// Notification — fire and forget
	wrapped := c.wrapNotification(msg.Method, msg.Params)
	c.serviceConn.Notify(MethodLSPNotification, wrapped)
	return nil, nil
}

// handleServiceMessage handles messages from the service (LSP notifications).
func (c *LSPClient) handleServiceMessage(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	if msg.Method == MethodLSPNotification {
		var params LSPNotificationParams
		if err := json.Unmarshal(msg.Params, &params); err == nil {
			c.clientConn.Notify(params.LSPMethod, params.LSPParams)
		}
	}
	return nil, nil
}

func (c *LSPClient) wrapRequest(method string, params json.RawMessage) LSPRequestParams {
	return LSPRequestParams{
		SessionID: c.sessionID,
		LSPMethod: method,
		LSPParams: params,
	}
}

func (c *LSPClient) wrapNotification(method string, params json.RawMessage) LSPNotificationParams {
	return LSPNotificationParams{
		SessionID: c.sessionID,
		LSPMethod: method,
		LSPParams: params,
	}
}

// Close deregisters the session.
func (c *LSPClient) Close(ctx context.Context) {
	if c.serviceConn != nil && c.sessionID != "" {
		c.serviceConn.Call(ctx, MethodSessionDeregister, DeregisterParams{
			SessionID: c.sessionID,
		})
	}
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test -v -run TestLSPClient ./packages/lux/internal/service/`
Expected: PASS

**Step 5: Commit**

```
git add packages/lux/internal/service/lspclient.go
git commit -m "feat(lux): add LSP proxy client for service"
```

---

## Task 7: Wire CLI Commands

**Files:**
- Modify: `packages/lux/cmd/lux/app.go`
  - Add `service run`, `service install`, `service uninstall`, `service status`, `service logs`
  - Rename `serve` → `lsp`
  - Update `mcp stdio` to proxy through service
  - Update `status`, `start`, `stop`, `warmup` to use service protocol

This task modifies the CLI entrypoint to wire up the new service commands. The
existing `serve` command becomes `lsp` (thin proxy). A new `service` command
group manages the daemon lifecycle.

**Step 1: Add `service run` command**

In `addCLICommands()` in `app.go`, add a `service` command group after the
existing commands. The `service run` subcommand creates a `Daemon` and runs it.

```go
// In addCLICommands(), after existing commands:

serviceApp := command.NewSubApp("service", "Manage the lux service daemon")

serviceApp.AddCommand("run", "Run the service daemon (called by launchd/systemd)", func(args []string) error {
    cfg, err := config.Load()
    if err != nil {
        return err
    }
    socketPath := cfg.SocketPath()
    d := service.NewDaemon(socketPath, cfg, 30*time.Minute)
    return d.Run(context.Background())
})

serviceApp.AddCommand("install", "Install launchd/systemd service", func(args []string) error {
    return installService()
})

serviceApp.AddCommand("uninstall", "Remove launchd/systemd service", func(args []string) error {
    return uninstallService()
})

serviceApp.AddCommand("status", "Show service status", func(args []string) error {
    // Connect to service and call lux/pool.status
    return serviceStatus()
})

app.AddSubApp(serviceApp)
```

**Step 2: Rename `serve` to `lsp`**

Change the command registration at line ~125 from `"serve"` to `"lsp"`. Update
the handler to create an `LSPClient` proxy instead of a full `Server`.

```go
app.AddCommand("lsp", "Start LSP proxy (connects to service)", func(args []string) error {
    cfg, err := config.Load()
    if err != nil {
        return err
    }
    client := service.NewLSPClient(cfg.SocketPath(), projectRoot())
    return client.Run(context.Background())
})
```

**Step 3: Update `mcp stdio` to proxy through service**

Similar to `lsp`, the `mcp stdio` command creates an MCP proxy client that
connects to the service.

**Step 4: Update management commands**

`status`, `start`, `stop`, `warmup` should connect to the service socket using
the new JSON-RPC protocol instead of the old line-based control protocol.

**Step 5: Run the build**

Run: `nix develop --command go build ./packages/lux/cmd/lux/`
Expected: Compiles successfully

**Step 6: Commit**

```
git add packages/lux/cmd/lux/app.go
git commit -m "feat(lux): wire service commands and rename serve to lsp"
```

---

## Task 8: Implement launchd Plist Generation

**Files:**
- Create: `packages/lux/internal/service/install.go`
- Test: `packages/lux/internal/service/install_test.go`

Generates and loads a launchd plist for socket activation on macOS. On Linux,
generates systemd socket + service units.

**Step 1: Write the failing test**

```go
package service

import (
	"strings"
	"testing"
)

func TestGenerateLaunchdPlist(t *testing.T) {
	plist := GenerateLaunchdPlist("/nix/store/xxx-lux/bin/lux", "/tmp/lux.sock")
	if !strings.Contains(plist, "com.lux.service") {
		t.Error("expected label com.lux.service")
	}
	if !strings.Contains(plist, "/nix/store/xxx-lux/bin/lux") {
		t.Error("expected binary path")
	}
	if !strings.Contains(plist, "SockPathName") {
		t.Error("expected socket activation config")
	}
	if !strings.Contains(plist, "/tmp/lux.sock") {
		t.Error("expected socket path")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test -v -run TestGenerateLaunchd ./packages/lux/internal/service/`
Expected: FAIL — `GenerateLaunchdPlist` undefined

**Step 3: Write minimal implementation**

```go
package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"text/template"
)

const launchdPlistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.lux.service</string>
  <key>ProgramArguments</key>
  <array>
    <string>{{.BinaryPath}}</string>
    <string>service</string>
    <string>run</string>
  </array>
  <key>Sockets</key>
  <dict>
    <key>Listeners</key>
    <dict>
      <key>SockPathName</key>
      <string>{{.SocketPath}}</string>
    </dict>
  </dict>
  <key>StandardOutPath</key>
  <string>{{.LogDir}}/lux-service.log</string>
  <key>StandardErrorPath</key>
  <string>{{.LogDir}}/lux-service.err</string>
</dict>
</plist>
`

type launchdConfig struct {
	BinaryPath string
	SocketPath string
	LogDir     string
}

func GenerateLaunchdPlist(binaryPath, socketPath string) string {
	homeDir, _ := os.UserHomeDir()
	logDir := filepath.Join(homeDir, "Library", "Logs", "lux")

	cfg := launchdConfig{
		BinaryPath: binaryPath,
		SocketPath: socketPath,
		LogDir:     logDir,
	}

	tmpl := template.Must(template.New("plist").Parse(launchdPlistTemplate))
	var buf strings.Builder
	tmpl.Execute(&buf, cfg)
	return buf.String()
}

func InstallService(binaryPath, socketPath string) error {
	switch runtime.GOOS {
	case "darwin":
		return installLaunchd(binaryPath, socketPath)
	case "linux":
		return installSystemd(binaryPath, socketPath)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func UninstallService() error {
	switch runtime.GOOS {
	case "darwin":
		return uninstallLaunchd()
	case "linux":
		return uninstallSystemd()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func installLaunchd(binaryPath, socketPath string) error {
	homeDir, _ := os.UserHomeDir()
	plistDir := filepath.Join(homeDir, "Library", "LaunchAgents")
	plistPath := filepath.Join(plistDir, "com.lux.service.plist")
	logDir := filepath.Join(homeDir, "Library", "Logs", "lux")

	os.MkdirAll(plistDir, 0o755)
	os.MkdirAll(logDir, 0o755)

	plist := GenerateLaunchdPlist(binaryPath, socketPath)
	if err := os.WriteFile(plistPath, []byte(plist), 0o644); err != nil {
		return fmt.Errorf("writing plist: %w", err)
	}

	return exec.Command("launchctl", "load", plistPath).Run()
}

func uninstallLaunchd() error {
	homeDir, _ := os.UserHomeDir()
	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", "com.lux.service.plist")
	exec.Command("launchctl", "unload", plistPath).Run()
	return os.Remove(plistPath)
}

func installSystemd(binaryPath, socketPath string) error {
	// TODO: Generate systemd .socket + .service units
	return fmt.Errorf("systemd install not yet implemented")
}

func uninstallSystemd() error {
	return fmt.Errorf("systemd uninstall not yet implemented")
}
```

Note: `install.go` needs `"strings"` import for the `GenerateLaunchdPlist`
function. Add it to the imports.

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test -v -run TestGenerateLaunchd ./packages/lux/internal/service/`
Expected: PASS

**Step 5: Commit**

```
git add packages/lux/internal/service/install.go packages/lux/internal/service/install_test.go
git commit -m "feat(lux): add launchd plist generation for socket activation"
```

---

## Task 9: Service Notification Broadcasting

**Files:**
- Modify: `packages/lux/internal/service/daemon.go`
- Modify: `packages/lux/internal/service/handler.go`
- Modify: `packages/lux/internal/service/workspace.go`
- Test: `packages/lux/internal/service/handler_test.go`

Wire up LSP notification broadcasting. When an LSP sends a notification
(diagnostics, progress), the service forwards it to all sessions that have the
relevant file open.

**Step 1: Write the failing test**

Add to `handler_test.go`:

```go
func TestHandler_NotificationBroadcast(t *testing.T) {
	// Verify that when a notification arrives from an LSP, it gets
	// forwarded to sessions that have the relevant workspace open.
	// This requires a mock pool and connection tracking.
}
```

The actual test will depend on how connections are tracked in the daemon. The
key behavior: when the workspace pool's notification handler fires, the daemon
looks up all sessions for that workspace and sends the notification to each
session's connection.

**Step 2: Wire notification handler in WorkspaceRegistry**

Update `WorkspaceRegistry.GetOrCreate` to accept a `NotificationBroadcaster`
callback. The pool's `handlerFactory` calls this broadcaster, which looks up
sessions and forwards to their connections.

```go
// In workspace.go, update the pool creation:
pool := subprocess.NewPool(executor, func(lspName string) jsonrpc.Handler {
    return func(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
        if r.broadcaster != nil {
            return r.broadcaster(root, lspName, ctx, msg)
        }
        return nil, nil
    }
})
```

**Step 3: Implement broadcaster in Daemon**

```go
// In daemon.go:
func (d *Daemon) broadcastNotification(workspace, lspName string, ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
    sessions := d.sessions.SessionsForWorkspace(workspace)
    for _, s := range sessions {
        d.mu.Lock()
        // Find the connection for this session
        for conn, sid := range d.conns {
            if sid == s.ID {
                // Send notification to this client
                wrapped := LSPNotificationParams{
                    SessionID: s.ID,
                    LSPMethod: msg.Method,
                    LSPParams: msg.Params,
                }
                // TODO: use the per-conn jsonrpc.Conn to send notification
            }
        }
        d.mu.Unlock()
    }
    return nil, nil
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test -v ./packages/lux/internal/service/`
Expected: PASS

**Step 5: Commit**

```
git add packages/lux/internal/service/daemon.go packages/lux/internal/service/handler.go packages/lux/internal/service/workspace.go
git commit -m "feat(lux): add LSP notification broadcasting to sessions"
```

---

## Task 10: Integration Test — Full Round Trip

**Files:**
- Create: `packages/lux/internal/service/integration_test.go`

End-to-end test: start a daemon, connect an LSP client, send an `initialize`
request through the proxy, verify it reaches the service and returns
capabilities. Uses a mock executor (no real LSP subprocesses needed).

**Step 1: Write the integration test**

```go
package service

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"code.linenisgreat.com/purse-first/libs/go-mcp/jsonrpc"
)

func TestIntegration_FullRoundTrip(t *testing.T) {
	socketPath := t.TempDir() + "/lux.sock"
	d := NewDaemon(socketPath, nil, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go d.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	// Connect and register
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := jsonrpc.NewConn(conn, conn, nil)
	go client.Run(ctx)

	// Register session
	result, err := client.Call(ctx, MethodSessionRegister, RegisterParams{
		WorkspaceRoot: "/proj/a",
		ClientType:    ClientTypeLSP,
	})
	if err != nil {
		t.Fatal(err)
	}

	var reg RegisterResult
	json.Unmarshal(result, &reg)
	if reg.SessionID == "" {
		t.Fatal("expected session ID")
	}

	// Query pool status
	statusResult, err := client.Call(ctx, MethodPoolStatus, nil)
	if err != nil {
		t.Fatal(err)
	}

	var status map[string]any
	json.Unmarshal(statusResult, &status)
	if status["sessions"].(float64) != 1 {
		t.Errorf("expected 1 session, got %v", status["sessions"])
	}

	// Deregister
	_, err = client.Call(ctx, MethodSessionDeregister, DeregisterParams{
		SessionID: reg.SessionID,
	})
	if err != nil {
		t.Fatal(err)
	}

	cancel()
}
```

**Step 2: Run test**

Run: `nix develop --command go test -v -run TestIntegration ./packages/lux/internal/service/`
Expected: PASS

**Step 3: Commit**

```
git add packages/lux/internal/service/integration_test.go
git commit -m "test(lux): add service integration test for full round trip"
```

---

## Task 11: Update Existing Control Commands

**Files:**
- Modify: `packages/lux/internal/control/socket.go`
- Modify: `packages/lux/cmd/lux/app.go`

Update `status`, `start`, `stop`, `warmup` CLI commands to use the new service
JSON-RPC protocol instead of the old line-based control protocol. The old
`control.Client` connects to the service socket and speaks JSON-RPC.

**Step 1: Update control.Client to use JSON-RPC**

Replace the line-based `sendCommand` with JSON-RPC calls to the service
protocol methods (`lux/pool.status`, `lux/pool.start`, etc.).

**Step 2: Update app.go command handlers**

Each management command creates a `control.Client`, calls the appropriate
service method, and prints the result.

**Step 3: Run the build**

Run: `nix develop --command go build ./packages/lux/cmd/lux/`
Expected: Compiles

**Step 4: Commit**

```
git add packages/lux/internal/control/socket.go packages/lux/cmd/lux/app.go
git commit -m "refactor(lux): update control commands to use service protocol"
```

---

## Task 12: Socket Activation Support

**Files:**
- Modify: `packages/lux/internal/service/daemon.go`

When launched by launchd/systemd with socket activation, the daemon should
inherit the listener fd instead of creating a new socket. Check for
`LISTEN_FDS` (systemd) or launchd socket inheritance.

**Step 1: Detect socket activation**

```go
func (d *Daemon) Run(ctx context.Context) error {
    var listener net.Listener
    var err error

    if fd := socketActivationFD(); fd >= 0 {
        // Inherited from launchd/systemd
        f := os.NewFile(uintptr(fd), "listener")
        listener, err = net.FileListener(f)
        f.Close()
    } else {
        os.Remove(d.socketPath)
        listener, err = net.Listen("unix", d.socketPath)
    }
    // ... rest of Run
}
```

**Step 2: Implement fd detection**

On macOS (launchd): use `launch_activate_socket` via cgo or check inherited fds.
On Linux (systemd): check `LISTEN_FDS` and `LISTEN_PID` environment variables.

For initial implementation, use the `LISTEN_FDS` approach which works on both
platforms when properly configured.

**Step 3: Run test**

Run: `nix develop --command go test -v ./packages/lux/internal/service/`
Expected: PASS (falls back to creating socket when not activated)

**Step 4: Commit**

```
git add packages/lux/internal/service/daemon.go
git commit -m "feat(lux): add socket activation support for launchd/systemd"
```

---

## Summary

| Task | Component | Description |
|------|-----------|-------------|
| 1 | Protocol | Service protocol types |
| 2 | Session | Session registry with ref-counted docs |
| 3 | Workspace | Workspace registry with per-workspace pools |
| 4 | Handler | Service handler dispatching |
| 5 | Daemon | Unix socket listener with idle timeout |
| 6 | LSP Client | Thin LSP proxy (stdin/stdout ↔ service) |
| 7 | CLI | Wire commands, rename serve → lsp |
| 8 | Install | launchd/systemd plist generation |
| 9 | Notifications | LSP notification broadcasting |
| 10 | Integration | End-to-end round trip test |
| 11 | Control | Update management commands to new protocol |
| 12 | Activation | Socket activation fd inheritance |

Tasks 1-6 are the core architecture. Tasks 7-12 are integration and polish.
Each task is independently committable and testable.
