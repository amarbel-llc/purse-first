package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/amarbel-llc/lux/internal/lsp"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
)

func TestServiceDocumentManager_IsOpenReturnsFalseForUnknown(t *testing.T) {
	dm := newTestServiceDocMgr(t)

	if dm.IsOpen("file:///nonexistent.go") {
		t.Error("expected IsOpen to return false for unknown URI")
	}
}

func TestServiceDocumentManager_OpenSendsDidOpenAndTracksDocument(t *testing.T) {
	dm, recorder := newTestServiceDocMgrWithRecorder(t)

	tmpFile := createTempFile(t, "test.go", "package main\n")
	uri := lsp.URIFromPath(tmpFile)

	if err := dm.Open(context.Background(), uri); err != nil {
		t.Fatalf("Open: %v", err)
	}

	if !dm.IsOpen(uri) {
		t.Error("expected IsOpen to return true after Open")
	}

	notifications := recorder.waitFor(t, 1)

	n := notifications[0]
	if n.method != MethodLSPNotification {
		t.Errorf("expected method %q, got %q", MethodLSPNotification, n.method)
	}

	var params LSPNotificationParams
	if err := json.Unmarshal(n.params, &params); err != nil {
		t.Fatalf("unmarshaling notification params: %v", err)
	}

	if params.LSPMethod != "textDocument/didOpen" {
		t.Errorf("expected LSP method textDocument/didOpen, got %q", params.LSPMethod)
	}

	if params.SessionID != "test-session" {
		t.Errorf("expected session ID test-session, got %q", params.SessionID)
	}
}

func TestServiceDocumentManager_OpenAlreadyOpenSendsDidChange(t *testing.T) {
	dm, recorder := newTestServiceDocMgrWithRecorder(t)

	tmpFile := createTempFile(t, "test.go", "package main\n")
	uri := lsp.URIFromPath(tmpFile)

	if err := dm.Open(context.Background(), uri); err != nil {
		t.Fatalf("first Open: %v", err)
	}

	// Modify the file content
	if err := os.WriteFile(tmpFile, []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	if err := dm.Open(context.Background(), uri); err != nil {
		t.Fatalf("second Open: %v", err)
	}

	notifications := recorder.waitFor(t, 2)

	methods := make(map[string]bool)
	for _, n := range notifications {
		var params LSPNotificationParams
		json.Unmarshal(n.params, &params)
		methods[params.LSPMethod] = true
	}

	if !methods["textDocument/didOpen"] {
		t.Error("expected a didOpen notification")
	}
	if !methods["textDocument/didChange"] {
		t.Error("expected a didChange notification")
	}
}

func TestServiceDocumentManager_CloseSendsDidCloseAndUntracks(t *testing.T) {
	dm, recorder := newTestServiceDocMgrWithRecorder(t)

	tmpFile := createTempFile(t, "test.go", "package main\n")
	uri := lsp.URIFromPath(tmpFile)

	dm.Open(context.Background(), uri)
	dm.Close(uri)

	if dm.IsOpen(uri) {
		t.Error("expected IsOpen to return false after Close")
	}

	recorder.waitForLSPMethod(t, "textDocument/didClose")

	notifications := recorder.notifications()
	if len(notifications) != 2 {
		t.Fatalf("expected 2 notifications (open + close), got %d", len(notifications))
	}
}

func TestServiceDocumentManager_CloseUnknownDocIsNoop(t *testing.T) {
	dm, recorder := newTestServiceDocMgrWithRecorder(t)

	dm.Close("file:///nonexistent.go")

	if len(recorder.notifications()) != 0 {
		t.Error("expected no notifications for closing unknown document")
	}
}

func TestServiceDocumentManager_CloseAllSendsDidCloseForAllDocs(t *testing.T) {
	dm, recorder := newTestServiceDocMgrWithRecorder(t)

	file1 := createTempFile(t, "a.go", "package a\n")
	file2 := createTempFile(t, "b.go", "package b\n")
	uri1 := lsp.URIFromPath(file1)
	uri2 := lsp.URIFromPath(file2)

	dm.Open(context.Background(), uri1)
	dm.Open(context.Background(), uri2)
	dm.CloseAll()

	if dm.IsOpen(uri1) || dm.IsOpen(uri2) {
		t.Error("expected all documents to be closed after CloseAll")
	}

	// 2 didOpen + 2 didClose = 4 notifications
	notifications := recorder.waitFor(t, 4)
	closeCount := 0
	for _, n := range notifications {
		var params LSPNotificationParams
		json.Unmarshal(n.params, &params)
		if params.LSPMethod == "textDocument/didClose" {
			closeCount++
		}
	}
	if closeCount != 2 {
		t.Errorf("expected 2 didClose notifications, got %d", closeCount)
	}
}

// Test helpers

type recordedNotification struct {
	method string
	params json.RawMessage
}

type notificationRecorder struct {
	mu    sync.Mutex
	items []recordedNotification
}

func (r *notificationRecorder) notifications() []recordedNotification {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]recordedNotification, len(r.items))
	copy(result, r.items)
	return result
}

func (r *notificationRecorder) waitFor(t *testing.T, count int) []recordedNotification {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		got := r.notifications()
		if len(got) >= count {
			return got
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for %d notifications, got %d", count, len(got))
			return nil
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func (r *notificationRecorder) waitForLSPMethod(t *testing.T, method string) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		for _, n := range r.notifications() {
			var params LSPNotificationParams
			if err := json.Unmarshal(n.params, &params); err == nil && params.LSPMethod == method {
				return
			}
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for LSP method %q", method)
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func stubInferLanguageID(uri lsp.DocumentURI) string {
	return "go"
}

func newTestServiceDocMgr(t *testing.T) *ServiceDocumentManager {
	t.Helper()
	dm, _ := newTestServiceDocMgrWithRecorder(t)
	return dm
}

func newTestServiceDocMgrWithRecorder(t *testing.T) (*ServiceDocumentManager, *notificationRecorder) {
	t.Helper()

	recorder := &notificationRecorder{}

	// Create a pipe-based jsonrpc connection. The "server" side records
	// notifications; the "client" side is used by the ServiceDocumentManager.
	clientReader, serverWriter, _ := os.Pipe()
	serverReader, clientWriter, _ := os.Pipe()

	t.Cleanup(func() {
		clientReader.Close()
		clientWriter.Close()
		serverReader.Close()
		serverWriter.Close()
	})

	serverConn := jsonrpc.NewConn(serverReader, serverWriter, func(_ context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
		if msg.IsNotification() {
			recorder.mu.Lock()
			recorder.items = append(recorder.items, recordedNotification{
				method: msg.Method,
				params: msg.Params,
			})
			recorder.mu.Unlock()
		}
		return nil, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go serverConn.Run(ctx)

	clientConn := jsonrpc.NewConn(clientReader, clientWriter, nil)
	go clientConn.Run(ctx)

	// Allow connections to start
	time.Sleep(10 * time.Millisecond)

	dm := NewServiceDocumentManager(clientConn, "test-session", stubInferLanguageID)

	return dm, recorder
}

func createTempFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	return path
}
