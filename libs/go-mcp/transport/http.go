package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
)

// StreamableHTTP implements the MCP Streamable HTTP transport.
//
// The transport exposes a single HTTP endpoint that accepts POST requests
// containing JSON-RPC messages and returns responses either as JSON or as
// SSE streams. Session management is handled via the Mcp-Session-Id header.
type StreamableHTTP struct {
	addr     string
	listener net.Listener
	server   *http.Server
	sessions *sessionManager

	// incoming receives messages from HTTP clients for the server loop.
	incoming chan *jsonrpc.Message

	// pending tracks outstanding requests awaiting responses.
	pending   map[string]chan *jsonrpc.Message
	pendingMu sync.Mutex

	// closed signals transport shutdown.
	closed chan struct{}

	// allowedOrigins restricts which Origin headers are accepted.
	// Empty means all origins are allowed.
	allowedOrigins map[string]bool
}

// StreamableHTTPOption configures a StreamableHTTP transport.
type StreamableHTTPOption func(*StreamableHTTP)

// WithAllowedOrigins restricts accepted Origin headers for DNS rebinding protection.
func WithAllowedOrigins(origins ...string) StreamableHTTPOption {
	return func(t *StreamableHTTP) {
		for _, o := range origins {
			t.allowedOrigins[o] = true
		}
	}
}

// NewStreamableHTTP creates a new Streamable HTTP transport.
// The addr is the listen address (e.g., "127.0.0.1:8080" or ":0" for random port).
func NewStreamableHTTP(addr string, opts ...StreamableHTTPOption) *StreamableHTTP {
	t := &StreamableHTTP{
		addr:           addr,
		sessions:       newSessionManager(),
		incoming:       make(chan *jsonrpc.Message, 64),
		pending:        make(map[string]chan *jsonrpc.Message),
		closed:         make(chan struct{}),
		allowedOrigins: make(map[string]bool),
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

// Start starts the HTTP server. Implements LifecycleTransport.
func (t *StreamableHTTP) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", t.addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", t.addr, err)
	}
	t.listener = ln

	mux := http.NewServeMux()
	mux.HandleFunc("/", t.handleMCP)

	t.server = &http.Server{Handler: mux}

	go func() {
		<-ctx.Done()
		t.server.Close()
	}()

	go t.server.Serve(ln)

	return nil
}

// Addr returns the listen address. Implements LifecycleTransport.
func (t *StreamableHTTP) Addr() string {
	if t.listener != nil {
		return t.listener.Addr().String()
	}
	return t.addr
}

// Read reads the next incoming message. Implements Transport.
func (t *StreamableHTTP) Read() (*jsonrpc.Message, error) {
	select {
	case msg := <-t.incoming:
		return msg, nil
	case <-t.closed:
		return nil, io.EOF
	}
}

// Write sends a response message back to the waiting client. Implements Transport.
func (t *StreamableHTTP) Write(msg *jsonrpc.Message) error {
	if msg == nil || msg.ID == nil {
		return nil
	}

	key := msg.ID.String()
	t.pendingMu.Lock()
	ch, ok := t.pending[key]
	if ok {
		delete(t.pending, key)
	}
	t.pendingMu.Unlock()

	if ok {
		ch <- msg
	}

	return nil
}

// Close shuts down the transport. Implements Transport.
func (t *StreamableHTTP) Close() error {
	select {
	case <-t.closed:
	default:
		close(t.closed)
	}

	if t.server != nil {
		return t.server.Close()
	}
	return nil
}

// handleMCP is the HTTP handler for the MCP endpoint.
func (t *StreamableHTTP) handleMCP(w http.ResponseWriter, r *http.Request) {
	// Validate Origin header for DNS rebinding protection.
	if !t.validateOrigin(r) {
		http.Error(w, "Forbidden: invalid origin", http.StatusForbidden)
		return
	}

	switch r.Method {
	case http.MethodPost:
		t.handlePost(w, r)
	case http.MethodGet:
		t.handleGet(w, r)
	case http.MethodDelete:
		t.handleDelete(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePost processes incoming JSON-RPC messages via POST.
func (t *StreamableHTTP) handlePost(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var msg jsonrpc.Message
	if err := json.Unmarshal(body, &msg); err != nil {
		http.Error(w, "Invalid JSON-RPC message", http.StatusBadRequest)
		return
	}

	// For non-initialize requests, validate session ID and protocol version header.
	if msg.Method != "initialize" {
		sessionID := r.Header.Get(HeaderMCPSessionID)
		if sessionID != "" {
			if !t.sessions.valid(sessionID) {
				http.Error(w, "Invalid session", http.StatusBadRequest)
				return
			}

			// Validate MCP-Protocol-Version header matches negotiated version.
			clientPV := r.Header.Get(HeaderMCPProtocolVersion)
			sessionPV := t.sessions.protocolVersion(sessionID)
			if clientPV != "" && sessionPV != "" && clientPV != sessionPV {
				http.Error(w, "Protocol version mismatch", http.StatusBadRequest)
				return
			}
		}
	}

	// Notifications and responses: accept and return 202.
	if msg.IsNotification() || msg.IsResponse() {
		t.incoming <- &msg
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Requests: register a pending response channel, forward to server, wait for response.
	respCh := make(chan *jsonrpc.Message, 1)
	key := msg.ID.String()

	t.pendingMu.Lock()
	t.pending[key] = respCh
	t.pendingMu.Unlock()

	t.incoming <- &msg

	// Wait for the response from the server loop.
	resp := <-respCh

	// For initialize responses, assign a session ID and extract protocol version.
	if msg.Method == "initialize" {
		var pv string
		if len(resp.Result) > 0 {
			var initResult struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
			if err := json.Unmarshal(resp.Result, &initResult); err == nil {
				pv = initResult.ProtocolVersion
			}
		}

		sessionID, err := t.sessions.create(pv)
		if err == nil {
			w.Header().Set(HeaderMCPSessionID, sessionID)
		}
	}

	// Check if client accepts SSE.
	accept := r.Header.Get("Accept")
	useSSE := strings.Contains(accept, "text/event-stream")

	if useSSE {
		sse := newSSEWriter(w)
		if sse != nil {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.WriteHeader(http.StatusOK)
			sse.writeMessage(resp)
			return
		}
	}

	// Fall back to plain JSON response.
	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// handleGet opens an SSE stream for server-to-client messages.
func (t *StreamableHTTP) handleGet(w http.ResponseWriter, r *http.Request) {
	accept := r.Header.Get("Accept")
	if !strings.Contains(accept, "text/event-stream") {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.Header.Get(HeaderMCPSessionID)
	if sessionID != "" && !t.sessions.valid(sessionID) {
		http.Error(w, "Invalid session", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	// Keep connection open until client disconnects.
	<-r.Context().Done()
}

// handleDelete terminates a session.
func (t *StreamableHTTP) handleDelete(w http.ResponseWriter, r *http.Request) {
	sessionID := r.Header.Get(HeaderMCPSessionID)
	if sessionID == "" {
		http.Error(w, "Missing session ID", http.StatusBadRequest)
		return
	}

	if !t.sessions.valid(sessionID) {
		http.Error(w, "Invalid session", http.StatusNotFound)
		return
	}

	t.sessions.remove(sessionID)
	w.WriteHeader(http.StatusOK)
}

// validateOrigin checks the Origin header against allowed origins.
func (t *StreamableHTTP) validateOrigin(r *http.Request) bool {
	if len(t.allowedOrigins) == 0 {
		return true
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		// No Origin header — allow (same-origin requests don't send Origin).
		return true
	}

	return t.allowedOrigins[origin]
}
