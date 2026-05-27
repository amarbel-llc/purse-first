package streamablehttp_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/server"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/transport"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/transport/streamablehttp"
)

type stubToolProvider struct{}

func (s *stubToolProvider) ListTools(ctx context.Context) ([]protocol.Tool, error) {
	return []protocol.Tool{
		{Name: "echo", Description: "Echo tool", InputSchema: json.RawMessage(`{"type":"object"}`)},
	}, nil
}

func (s *stubToolProvider) CallTool(ctx context.Context, name string, args json.RawMessage) (*protocol.ToolCallResult, error) {
	return &protocol.ToolCallResult{
		Content: []protocol.ContentBlock{protocol.TextContent("ok")},
	}, nil
}

// startTestServer brings up a StreamableHTTP transport + server.Server, runs
// the server in a goroutine, and returns the transport plus a stop function.
func startTestServer(t *testing.T, opts ...streamablehttp.Option) (*streamablehttp.StreamableHTTP, func()) {
	t.Helper()

	tr := streamablehttp.New("127.0.0.1:0", opts...)

	srv, err := server.New(tr, server.Options{
		ServerName:    "test",
		ServerVersion: "1.0",
		Tools:         &stubToolProvider{},
	})
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = srv.Run(ctx)
	}()

	// Wait until Addr() reflects the listening socket (Start happens inside Run).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if a := tr.Addr(); a != "" && !strings.HasSuffix(a, ":0") {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if a := tr.Addr(); a == "" || strings.HasSuffix(a, ":0") {
		cancel()
		t.Fatalf("transport never started listening (addr=%q)", a)
	}

	stop := func() {
		cancel()
		_ = tr.Close()
		<-done
	}
	return tr, stop
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func newInitializeBody(t *testing.T, id int64) []byte {
	t.Helper()
	params := protocol.InitializeParams{
		ProtocolVersion: protocol.ProtocolVersionV0,
		ClientInfo:      protocol.Implementation{Name: "test-client", Version: "1.0"},
	}
	rpcID := jsonrpc.NewNumberID(id)
	msg := jsonrpc.Message{
		JSONRPC: "2.0",
		ID:      &rpcID,
		Method:  "initialize",
		Params:  mustMarshal(t, params),
	}
	return mustMarshal(t, msg)
}

func newToolsListBody(t *testing.T, id int64) []byte {
	t.Helper()
	rpcID := jsonrpc.NewNumberID(id)
	msg := jsonrpc.Message{
		JSONRPC: "2.0",
		ID:      &rpcID,
		Method:  "tools/list",
	}
	return mustMarshal(t, msg)
}

