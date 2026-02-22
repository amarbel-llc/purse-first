package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
)

// StreamableHTTP implements the MCP Streamable HTTP transport.
// It uses HTTP POST for requests and streaming responses.
type StreamableHTTP struct {
	addr      string
	server    *http.Server
	requests  chan *jsonrpc.Message
	responses map[string]chan *jsonrpc.Message
	mu        sync.RWMutex
	closed    bool
}

func NewStreamableHTTP(addr string) *StreamableHTTP {
	return &StreamableHTTP{
		addr:      addr,
		requests:  make(chan *jsonrpc.Message, 100),
		responses: make(map[string]chan *jsonrpc.Message),
	}
}

func (t *StreamableHTTP) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", t.handleMCP)

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

func (t *StreamableHTTP) handleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check for streaming support
	accept := r.Header.Get("Accept")
	streaming := accept == "text/event-stream" || accept == "application/x-ndjson"

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

	// For notifications, just accept them
	if msg.IsNotification() {
		t.requests <- &msg
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// For requests, set up response channel
	respChan := make(chan *jsonrpc.Message, 1)
	requestID := msg.ID.String()

	t.mu.Lock()
	t.responses[requestID] = respChan
	t.mu.Unlock()

	defer func() {
		t.mu.Lock()
		delete(t.responses, requestID)
		t.mu.Unlock()
	}()

	// Queue the request
	t.requests <- &msg

	// Wait for response
	select {
	case resp := <-respChan:
		if streaming {
			w.Header().Set("Content-Type", "application/x-ndjson")
			flusher, _ := w.(http.Flusher)
			data, _ := json.Marshal(resp)
			w.Write(data)
			w.Write([]byte("\n"))
			if flusher != nil {
				flusher.Flush()
			}
		} else {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}
	case <-r.Context().Done():
		http.Error(w, "Request timeout", http.StatusRequestTimeout)
	}
}

func (t *StreamableHTTP) Read() (*jsonrpc.Message, error) {
	msg, ok := <-t.requests
	if !ok {
		return nil, io.EOF
	}
	return msg, nil
}

func (t *StreamableHTTP) Write(msg *jsonrpc.Message) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed {
		return fmt.Errorf("transport closed")
	}

	// Route response to waiting request
	if msg.ID != nil {
		requestID := msg.ID.String()
		if ch, ok := t.responses[requestID]; ok {
			ch <- msg
			return nil
		}
	}

	// For notifications from server, we'd need SSE or similar
	// For now, just log that we can't send unsolicited messages
	return nil
}

func (t *StreamableHTTP) Close() error {
	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()

	close(t.requests)

	if t.server != nil {
		return t.server.Shutdown(context.Background())
	}
	return nil
}

// StreamableHTTPClient connects to an MCP server via Streamable HTTP
type StreamableHTTPClient struct {
	endpoint   string
	httpClient *http.Client
	responses  chan *jsonrpc.Message
	mu         sync.Mutex
	closed     bool
}

func NewStreamableHTTPClient(endpoint string) *StreamableHTTPClient {
	return &StreamableHTTPClient{
		endpoint:   endpoint,
		httpClient: &http.Client{},
		responses:  make(chan *jsonrpc.Message, 100),
	}
}

func (c *StreamableHTTPClient) Read() (*jsonrpc.Message, error) {
	msg, ok := <-c.responses
	if !ok {
		return nil, io.EOF
	}
	return msg, nil
}

func (c *StreamableHTTPClient) Write(msg *jsonrpc.Message) error {
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()

	if closed {
		return fmt.Errorf("transport closed")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", c.endpoint, bufio.NewReader(
		&readerFromBytes{data: data},
	))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// For notifications, no response expected
	if msg.IsNotification() {
		return nil
	}

	var response jsonrpc.Message
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return err
	}

	c.responses <- &response
	return nil
}

func (c *StreamableHTTPClient) Close() error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()

	close(c.responses)
	return nil
}

type readerFromBytes struct {
	data []byte
	pos  int
}

func (r *readerFromBytes) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
