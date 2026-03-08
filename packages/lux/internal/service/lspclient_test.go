package service

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
	"github.com/amarbel-llc/lux/internal/lsp"
)

func TestLSPClient_WrapRequest(t *testing.T) {
	c := &LSPClient{sessionID: "abc123"}
	wrapped := c.wrapRequest("textDocument/completion", []byte(`{"position":{"line":1}}`))

	if wrapped.LSPMethod != "textDocument/completion" {
		t.Errorf("LSPMethod: got %q, want %q", wrapped.LSPMethod, "textDocument/completion")
	}

	if wrapped.SessionID != "abc123" {
		t.Errorf("SessionID: got %q, want %q", wrapped.SessionID, "abc123")
	}

	var params map[string]any
	if err := json.Unmarshal(wrapped.LSPParams, &params); err != nil {
		t.Fatalf("unmarshaling LSPParams: %v", err)
	}

	pos, ok := params["position"]
	if !ok {
		t.Fatal("expected position key in LSPParams")
	}

	posMap, ok := pos.(map[string]any)
	if !ok {
		t.Fatal("expected position to be a map")
	}

	if posMap["line"] != float64(1) {
		t.Errorf("position.line: got %v, want 1", posMap["line"])
	}
}

func TestLSPClient_WrapNotification(t *testing.T) {
	c := &LSPClient{sessionID: "def456"}
	wrapped := c.wrapNotification("textDocument/didOpen", []byte(`{"textDocument":{"uri":"file:///a.go"}}`))

	if wrapped.LSPMethod != "textDocument/didOpen" {
		t.Errorf("LSPMethod: got %q, want %q", wrapped.LSPMethod, "textDocument/didOpen")
	}

	if wrapped.SessionID != "def456" {
		t.Errorf("SessionID: got %q, want %q", wrapped.SessionID, "def456")
	}

	var params map[string]any
	if err := json.Unmarshal(wrapped.LSPParams, &params); err != nil {
		t.Fatalf("unmarshaling LSPParams: %v", err)
	}

	td, ok := params["textDocument"]
	if !ok {
		t.Fatal("expected textDocument key in LSPParams")
	}

	tdMap, ok := td.(map[string]any)
	if !ok {
		t.Fatal("expected textDocument to be a map")
	}

	if tdMap["uri"] != "file:///a.go" {
		t.Errorf("textDocument.uri: got %v, want file:///a.go", tdMap["uri"])
	}
}

func TestLSPClient_ProxyRoundTrip(t *testing.T) {
	socketPath := shortSocketPath(t, "proxy.sock")
	d := NewDaemon(socketPath, nil, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	waitForSocket(t, socketPath, 2*time.Second)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dialing socket: %v", err)
	}
	defer conn.Close()

	svcConn := jsonrpc.NewConn(conn, conn, nil)
	go svcConn.Run(ctx)

	result, err := svcConn.Call(ctx, MethodSessionRegister, RegisterParams{
		WorkspaceRoot: "/proj/a",
		ClientType:    ClientTypeLSP,
	})
	if err != nil {
		t.Fatalf("register call: %v", err)
	}

	var reg RegisterResult
	if err := json.Unmarshal(result, &reg); err != nil {
		t.Fatalf("unmarshaling result: %v", err)
	}

	if reg.SessionID == "" {
		t.Fatal("expected non-empty session ID")
	}

	// Verify session was registered
	if d.sessions.ActiveCount() != 1 {
		t.Errorf("expected 1 active session, got %d", d.sessions.ActiveCount())
	}

	// Deregister and verify
	_, err = svcConn.Call(ctx, MethodSessionDeregister, DeregisterParams{
		SessionID: reg.SessionID,
	})
	if err != nil {
		t.Fatalf("deregister call: %v", err)
	}

	if d.sessions.ActiveCount() != 0 {
		t.Errorf("expected 0 active sessions after deregister, got %d", d.sessions.ActiveCount())
	}

	cancel()
	<-errCh
}

func TestLSPClient_NewLSPClient(t *testing.T) {
	c := NewLSPClient("/tmp/lux.sock", "/proj/a")

	if c.socketPath != "/tmp/lux.sock" {
		t.Errorf("socketPath: got %q, want %q", c.socketPath, "/tmp/lux.sock")
	}

	if c.workspaceRoot != "/proj/a" {
		t.Errorf("workspaceRoot: got %q, want %q", c.workspaceRoot, "/proj/a")
	}

	if c.sessionID != "" {
		t.Errorf("sessionID should be empty before Run, got %q", c.sessionID)
	}
}

func TestLSPClient_InitializeReturnsCapabilities(t *testing.T) {
	c := &LSPClient{}

	id := jsonrpc.NewNumberID(1)
	msg := &jsonrpc.Message{
		JSONRPC: jsonrpc.Version,
		ID:      &id,
		Method:  lsp.MethodInitialize,
		Params:  json.RawMessage(`{"capabilities":{}}`),
	}

	resp, err := c.handleClientMessage(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}

	var result lsp.InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshaling result: %v", err)
	}

	if result.ServerInfo == nil {
		t.Fatal("expected serverInfo")
	}
	if result.ServerInfo.Name != "lux" {
		t.Errorf("serverInfo.name: got %q, want %q", result.ServerInfo.Name, "lux")
	}
	if result.Capabilities.DefinitionProvider != true {
		t.Error("expected definitionProvider to be true")
	}
}

func TestLSPClient_InitializedSwallowed(t *testing.T) {
	c := &LSPClient{}

	msg := &jsonrpc.Message{
		JSONRPC: jsonrpc.Version,
		Method:  lsp.MethodInitialized,
		Params:  json.RawMessage(`{}`),
	}

	resp, err := c.handleClientMessage(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != nil {
		t.Errorf("expected nil response for initialized notification, got %+v", resp)
	}
}

func TestLSPClient_ShutdownReturnsNull(t *testing.T) {
	c := &LSPClient{}

	id := jsonrpc.NewNumberID(2)
	msg := &jsonrpc.Message{
		JSONRPC: jsonrpc.Version,
		ID:      &id,
		Method:  lsp.MethodShutdown,
	}

	resp, err := c.handleClientMessage(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Error != nil {
		t.Errorf("expected no error, got: %v", resp.Error)
	}
	if resp.Result != nil {
		t.Errorf("expected nil result, got %s", resp.Result)
	}
}

func TestLSPClient_ExitSwallowed(t *testing.T) {
	c := &LSPClient{}

	msg := &jsonrpc.Message{
		JSONRPC: jsonrpc.Version,
		Method:  lsp.MethodExit,
	}

	resp, err := c.handleClientMessage(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != nil {
		t.Errorf("expected nil response for exit notification, got %+v", resp)
	}
}

func TestLSPClient_DollarPrefixedMethodsSwallowed(t *testing.T) {
	c := &LSPClient{}

	methods := []string{"$/cancelRequest", "$/setTrace", "$/logTrace"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			msg := &jsonrpc.Message{
				JSONRPC: jsonrpc.Version,
				Method:  method,
			}

			resp, err := c.handleClientMessage(context.Background(), msg)
			if err != nil {
				t.Fatalf("unexpected error for %s: %v", method, err)
			}
			if resp != nil {
				t.Errorf("expected nil response for %s, got %+v", method, resp)
			}
		})
	}
}
