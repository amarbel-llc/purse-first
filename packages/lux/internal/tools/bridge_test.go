package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/amarbel-llc/lux/internal/lsp"
	"github.com/amarbel-llc/lux/internal/service"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
)

func TestNewServiceBridge_SetsServiceFields(t *testing.T) {
	conn := &jsonrpc.Conn{}
	b := NewServiceBridge(conn, "sess-123", nil, nil, nil)

	if b.serviceConn != conn {
		t.Error("expected serviceConn to be set")
	}
	if b.sessionID != "sess-123" {
		t.Errorf("expected sessionID sess-123, got %q", b.sessionID)
	}
	if b.pool != nil {
		t.Error("expected pool to be nil in service mode")
	}
	if b.router != nil {
		t.Error("expected router to be nil in service mode")
	}
}

func TestNewBridge_SetsLocalFields(t *testing.T) {
	b := NewBridge(nil, nil, nil, nil, nil)

	if b.serviceConn != nil {
		t.Error("expected serviceConn to be nil in local mode")
	}
	if b.sessionID != "" {
		t.Error("expected sessionID to be empty in local mode")
	}
}

func TestServiceBridge_WithDocumentRemoteRoutesToDaemon(t *testing.T) {
	// Set up a fake daemon that responds to lux/lsp.request
	recorder := &callRecorder{}

	clientReader, serverWriter, _ := os.Pipe()
	serverReader, clientWriter, _ := os.Pipe()
	t.Cleanup(func() {
		clientReader.Close()
		clientWriter.Close()
		serverReader.Close()
		serverWriter.Close()
	})

	serverConn := jsonrpc.NewConn(serverReader, serverWriter, func(_ context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
		if msg.IsRequest() && msg.Method == service.MethodLSPRequest {
			recorder.mu.Lock()
			recorder.calls = append(recorder.calls, recordedCall{
				method: msg.Method,
				params: msg.Params,
			})
			recorder.mu.Unlock()

			// Return a mock hover result
			result := json.RawMessage(`{"contents":{"kind":"markdown","value":"test hover"}}`)
			return jsonrpc.NewResponse(*msg.ID, result)
		}
		return nil, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go serverConn.Run(ctx)

	clientConn := jsonrpc.NewConn(clientReader, clientWriter, nil)
	go clientConn.Run(ctx)

	time.Sleep(10 * time.Millisecond)

	bridge := NewServiceBridge(clientConn, "test-session", nil, nil, nil)

	// Set up a stub document tracker that says doc is already open
	bridge.SetDocumentManager(&stubDocTracker{open: true})

	tmpFile := createTestFile(t, "test.go", "package main\n")
	uri := lsp.URIFromPath(tmpFile)

	result, err := bridge.Hover(ctx, uri, 0, 0)
	if err != nil {
		t.Fatalf("Hover: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Verify the daemon received the lux/lsp.request call
	calls := recorder.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call to daemon, got %d", len(calls))
	}

	if calls[0].method != service.MethodLSPRequest {
		t.Errorf("expected method %q, got %q", service.MethodLSPRequest, calls[0].method)
	}

	var reqParams service.LSPRequestParams
	if err := json.Unmarshal(calls[0].params, &reqParams); err != nil {
		t.Fatalf("unmarshaling request params: %v", err)
	}

	if reqParams.SessionID != "test-session" {
		t.Errorf("expected session ID test-session, got %q", reqParams.SessionID)
	}

	if reqParams.LSPMethod != "textDocument/hover" {
		t.Errorf("expected LSP method textDocument/hover, got %q", reqParams.LSPMethod)
	}
}

func TestServiceBridge_WithDocumentRemoteOpensDocumentFirst(t *testing.T) {
	clientReader, serverWriter, _ := os.Pipe()
	serverReader, clientWriter, _ := os.Pipe()
	t.Cleanup(func() {
		clientReader.Close()
		clientWriter.Close()
		serverReader.Close()
		serverWriter.Close()
	})

	serverConn := jsonrpc.NewConn(serverReader, serverWriter, func(_ context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
		if msg.IsRequest() {
			result := json.RawMessage(`null`)
			return jsonrpc.NewResponse(*msg.ID, result)
		}
		return nil, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go serverConn.Run(ctx)

	clientConn := jsonrpc.NewConn(clientReader, clientWriter, nil)
	go clientConn.Run(ctx)

	time.Sleep(10 * time.Millisecond)

	bridge := NewServiceBridge(clientConn, "test-session", nil, nil, nil)
	tracker := &stubDocTracker{open: false}
	bridge.SetDocumentManager(tracker)

	tmpFile := createTestFile(t, "test.go", "package main\n")
	uri := lsp.URIFromPath(tmpFile)

	// Definition will call withDocument which should open the document first
	bridge.Definition(ctx, uri, 0, 0)

	if !tracker.opened {
		t.Error("expected document manager Open to be called")
	}
}

// Helpers

type recordedCall struct {
	method string
	params json.RawMessage
}

type callRecorder struct {
	mu    sync.Mutex
	calls []recordedCall
}

func (r *callRecorder) getCalls() []recordedCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]recordedCall, len(r.calls))
	copy(result, r.calls)
	return result
}

type stubDocTracker struct {
	open   bool
	opened bool
}

func (s *stubDocTracker) IsOpen(_ lsp.DocumentURI) bool {
	return s.open
}

func (s *stubDocTracker) Open(_ context.Context, _ lsp.DocumentURI) error {
	s.opened = true
	s.open = true
	return nil
}

func createTestFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	return path
}
