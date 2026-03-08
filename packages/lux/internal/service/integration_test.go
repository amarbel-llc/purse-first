package service

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/amarbel-llc/lux/internal/subprocess"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
)

func TestIntegration_FullRoundTrip(t *testing.T) {
	socketPath := shortSocketPath(t, "roundtrip.sock")
	d := NewDaemon(socketPath, nil, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	waitForSocket(t, socketPath, 2*time.Second)

	// Connect client
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dialing socket: %v", err)
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
		t.Fatalf("register call: %v", err)
	}

	var reg RegisterResult
	if err := json.Unmarshal(result, &reg); err != nil {
		t.Fatalf("unmarshaling register result: %v", err)
	}
	if reg.SessionID == "" {
		t.Fatal("expected non-empty session ID")
	}

	// Query pool status and verify state
	statusResult, err := client.Call(ctx, MethodPoolStatus, nil)
	if err != nil {
		t.Fatalf("pool status call: %v", err)
	}

	var status poolStatusResult
	if err := json.Unmarshal(statusResult, &status); err != nil {
		t.Fatalf("unmarshaling pool status: %v", err)
	}
	if status.SessionCount != 1 {
		t.Errorf("expected 1 session, got %d", status.SessionCount)
	}
	if status.WorkspaceCount != 1 {
		t.Errorf("expected 1 workspace, got %d", status.WorkspaceCount)
	}

	// Deregister session
	_, err = client.Call(ctx, MethodSessionDeregister, DeregisterParams{
		SessionID: reg.SessionID,
	})
	if err != nil {
		t.Fatalf("deregister call: %v", err)
	}

	// Verify session count dropped after deregister
	statusResult2, err := client.Call(ctx, MethodPoolStatus, nil)
	if err != nil {
		t.Fatalf("pool status call after deregister: %v", err)
	}

	var status2 poolStatusResult
	if err := json.Unmarshal(statusResult2, &status2); err != nil {
		t.Fatalf("unmarshaling pool status after deregister: %v", err)
	}
	if status2.SessionCount != 0 {
		t.Errorf("expected 0 sessions after deregister, got %d", status2.SessionCount)
	}

	cancel()
	<-errCh
}

