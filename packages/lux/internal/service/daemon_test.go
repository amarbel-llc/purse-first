package service

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
)

// shortSocketPath creates a Unix socket path short enough to stay under the
// 108-byte sun_path limit. t.TempDir() can produce very long paths when TMPDIR
// is set (e.g. inside nix-shell), which causes bind to fail.
func shortSocketPath(t *testing.T, name string) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "lux-test-*")
	if err != nil {
		t.Fatalf("creating short temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir + "/" + name
}

func waitForSocket(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("socket %s did not appear within %v", path, timeout)
}

func waitForListeningSocket(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("unix", path)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("socket %s did not become connectable within %v", path, timeout)
}

func TestDaemon_AcceptAndRegister(t *testing.T) {
	socketPath := shortSocketPath(t, "lux.sock")
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

	clientConn := jsonrpc.NewConn(conn, conn, nil)
	go clientConn.Run(ctx)

	result, err := clientConn.Call(ctx, MethodSessionRegister, RegisterParams{
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
		t.Error("expected non-empty session ID")
	}

	if d.sessions.ActiveCount() != 1 {
		t.Errorf("expected 1 active session, got %d", d.sessions.ActiveCount())
	}

	cancel()
	<-errCh
}

func TestDaemon_DeregisterOnDisconnect(t *testing.T) {
	socketPath := shortSocketPath(t, "lux.sock")
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

	clientCtx, clientCancel := context.WithCancel(ctx)
	clientConn := jsonrpc.NewConn(conn, conn, nil)
	go clientConn.Run(clientCtx)

	result, err := clientConn.Call(clientCtx, MethodSessionRegister, RegisterParams{
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

	if d.sessions.ActiveCount() != 1 {
		t.Errorf("expected 1 active session before disconnect, got %d", d.sessions.ActiveCount())
	}

	clientCancel()
	conn.Close()

	// Allow time for the daemon to process the disconnect
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if d.sessions.ActiveCount() == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if d.sessions.ActiveCount() != 0 {
		t.Errorf("expected 0 active sessions after disconnect, got %d", d.sessions.ActiveCount())
	}

	cancel()
	<-errCh
}

func TestDaemon_MultipleClients(t *testing.T) {
	socketPath := shortSocketPath(t, "lux.sock")
	d := NewDaemon(socketPath, nil, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	waitForSocket(t, socketPath, 2*time.Second)

	// Connect two clients
	conn1, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dialing socket (client 1): %v", err)
	}
	defer conn1.Close()

	conn2, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dialing socket (client 2): %v", err)
	}
	defer conn2.Close()

	client1 := jsonrpc.NewConn(conn1, conn1, nil)
	go client1.Run(ctx)

	client2 := jsonrpc.NewConn(conn2, conn2, nil)
	go client2.Run(ctx)

	// Register from both clients
	result1, err := client1.Call(ctx, MethodSessionRegister, RegisterParams{
		WorkspaceRoot: "/proj/a",
		ClientType:    ClientTypeLSP,
	})
	if err != nil {
		t.Fatalf("register call (client 1): %v", err)
	}

	result2, err := client2.Call(ctx, MethodSessionRegister, RegisterParams{
		WorkspaceRoot: "/proj/b",
		ClientType:    ClientTypeMCP,
	})
	if err != nil {
		t.Fatalf("register call (client 2): %v", err)
	}

	var reg1, reg2 RegisterResult
	json.Unmarshal(result1, &reg1)
	json.Unmarshal(result2, &reg2)

	if reg1.SessionID == reg2.SessionID {
		t.Error("expected different session IDs for different clients")
	}

	if d.sessions.ActiveCount() != 2 {
		t.Errorf("expected 2 active sessions, got %d", d.sessions.ActiveCount())
	}

	cancel()
	<-errCh
}

func TestDaemon_IdleTimeout(t *testing.T) {
	socketPath := shortSocketPath(t, "idle.sock")
	d := NewDaemon(socketPath, nil, 200*time.Millisecond)
	d.idleCheckInterval = 30 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	waitForSocket(t, socketPath, 2*time.Second)

	// No clients connect, so idle timeout should fire
	select {
	case <-errCh:
		// Daemon shut down due to idle timeout
	case <-time.After(5 * time.Second):
		t.Fatal("daemon did not shut down after idle timeout")
	}

	// Socket should be cleaned up (Run removes it on exit)
	if _, err := os.Stat(socketPath); err == nil {
		t.Error("expected socket to be removed after shutdown")
	}
}

func TestDaemon_RemovesStaleSocket(t *testing.T) {
	socketPath := shortSocketPath(t, "lux.sock")

	// Create a stale socket file
	if err := os.WriteFile(socketPath, []byte{}, 0o600); err != nil {
		t.Fatalf("creating stale socket: %v", err)
	}

	d := NewDaemon(socketPath, nil, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Wait until the socket is actually connectable, not just file existence
	waitForListeningSocket(t, socketPath, 2*time.Second)

	// Verify we can connect and communicate
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dialing socket after stale removal: %v", err)
	}
	conn.Close()

	cancel()
	<-errCh
}

func TestRunWithListener(t *testing.T) {
	socketPath := shortSocketPath(t, "inherited.sock")

	// Create a listener and disable automatic socket file cleanup. This
	// simulates what happens with socket activation — the init system owns
	// the socket file and the daemon should not remove it.
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("creating listener: %v", err)
	}
	listener.(*net.UnixListener).SetUnlinkOnClose(false)

	d := NewDaemon(socketPath, nil, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.RunWithListener(ctx, listener)
	}()

	waitForListeningSocket(t, socketPath, 2*time.Second)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dialing socket: %v", err)
	}
	conn.Close()

	cancel()
	<-errCh

	// RunWithListener should NOT remove the socket — caller owns it
	if _, err := os.Stat(socketPath); err != nil {
		t.Error("expected socket to still exist after RunWithListener shutdown")
	}
}

func TestSystemdListener_NotSet(t *testing.T) {
	t.Setenv("LISTEN_PID", "")
	t.Setenv("LISTEN_FDS", "")

	l, err := SystemdListener()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l != nil {
		t.Error("expected nil listener when env vars not set")
	}
}

func TestSystemdListener_WrongPID(t *testing.T) {
	t.Setenv("LISTEN_PID", "99999")
	t.Setenv("LISTEN_FDS", "1")

	l, err := SystemdListener()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l != nil {
		t.Error("expected nil listener when PID doesn't match")
	}
}

func TestSystemdListener_ZeroFDs(t *testing.T) {
	t.Setenv("LISTEN_PID", strconv.Itoa(os.Getpid()))
	t.Setenv("LISTEN_FDS", "0")

	l, err := SystemdListener()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l != nil {
		t.Error("expected nil listener when LISTEN_FDS is 0")
	}
}

func TestSystemdListener_InvalidPID(t *testing.T) {
	t.Setenv("LISTEN_PID", "notanumber")
	t.Setenv("LISTEN_FDS", "1")

	l, err := SystemdListener()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l != nil {
		t.Error("expected nil listener when LISTEN_PID is invalid")
	}
}

func TestSystemdListener_InvalidFDs(t *testing.T) {
	t.Setenv("LISTEN_PID", strconv.Itoa(os.Getpid()))
	t.Setenv("LISTEN_FDS", "notanumber")

	l, err := SystemdListener()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l != nil {
		t.Error("expected nil listener when LISTEN_FDS is invalid")
	}
}
