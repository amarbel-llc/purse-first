package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
)

func TestHandler_SessionRegister(t *testing.T) {
	h := newTestHandler(t)

	params := RegisterParams{
		WorkspaceRoot: "/proj/a",
		ClientType:    ClientTypeLSP,
	}
	paramsJSON, _ := json.Marshal(params)

	id := jsonrpc.NewNumberID(1)
	msg := &jsonrpc.Message{
		JSONRPC: jsonrpc.Version,
		ID:      &id,
		Method:  MethodSessionRegister,
		Params:  paramsJSON,
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
	sessionID := registerTestSession(t, h, "/proj/a", ClientTypeLSP)

	params := DeregisterParams{SessionID: sessionID}
	paramsJSON, _ := json.Marshal(params)
	id := jsonrpc.NewNumberID(2)
	msg := &jsonrpc.Message{
		JSONRPC: jsonrpc.Version,
		ID:      &id,
		Method:  MethodSessionDeregister,
		Params:  paramsJSON,
	}

	resp, err := h.Handle(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	if h.sessions.ActiveCount() != 0 {
		t.Error("expected 0 active sessions")
	}
}

func TestHandler_SessionRegisterCreatesWorkspace(t *testing.T) {
	h := newTestHandler(t)
	registerTestSession(t, h, "/proj/b", ClientTypeMCP)

	if h.workspaces.Count() != 1 {
		t.Errorf("expected 1 workspace, got %d", h.workspaces.Count())
	}

	_, ok := h.workspaces.Get("/proj/b")
	if !ok {
		t.Error("expected workspace for /proj/b")
	}
}

func TestHandler_PoolStatus(t *testing.T) {
	h := newTestHandler(t)
	registerTestSession(t, h, "/proj/a", ClientTypeLSP)

	id := jsonrpc.NewNumberID(3)
	msg := &jsonrpc.Message{
		JSONRPC: jsonrpc.Version,
		ID:      &id,
		Method:  MethodPoolStatus,
	}

	resp, err := h.Handle(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}

	var status poolStatusResult
	if err := json.Unmarshal(resp.Result, &status); err != nil {
		t.Fatal(err)
	}
	if status.SessionCount != 1 {
		t.Errorf("expected session_count=1, got %d", status.SessionCount)
	}
	if status.WorkspaceCount != 1 {
		t.Errorf("expected workspace_count=1, got %d", status.WorkspaceCount)
	}
}

func TestHandler_UnknownMethod(t *testing.T) {
	h := newTestHandler(t)
	id := jsonrpc.NewNumberID(4)
	msg := &jsonrpc.Message{
		JSONRPC: jsonrpc.Version,
		ID:      &id,
		Method:  "lux/nonexistent",
	}

	resp, err := h.Handle(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("expected error response")
	}
	if resp.Error == nil {
		t.Fatal("expected error in response")
	}
	if resp.Error.Code != jsonrpc.MethodNotFound {
		t.Errorf("expected MethodNotFound code %d, got %d", jsonrpc.MethodNotFound, resp.Error.Code)
	}
}

func TestHandler_InvalidParams(t *testing.T) {
	h := newTestHandler(t)
	id := jsonrpc.NewNumberID(5)
	msg := &jsonrpc.Message{
		JSONRPC: jsonrpc.Version,
		ID:      &id,
		Method:  MethodSessionRegister,
		Params:  json.RawMessage(`{invalid`),
	}

	resp, err := h.Handle(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("expected error response")
	}
	if resp.Error == nil {
		t.Fatal("expected error in response")
	}
	if resp.Error.Code != jsonrpc.InvalidParams {
		t.Errorf("expected InvalidParams code %d, got %d", jsonrpc.InvalidParams, resp.Error.Code)
	}
}

func TestHandler_DeregisterUnknownSession(t *testing.T) {
	h := newTestHandler(t)

	params := DeregisterParams{SessionID: "nonexistent"}
	paramsJSON, _ := json.Marshal(params)
	id := jsonrpc.NewNumberID(6)
	msg := &jsonrpc.Message{
		JSONRPC: jsonrpc.Version,
		ID:      &id,
		Method:  MethodSessionDeregister,
		Params:  paramsJSON,
	}

	resp, err := h.Handle(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	// Deregistering an unknown session should succeed silently
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Error != nil {
		t.Errorf("expected no error, got: %v", resp.Error)
	}
}

func TestHandler_Warmup(t *testing.T) {
	h := newTestHandler(t)

	params := WarmupParams{Dir: "/proj/warm"}
	paramsJSON, _ := json.Marshal(params)
	id := jsonrpc.NewNumberID(7)
	msg := &jsonrpc.Message{
		JSONRPC: jsonrpc.Version,
		ID:      &id,
		Method:  MethodWarmup,
		Params:  paramsJSON,
	}

	resp, err := h.Handle(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Error != nil {
		t.Errorf("expected no error, got: %v", resp.Error)
	}

	if h.workspaces.Count() != 1 {
		t.Errorf("expected 1 workspace after warmup, got %d", h.workspaces.Count())
	}
}

func TestWorkspaceRegistry_BroadcasterWiredToPool(t *testing.T) {
	reg := NewWorkspaceRegistry(nil)

	var broadcasterCalled bool
	var broadcasterWorkspace string
	var broadcasterLSP string
	reg.SetBroadcaster(func(workspace, lspName string, ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
		broadcasterCalled = true
		broadcasterWorkspace = workspace
		broadcasterLSP = lspName
		return nil, nil
	})

	ws := reg.GetOrCreate("/proj/test")
	if ws == nil {
		t.Fatal("expected workspace")
	}

	// Verify the broadcaster field is set on the registry.
	reg.mu.RLock()
	hasBroadcaster := reg.broadcaster != nil
	reg.mu.RUnlock()
	if !hasBroadcaster {
		t.Error("expected broadcaster to be set on registry")
	}

	// Verify the pool's handler factory produces a handler that calls the
	// broadcaster. We can test this by invoking the handler returned for
	// a given LSP name directly.
	handler := ws.Pool.HandlerForLSP("test-lsp")
	if handler == nil {
		t.Fatal("expected handler from pool handler factory")
	}

	msg := &jsonrpc.Message{
		JSONRPC: jsonrpc.Version,
		Method:  "textDocument/publishDiagnostics",
		Params:  json.RawMessage(`{"uri":"file:///test.go"}`),
	}

	_, err := handler(context.Background(), msg)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if !broadcasterCalled {
		t.Error("expected broadcaster to be called")
	}
	if broadcasterWorkspace != "/proj/test" {
		t.Errorf("expected workspace '/proj/test', got %q", broadcasterWorkspace)
	}
	if broadcasterLSP != "test-lsp" {
		t.Errorf("expected lsp 'test-lsp', got %q", broadcasterLSP)
	}
}

func TestHandler_LSPRequestUnknownSession(t *testing.T) {
	h := newTestHandler(t)

	params := LSPRequestParams{
		SessionID: "nonexistent",
		LSPMethod: "textDocument/hover",
		LSPParams: json.RawMessage(`{"textDocument":{"uri":"file:///test.go"}}`),
	}
	paramsJSON, _ := json.Marshal(params)
	id := jsonrpc.NewNumberID(10)
	msg := &jsonrpc.Message{
		JSONRPC: jsonrpc.Version,
		ID:      &id,
		Method:  MethodLSPRequest,
		Params:  paramsJSON,
	}

	resp, err := h.Handle(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("expected error response")
	}
	if resp.Error == nil {
		t.Fatal("expected error for unknown session")
	}
	if resp.Error.Code != jsonrpc.InvalidParams {
		t.Errorf("expected InvalidParams code %d, got %d", jsonrpc.InvalidParams, resp.Error.Code)
	}
}

func TestHandler_LSPRequestNoMatchingLSP(t *testing.T) {
	h := newTestHandler(t)
	sessionID := registerTestSession(t, h, "/proj/c", ClientTypeLSP)

	// Request with a URI that won't match any LSP (no filetype configs loaded)
	params := LSPRequestParams{
		SessionID: sessionID,
		LSPMethod: "textDocument/hover",
		LSPParams: json.RawMessage(`{"textDocument":{"uri":"file:///unknown.xyz"}}`),
	}
	paramsJSON, _ := json.Marshal(params)
	id := jsonrpc.NewNumberID(11)
	msg := &jsonrpc.Message{
		JSONRPC: jsonrpc.Version,
		ID:      &id,
		Method:  MethodLSPRequest,
		Params:  paramsJSON,
	}

	resp, err := h.Handle(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("expected error response")
	}
	if resp.Error == nil {
		t.Fatal("expected error for no matching LSP")
	}
	if resp.Error.Code != jsonrpc.InternalError {
		t.Errorf("expected InternalError code %d, got %d", jsonrpc.InternalError, resp.Error.Code)
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
	id := jsonrpc.NewNumberID(99)
	msg := &jsonrpc.Message{
		JSONRPC: jsonrpc.Version,
		ID:      &id,
		Method:  MethodSessionRegister,
		Params:  paramsJSON,
	}
	resp, err := h.Handle(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	var result RegisterResult
	json.Unmarshal(resp.Result, &result)
	return result.SessionID
}