func TestIntegration_LSPRequestRoundTrip(t *testing.T) {
	// Set up isolated config directory with filetype + LSP config
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	ftDir := filepath.Join(configDir, "lux", "filetype")
	if err := os.MkdirAll(ftDir, 0o755); err != nil {
		t.Fatalf("creating filetype dir: %v", err)
	}

	// Filetype config: .go files → "fake" LSP
	if err := os.WriteFile(filepath.Join(ftDir, "go.toml"), []byte("extensions = [\"go\"]\nlsp = \"fake\"\n"), 0o644); err != nil {
		t.Fatalf("writing filetype config: %v", err)
	}

	// LSP config so LoadWithProject succeeds
	luxDir := filepath.Join(configDir, "lux")
	if err := os.WriteFile(filepath.Join(luxDir, "lsps.toml"), []byte("[[lsp]]\nname = \"fake\"\nflake = \"fake#lsp\"\nextensions = [\"go\"]\n"), 0o644); err != nil {
		t.Fatalf("writing LSP config: %v", err)
	}

	socketPath := shortSocketPath(t, "lsp-roundtrip.sock")

	d := NewDaemon(socketPath, nil, 0)
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
	workDir := t.TempDir()
	regResult, err := client.Call(ctx, MethodSessionRegister, RegisterParams{
		WorkspaceRoot: workDir,
		ClientType:    ClientTypeLSP,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	var reg RegisterResult
	if err := json.Unmarshal(regResult, &reg); err != nil {
		t.Fatalf("unmarshal register: %v", err)
	}

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

func TestIntegration_LSPNotificationBroadcast(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	ftDir := filepath.Join(configDir, "lux", "filetype")
	if err := os.MkdirAll(ftDir, 0o755); err != nil {
		t.Fatalf("creating filetype dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(ftDir, "go.toml"), []byte("extensions = [\"go\"]\nlsp = \"fake\"\n"), 0o644); err != nil {
		t.Fatalf("writing filetype config: %v", err)
	}

	luxDir := filepath.Join(configDir, "lux")
	if err := os.WriteFile(filepath.Join(luxDir, "lsps.toml"), []byte("[[lsp]]\nname = \"fake\"\nflake = \"fake#lsp\"\nextensions = [\"go\"]\n"), 0o644); err != nil {
		t.Fatalf("writing LSP config: %v", err)
	}

	socketPath := shortSocketPath(t, "notify-broadcast.sock")

	d := NewDaemon(socketPath, nil, 0)
	d.workspaces.executorFactory = func() subprocess.Executor {
		return &notifyingFakeExecutor{}
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

	received := make(chan LSPNotificationParams, 1)
	client := jsonrpc.NewConn(conn, conn, func(_ context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
		if msg.Method == MethodLSPNotification {
			var params LSPNotificationParams
			if err := json.Unmarshal(msg.Params, &params); err == nil {
				received <- params
			}
		}
		return nil, nil
	})
	go client.Run(ctx)

	workDir := t.TempDir()
	regResult, err := client.Call(ctx, MethodSessionRegister, RegisterParams{
		WorkspaceRoot: workDir,
		ClientType:    ClientTypeLSP,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	var reg RegisterResult
	if err := json.Unmarshal(regResult, &reg); err != nil {
		t.Fatalf("unmarshal register: %v", err)
	}

	// Send didOpen — the notifying fake LSP responds with publishDiagnostics
	didOpenParams, _ := json.Marshal(map[string]any{
		"textDocument": map[string]any{
			"uri":        "file:///test.go",
			"languageId": "go",
			"version":    1,
			"text":       "package main\n",
		},
	})

	client.Notify(MethodLSPNotification, LSPNotificationParams{
		SessionID: reg.SessionID,
		LSPMethod: "textDocument/didOpen",
		LSPParams: didOpenParams,
	})

	select {
	case notification := <-received:
		if notification.LSPMethod != "textDocument/publishDiagnostics" {
			t.Errorf("expected LSPMethod %q, got %q", "textDocument/publishDiagnostics", notification.LSPMethod)
		}

		var diagParams map[string]any
		if err := json.Unmarshal(notification.LSPParams, &diagParams); err != nil {
			t.Fatalf("unmarshal diagnostic params: %v", err)
		}
		if diagParams["uri"] != "file:///test.go" {
			t.Errorf("expected diagnostic URI %q, got %v", "file:///test.go", diagParams["uri"])
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for publishDiagnostics notification")
	}

	cancel()
	<-errCh
}

func TestIntegration_MCPSessionRoundTrip(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	ftDir := filepath.Join(configDir, "lux", "filetype")
	if err := os.MkdirAll(ftDir, 0o755); err != nil {
		t.Fatalf("creating filetype dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(ftDir, "go.toml"), []byte("extensions = [\"go\"]\nlsp = \"fake\"\n"), 0o644); err != nil {
		t.Fatalf("writing filetype config: %v", err)
	}

	luxDir := filepath.Join(configDir, "lux")
	if err := os.WriteFile(filepath.Join(luxDir, "lsps.toml"), []byte("[[lsp]]\nname = \"fake\"\nflake = \"fake#lsp\"\nextensions = [\"go\"]\n"), 0o644); err != nil {
		t.Fatalf("writing LSP config: %v", err)
	}

	socketPath := shortSocketPath(t, "mcp-roundtrip.sock")

	d := NewDaemon(socketPath, nil, 0)
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

	// Register as MCP client
	workDir := t.TempDir()
	regResult, err := client.Call(ctx, MethodSessionRegister, RegisterParams{
		WorkspaceRoot: workDir,
		ClientType:    ClientTypeMCP,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	var reg RegisterResult
	if err := json.Unmarshal(regResult, &reg); err != nil {
		t.Fatalf("unmarshal register: %v", err)
	}

	if reg.SessionID == "" {
		t.Fatal("expected non-empty session ID")
	}

	// Verify pool status shows 1 MCP session
	statusResult, err := client.Call(ctx, MethodPoolStatus, nil)
	if err != nil {
		t.Fatalf("pool status: %v", err)
	}

	var status poolStatusResult
	if err := json.Unmarshal(statusResult, &status); err != nil {
		t.Fatalf("unmarshal status: %v", err)
	}

	if status.SessionCount != 1 {
		t.Errorf("expected 1 session, got %d", status.SessionCount)
	}

	// Send document lifecycle notification (didOpen) via lux/lsp.notification
	didOpenParams, _ := json.Marshal(map[string]any{
		"textDocument": map[string]any{
			"uri":        "file:///test.go",
			"languageId": "go",
			"version":    1,
			"text":       "package main\n",
		},
	})

	client.Notify(MethodLSPNotification, LSPNotificationParams{
		SessionID: reg.SessionID,
		LSPMethod: "textDocument/didOpen",
		LSPParams: didOpenParams,
	})

	// Small delay for notification to propagate through daemon to fake LSP
	time.Sleep(50 * time.Millisecond)

	// Send LSP request via lux/lsp.request — should route to fake LSP
	lspResult, err := client.Call(ctx, MethodLSPRequest, LSPRequestParams{
		SessionID: reg.SessionID,
		LSPMethod: "textDocument/hover",
		LSPParams: json.RawMessage(`{"textDocument":{"uri":"file:///test.go"},"position":{"line":0,"character":0}}`),
	})
	if err != nil {
		t.Fatalf("LSP request: %v", err)
	}

	var lspResp map[string]any
	if err := json.Unmarshal(lspResult, &lspResp); err != nil {
		t.Fatalf("unmarshal LSP response: %v", err)
	}

	if lspResp["echo"] != "textDocument/hover" {
		t.Errorf("expected echo of method, got: %v", lspResp)
	}

	// Deregister and verify cleanup
	_, err = client.Call(ctx, MethodSessionDeregister, DeregisterParams{
		SessionID: reg.SessionID,
	})
	if err != nil {
		t.Fatalf("deregister: %v", err)
	}

	statusResult2, err := client.Call(ctx, MethodPoolStatus, nil)
	if err != nil {
		t.Fatalf("pool status after deregister: %v", err)
	}

	var status2 poolStatusResult
	json.Unmarshal(statusResult2, &status2)
	if status2.SessionCount != 0 {
		t.Errorf("expected 0 sessions after deregister, got %d", status2.SessionCount)
	}

	cancel()
	<-errCh
}
