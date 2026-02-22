package transport_test

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

	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
	intTransport "github.com/friedenberg/grit/internal/transport"
)

// startSSE creates and starts an SSE transport on an ephemeral port.
func startSSE(t *testing.T) *intTransport.SSE {
	t.Helper()
	s := intTransport.NewSSE(":0")
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("starting SSE: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func baseURL(s *intTransport.SSE) string {
	return fmt.Sprintf("http://%s", s.Addr().String())
}

// connectSSE opens the SSE stream and returns the endpoint data and response.
func connectSSE(t *testing.T, s *intTransport.SSE) (endpoint string, resp *http.Response) {
	t.Helper()
	resp, err := http.Get(baseURL(s) + "/sse")
	if err != nil {
		t.Fatalf("GET /sse: %v", err)
	}

	scanner := bufio.NewScanner(resp.Body)
	var eventType, data string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			data = strings.TrimPrefix(line, "data: ")
		} else if line == "" && eventType != "" {
			break
		}
	}

	if eventType != "endpoint" {
		t.Fatalf("expected endpoint event, got %q", eventType)
	}

	return data, resp
}

func postMessage(t *testing.T, s *intTransport.SSE, endpoint string, msg *jsonrpc.Message) *http.Response {
	t.Helper()
	body, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshaling message: %v", err)
	}

	resp, err := http.Post(
		baseURL(s)+endpoint,
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("POST /message: %v", err)
	}

	return resp
}

func TestRoundTrip(t *testing.T) {
	s := startSSE(t)

	var endpoint string
	var sseResp *http.Response
	done := make(chan struct{})

	go func() {
		defer close(done)
		var r *http.Response
		endpoint, r = connectSSE(t, s)
		sseResp = r
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout connecting SSE")
	}
	defer sseResp.Body.Close()

	// POST a request
	reqMsg, _ := jsonrpc.NewRequest(jsonrpc.NewNumberID(1), "test/method", map[string]string{"key": "value"})
	resp := postMessage(t, s, endpoint, reqMsg)
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	// Read should return the posted message
	readDone := make(chan struct{})
	var readMsg *jsonrpc.Message
	var readErr error
	go func() {
		defer close(readDone)
		readMsg, readErr = s.Read()
	}()

	select {
	case <-readDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout reading message")
	}

	if readErr != nil {
		t.Fatalf("Read: %v", readErr)
	}
	if readMsg.Method != "test/method" {
		t.Fatalf("expected method test/method, got %s", readMsg.Method)
	}

	// Write a response and verify it appears on the SSE stream
	respMsg, _ := jsonrpc.NewResponse(jsonrpc.NewNumberID(1), map[string]string{"result": "ok"})
	if err := s.Write(respMsg); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Read the SSE event from the stream
	scanner := bufio.NewScanner(sseResp.Body)
	var eventType, data string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			data = strings.TrimPrefix(line, "data: ")
		} else if line == "" && eventType == "message" {
			break
		}
	}

	if eventType != "message" {
		t.Fatalf("expected message event, got %q", eventType)
	}

	var gotMsg jsonrpc.Message
	if err := json.Unmarshal([]byte(data), &gotMsg); err != nil {
		t.Fatalf("parsing SSE data: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(gotMsg.Result, &result); err != nil {
		t.Fatalf("parsing result: %v", err)
	}
	if result["result"] != "ok" {
		t.Fatalf("expected result ok, got %s", result["result"])
	}
}

func TestPostBeforeSSE(t *testing.T) {
	s := startSSE(t)

	msg, _ := jsonrpc.NewRequest(jsonrpc.NewNumberID(1), "test", nil)
	body, _ := json.Marshal(msg)
	resp, err := http.Post(
		baseURL(s)+"/message?sessionId=fake",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}
}

func TestWrongSessionID(t *testing.T) {
	s := startSSE(t)

	done := make(chan struct{})
	var sseResp *http.Response
	go func() {
		defer close(done)
		_, sseResp = connectSSE(t, s)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout connecting SSE")
	}
	defer sseResp.Body.Close()

	msg, _ := jsonrpc.NewRequest(jsonrpc.NewNumberID(1), "test", nil)
	body, _ := json.Marshal(msg)
	resp, err := http.Post(
		baseURL(s)+"/message?sessionId=wrong-id",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestClientDisconnectCausesEOF(t *testing.T) {
	s := startSSE(t)

	done := make(chan struct{})
	var sseResp *http.Response
	go func() {
		defer close(done)
		_, sseResp = connectSSE(t, s)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout connecting SSE")
	}

	// Close the SSE response body to simulate client disconnect.
	sseResp.Body.Close()

	// Give the server a moment to detect the disconnect.
	time.Sleep(100 * time.Millisecond)

	readDone := make(chan struct{})
	var readErr error
	go func() {
		defer close(readDone)
		_, readErr = s.Read()
	}()

	select {
	case <-readDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for Read to return EOF")
	}

	if readErr != io.EOF {
		t.Fatalf("expected io.EOF, got %v", readErr)
	}
}

func TestMultipleSSEConnections(t *testing.T) {
	s := startSSE(t)

	done := make(chan struct{})
	var sseResp *http.Response
	go func() {
		defer close(done)
		_, sseResp = connectSSE(t, s)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout connecting SSE")
	}
	defer sseResp.Body.Close()

	// Second connection should get 409.
	resp, err := http.Get(baseURL(s) + "/sse")
	if err != nil {
		t.Fatalf("GET /sse: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
}

func TestConcurrentWrites(t *testing.T) {
	s := startSSE(t)

	done := make(chan struct{})
	var sseResp *http.Response
	go func() {
		defer close(done)
		_, sseResp = connectSSE(t, s)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout connecting SSE")
	}
	defer sseResp.Body.Close()

	// Fire off concurrent writes — should not panic or produce errors.
	var wg sync.WaitGroup
	errs := make(chan error, 10)

	for i := range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			msg, _ := jsonrpc.NewResponse(jsonrpc.NewNumberID(int64(i)), map[string]int{"n": i})
			if err := s.Write(msg); err != nil {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent write error: %v", err)
	}
}

func TestMalformedJSON(t *testing.T) {
	s := startSSE(t)

	done := make(chan struct{})
	var endpoint string
	var sseResp *http.Response
	go func() {
		defer close(done)
		endpoint, sseResp = connectSSE(t, s)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout connecting SSE")
	}
	defer sseResp.Body.Close()

	// Extract session ID from endpoint
	parts := strings.SplitN(endpoint, "?sessionId=", 2)
	if len(parts) != 2 {
		t.Fatalf("unexpected endpoint format: %s", endpoint)
	}

	resp, err := http.Post(
		baseURL(s)+endpoint,
		"application/json",
		strings.NewReader("{not valid json"),
	)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}
