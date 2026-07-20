package streamablehttp_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"code.linenisgreat.com/purse-first/libs/go-mcp/jsonrpc"
	"code.linenisgreat.com/purse-first/libs/go-mcp/protocol"
	"code.linenisgreat.com/purse-first/libs/go-mcp/server"
	"code.linenisgreat.com/purse-first/libs/go-mcp/transport"
	"code.linenisgreat.com/purse-first/libs/go-mcp/transport/streamablehttp"
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

// blockingToolProvider supports a single "slow" tool whose CallTool blocks
// on a release channel after signaling via started. Used to test cleanup
// while a request is in-flight on the server side.
type blockingToolProvider struct {
	started chan struct{}
	release chan struct{}
}

func (s *blockingToolProvider) ListTools(ctx context.Context) ([]protocol.Tool, error) {
	return []protocol.Tool{
		{Name: "slow", Description: "Blocks", InputSchema: json.RawMessage(`{"type":"object"}`)},
	}, nil
}

func (s *blockingToolProvider) CallTool(ctx context.Context, name string, args json.RawMessage) (*protocol.ToolCallResult, error) {
	close(s.started)
	select {
	case <-s.release:
		return &protocol.ToolCallResult{
			Content: []protocol.ContentBlock{protocol.TextContent("done")},
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// startTestServer brings up a StreamableHTTP transport + server.Server with
// the default echo tool provider, runs the server in a goroutine, and
// returns the transport plus a stop function.
func startTestServer(t *testing.T, opts ...streamablehttp.Option) (*streamablehttp.StreamableHTTP, func()) {
	t.Helper()
	return startTestServerWithProvider(t, &stubToolProvider{}, opts...)
}

// startTestServerWithProvider is the parameterizable variant — pass an
// explicit ToolProvider when a test needs to control tool-call behavior.
func startTestServerWithProvider(t *testing.T, tools server.ToolProvider, opts ...streamablehttp.Option) (*streamablehttp.StreamableHTTP, func()) {
	t.Helper()

	tr := streamablehttp.New("127.0.0.1:0", opts...)

	srv, err := server.New(tr, server.Options{
		ServerName:    "test",
		ServerVersion: "1.0",
		Tools:         tools,
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

	select {
	case <-tr.Ready():
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatalf("transport never reported ready")
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

func TestProtocolVersionMismatchRejected(t *testing.T) {
	tr, stop := startTestServer(t)
	defer stop()

	url := "http://" + tr.Addr() + "/"

	// Initialize to bind a session with a negotiated protocol version.
	initResp, err := http.Post(url, "application/json", bytes.NewReader(newInitializeBody(t, 1)))
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	initResp.Body.Close()
	sessionID := initResp.Header.Get(transport.HeaderMCPSessionID)
	if sessionID == "" {
		t.Fatal("no session ID")
	}

	// tools/list with a Mcp-Protocol-Version header that doesn't match the
	// version the session negotiated → 400.
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(newToolsListBody(t, 2)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(transport.HeaderMCPSessionID, sessionID)
	req.Header.Set(transport.HeaderMCPProtocolVersion, "not-a-real-version")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for protocol version mismatch", resp.StatusCode)
	}
}

func TestListenFailureReturnsError(t *testing.T) {
	// Bind one transport so a specific port is in use.
	tr1, stop := startTestServer(t)
	defer stop()

	// Try to start a second transport on the exact same address. net.Listen
	// should fail; Start should surface a wrapped error rather than panic
	// or silently succeed.
	tr2 := streamablehttp.New(tr1.Addr())
	err := tr2.Start(context.Background())
	if err == nil {
		_ = tr2.Close()
		t.Fatal("expected error binding to occupied port, got nil")
	}
	if !strings.Contains(err.Error(), "listening on") {
		t.Errorf("error does not mention listening: %v", err)
	}
}

func TestTransportCloseWithInflightRequest(t *testing.T) {
	provider := &blockingToolProvider{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	// Release the blocked tool on the way out so srv.Run's gracefulShutdown
	// can drain the in-flight handler goroutine.
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(provider.release) }) }
	defer release()

	tr, stop := startTestServerWithProvider(t, provider)
	defer stop()

	url := "http://" + tr.Addr() + "/"
	initResp, err := http.Post(url, "application/json", bytes.NewReader(newInitializeBody(t, 1)))
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	initResp.Body.Close()
	sessionID := initResp.Header.Get(transport.HeaderMCPSessionID)
	if sessionID == "" {
		t.Fatal("no session ID")
	}

	// Issue tools/call from a client goroutine; it will park inside the
	// blocking tool provider on the server side.
	callBody := mustMarshal(t, jsonrpc.Message{
		JSONRPC: "2.0",
		ID:      idPtr(jsonrpc.NewNumberID(2)),
		Method:  "tools/call",
		Params:  mustMarshal(t, protocol.ToolCallParams{Name: "slow"}),
	})

	done := make(chan error, 1)
	go func() {
		req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(callBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(transport.HeaderMCPSessionID, sessionID)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			done <- err
			return
		}
		resp.Body.Close()
		// Reaching here means the server actually responded. For this test,
		// any concrete status counts as "request finished without error";
		// the goroutine pushes nil and the test interprets that as "got a
		// real response," which the assertion below also accepts.
		done <- nil
	}()

	// Wait for the call to enter the blocking tool. Avoids a race where
	// Close fires before the request lands on the server side.
	select {
	case <-provider.started:
	case <-time.After(2 * time.Second):
		t.Fatal("tool call never started")
	}

	// Close the transport mid-flight. The client connection should tear
	// down; the in-flight handler returns early via r.Context().Done().
	if err := tr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Either the client errors out (connection closed) OR the server
	// happened to write a response before the close took effect. The
	// failure mode we're guarding against is the client hanging forever.
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("client request never completed after transport close")
	}

	// Let the blocked tool exit so gracefulShutdown can finish.
	release()
}

func TestConcurrentRequestsOnSameSession(t *testing.T) {
	tr, stop := startTestServer(t)
	defer stop()

	url := "http://" + tr.Addr() + "/"

	initResp, err := http.Post(url, "application/json", bytes.NewReader(newInitializeBody(t, 1)))
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	initResp.Body.Close()
	sessionID := initResp.Header.Get(transport.HeaderMCPSessionID)
	if sessionID == "" {
		t.Fatal("no session ID")
	}

	const N = 20
	var wg sync.WaitGroup
	errs := make(chan error, N)

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Each request needs a unique JSON-RPC ID so the pending-map
			// keys don't collide.
			body := newToolsListBody(t, int64(100+id))
			req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(transport.HeaderMCPSessionID, sessionID)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				errs <- fmt.Errorf("request %d: %w", id, err)
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				errs <- fmt.Errorf("request %d: status %d", id, resp.StatusCode)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func idPtr(id jsonrpc.ID) *jsonrpc.ID {
	return &id
}