func TestRoundTripInitializeAndToolsList(t *testing.T) {
	tr, stop := startTestServer(t)
	defer stop()

	url := "http://" + tr.Addr() + "/"

	// initialize: expect a session ID in the response header.
	resp, err := http.Post(url, "application/json", bytes.NewReader(newInitializeBody(t, 1)))
	if err != nil {
		t.Fatalf("initialize POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initialize status = %d, want 200", resp.StatusCode)
	}
	sessionID := resp.Header.Get(transport.HeaderMCPSessionID)
	if sessionID == "" {
		t.Fatal("expected Mcp-Session-Id header on initialize response")
	}

	var initResp jsonrpc.Message
	if err := json.NewDecoder(resp.Body).Decode(&initResp); err != nil {
		t.Fatalf("decode initialize response: %v", err)
	}
	if initResp.Error != nil {
		t.Fatalf("initialize error: %v", initResp.Error)
	}

	// tools/list with the session ID echoed back.
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(newToolsListBody(t, 2)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(transport.HeaderMCPSessionID, sessionID)

	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("tools/list POST: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp2.Body)
		t.Fatalf("tools/list status = %d (body=%q), want 200", resp2.StatusCode, body)
	}

	var listResp jsonrpc.Message
	if err := json.NewDecoder(resp2.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode tools/list response: %v", err)
	}
	if listResp.Error != nil {
		t.Fatalf("tools/list error: %v", listResp.Error)
	}
	var result protocol.ToolsListResult
	if err := json.Unmarshal(listResp.Result, &result); err != nil {
		t.Fatalf("unmarshal tools/list result: %v", err)
	}
	if len(result.Tools) != 1 || result.Tools[0].Name != "echo" {
		t.Errorf("tools = %+v, want one entry named 'echo'", result.Tools)
	}
}

func TestInvalidSessionIDIsRejected(t *testing.T) {
	tr, stop := startTestServer(t)
	defer stop()

	url := "http://" + tr.Addr() + "/"

	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(newToolsListBody(t, 1)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(transport.HeaderMCPSessionID, "this-session-does-not-exist")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for invalid session", resp.StatusCode)
	}
}

func TestDeleteTerminatesSession(t *testing.T) {
	tr, stop := startTestServer(t)
	defer stop()

	url := "http://" + tr.Addr() + "/"

	// initialize to get a session.
	initResp, err := http.Post(url, "application/json", bytes.NewReader(newInitializeBody(t, 1)))
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	initResp.Body.Close()
	sessionID := initResp.Header.Get(transport.HeaderMCPSessionID)
	if sessionID == "" {
		t.Fatal("no session ID")
	}

	// DELETE the session.
	delReq, _ := http.NewRequest(http.MethodDelete, url, nil)
	delReq.Header.Set(transport.HeaderMCPSessionID, sessionID)
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	delResp.Body.Close()
	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE status = %d, want 200", delResp.StatusCode)
	}

	// Subsequent POST with the now-defunct session ID should be rejected.
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(newToolsListBody(t, 2)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(transport.HeaderMCPSessionID, sessionID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post-delete POST: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("post-delete status = %d, want 400", resp.StatusCode)
	}

	// DELETE-ing again should report Not Found.
	delReq2, _ := http.NewRequest(http.MethodDelete, url, nil)
	delReq2.Header.Set(transport.HeaderMCPSessionID, sessionID)
	delResp2, err := http.DefaultClient.Do(delReq2)
	if err != nil {
		t.Fatalf("DELETE again: %v", err)
	}
	delResp2.Body.Close()
	if delResp2.StatusCode != http.StatusNotFound {
		t.Errorf("second-DELETE status = %d, want 404", delResp2.StatusCode)
	}
}

func TestOriginValidation(t *testing.T) {
	tr, stop := startTestServer(t, streamablehttp.WithAllowedOrigins("https://allowed.example"))
	defer stop()

	url := "http://" + tr.Addr() + "/"

	// Disallowed origin → 403.
	badReq, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(newInitializeBody(t, 1)))
	badReq.Header.Set("Content-Type", "application/json")
	badReq.Header.Set("Origin", "https://attacker.example")
	badResp, err := http.DefaultClient.Do(badReq)
	if err != nil {
		t.Fatalf("bad-origin POST: %v", err)
	}
	badResp.Body.Close()
	if badResp.StatusCode != http.StatusForbidden {
		t.Errorf("disallowed-origin status = %d, want 403", badResp.StatusCode)
	}

	// Allowed origin → 200.
	goodReq, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(newInitializeBody(t, 2)))
	goodReq.Header.Set("Content-Type", "application/json")
	goodReq.Header.Set("Origin", "https://allowed.example")
	goodResp, err := http.DefaultClient.Do(goodReq)
	if err != nil {
		t.Fatalf("good-origin POST: %v", err)
	}
	goodResp.Body.Close()
	if goodResp.StatusCode != http.StatusOK {
		t.Errorf("allowed-origin status = %d, want 200", goodResp.StatusCode)
	}

	// No Origin header → also allowed (same-origin requests don't send Origin).
	bareReq, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(newInitializeBody(t, 3)))
	bareReq.Header.Set("Content-Type", "application/json")
	bareResp, err := http.DefaultClient.Do(bareReq)
	if err != nil {
		t.Fatalf("no-origin POST: %v", err)
	}
	bareResp.Body.Close()
	if bareResp.StatusCode != http.StatusOK {
		t.Errorf("no-origin status = %d, want 200", bareResp.StatusCode)
	}
}

func TestSSEResponseFormat(t *testing.T) {
	tr, stop := startTestServer(t)
	defer stop()

	url := "http://" + tr.Addr() + "/"

	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(newInitializeBody(t, 1)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	// Read the first SSE event and confirm it contains a JSON-RPC payload.
	scanner := bufio.NewScanner(resp.Body)
	var sawData bool
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			payload := strings.TrimPrefix(line, "data: ")
			var msg jsonrpc.Message
			if err := json.Unmarshal([]byte(payload), &msg); err != nil {
				t.Fatalf("SSE payload not valid JSON-RPC: %v (payload=%q)", err, payload)
			}
			if msg.Error != nil {
				t.Fatalf("initialize error in SSE: %v", msg.Error)
			}
			sawData = true
			break
		}
	}
	if !sawData {
		t.Fatal("never observed a `data: ` line in SSE response")
	}
}
