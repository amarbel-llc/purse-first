package service

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
)

func TestIntegration_FullRoundTrip(t *testing.T) {
	socketPath := t.TempDir() + "/lux.sock"
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
