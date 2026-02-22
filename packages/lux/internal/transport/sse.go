package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
)

// DocumentLifecycle allows the SSE transport to delegate document open/close
// requests to the MCP server's DocumentManager without a circular dependency.
type DocumentLifecycle interface {
	OpenURI(ctx context.Context, uri string) error
	CloseURI(uri string) error
	CloseAllDocs()
}

type SSE struct {
	addr     string
	server   *http.Server
	messages chan *jsonrpc.Message
	writers  map[string]http.ResponseWriter
	docMgr   DocumentLifecycle
	mu       sync.RWMutex
	closed   bool
}

func NewSSE(addr string) *SSE {
	return &SSE{
		addr:     addr,
		messages: make(chan *jsonrpc.Message, 100),
		writers:  make(map[string]http.ResponseWriter),
	}
}

func (t *SSE) SetDocumentLifecycle(dl DocumentLifecycle) {
	t.docMgr = dl
}

func (t *SSE) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/sse", t.handleSSE)
	mux.HandleFunc("/message", t.handleMessage)
	mux.HandleFunc("/documents/open", t.handleDocumentOpen)
	mux.HandleFunc("/documents/close", t.handleDocumentClose)
	mux.HandleFunc("/documents/close-all", t.handleDocumentCloseAll)

	t.server = &http.Server{
		Addr:    t.addr,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		t.server.Shutdown(context.Background())
	}()

	return t.server.ListenAndServe()
}

func (t *SSE) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	sessionID := r.URL.Query().Get("session")
	if sessionID == "" {
		sessionID = fmt.Sprintf("%d", len(t.writers)+1)
	}

	t.mu.Lock()
	t.writers[sessionID] = w
	t.mu.Unlock()

	defer func() {
		t.mu.Lock()
		delete(t.writers, sessionID)
		t.mu.Unlock()
	}()

	// Send endpoint event
	fmt.Fprintf(w, "event: endpoint\ndata: /message?session=%s\n\n", sessionID)
	flusher.Flush()

	// Keep connection open
	<-r.Context().Done()
}

func (t *SSE) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	var msg jsonrpc.Message
	if err := json.Unmarshal(body, &msg); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	t.messages <- &msg

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"accepted"}`))
}

func (t *SSE) Read() (*jsonrpc.Message, error) {
	msg, ok := <-t.messages
	if !ok {
		return nil, io.EOF
	}
	return msg, nil
}

func (t *SSE) Write(msg *jsonrpc.Message) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed {
		return fmt.Errorf("transport closed")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Write to all connected SSE clients
	for _, w := range t.writers {
		fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	return nil
}

func (t *SSE) Close() error {
	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()

	close(t.messages)

	if t.server != nil {
		return t.server.Shutdown(context.Background())
	}
	return nil
}

func (t *SSE) handleDocumentOpen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if t.docMgr == nil {
		http.Error(w, "Document manager not configured", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		URI string `json:"uri"`
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	if err := json.Unmarshal(body, &req); err != nil || req.URI == "" {
		http.Error(w, "Invalid request: requires {\"uri\": \"file:///...\"}", http.StatusBadRequest)
		return
	}

	if err := t.docMgr.OpenURI(r.Context(), req.URI); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"opened"}`))
}

func (t *SSE) handleDocumentClose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if t.docMgr == nil {
		http.Error(w, "Document manager not configured", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		URI string `json:"uri"`
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	if err := json.Unmarshal(body, &req); err != nil || req.URI == "" {
		http.Error(w, "Invalid request: requires {\"uri\": \"file:///...\"}", http.StatusBadRequest)
		return
	}

	if err := t.docMgr.CloseURI(req.URI); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"closed"}`))
}

func (t *SSE) handleDocumentCloseAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if t.docMgr == nil {
		http.Error(w, "Document manager not configured", http.StatusServiceUnavailable)
		return
	}

	t.docMgr.CloseAllDocs()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"closed_all"}`))
}

// SSEClient is for connecting to an SSE MCP server (client-side)
type SSEClient struct {
	endpoint   string
	sseURL     string
	messages   chan *jsonrpc.Message
	httpClient *http.Client
	cancel     context.CancelFunc
	mu         sync.Mutex
	closed     bool
}

func NewSSEClient(sseURL string) *SSEClient {
	return &SSEClient{
		sseURL:     sseURL,
		messages:   make(chan *jsonrpc.Message, 100),
		httpClient: &http.Client{},
	}
}

func (c *SSEClient) Connect(ctx context.Context) error {
	ctx, c.cancel = context.WithCancel(ctx)

	req, err := http.NewRequestWithContext(ctx, "GET", c.sseURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	go c.readSSE(ctx, resp.Body)
	return nil
}

func (c *SSEClient) readSSE(ctx context.Context, body io.ReadCloser) {
	defer body.Close()

	scanner := bufio.NewScanner(body)
	var eventType, data string

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			// End of event
			if eventType == "endpoint" {
				c.mu.Lock()
				c.endpoint = strings.TrimPrefix(data, "/message")
				// Build full URL from sseURL base
				if strings.HasPrefix(c.sseURL, "http") {
					base := c.sseURL[:strings.LastIndex(c.sseURL, "/")]
					c.endpoint = base + data
				}
				c.mu.Unlock()
			} else if eventType == "message" {
				var msg jsonrpc.Message
				if err := json.Unmarshal([]byte(data), &msg); err == nil {
					c.messages <- &msg
				}
			}
			eventType = ""
			data = ""
			continue
		}

		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}
}

func (c *SSEClient) Read() (*jsonrpc.Message, error) {
	msg, ok := <-c.messages
	if !ok {
		return nil, io.EOF
	}
	return msg, nil
}

func (c *SSEClient) Write(msg *jsonrpc.Message) error {
	c.mu.Lock()
	endpoint := c.endpoint
	closed := c.closed
	c.mu.Unlock()

	if closed {
		return fmt.Errorf("transport closed")
	}

	if endpoint == "" {
		return fmt.Errorf("no endpoint received yet")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Post(endpoint, "application/json", strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}

func (c *SSEClient) Close() error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}
	close(c.messages)
	return nil
}
