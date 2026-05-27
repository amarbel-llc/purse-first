// Package streamablehttp implements the MCP Streamable HTTP server transport.
//
// Separated from the core transport package so plugin binaries that only need
// stdio don't transitively link net/http.
package streamablehttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/transport"
)

// Compile-time assertion that *StreamableHTTP satisfies transport.LifecycleTransport.
var _ transport.LifecycleTransport = (*StreamableHTTP)(nil)

// Server timeouts and body-size limit. Values mitigate slow-loris and
// OOM-via-large-POST without being so tight that typical MCP traffic
// breaks. Tune via WithReadTimeout/WithWriteTimeout/etc. if the defaults
// don't fit your deployment (no overrides exposed yet — file an issue).
const (
	defaultReadHeaderTimeout = 10 * time.Second
	defaultReadTimeout       = 30 * time.Second
	defaultWriteTimeout      = 30 * time.Second
	defaultIdleTimeout       = 120 * time.Second
	defaultMaxBodyBytes      = 10 << 20 // 10 MiB
)

// StreamableHTTP implements the MCP Streamable HTTP transport.
//
// The transport exposes a single HTTP endpoint that accepts POST requests
// containing JSON-RPC messages and returns responses either as JSON or as
// SSE streams. Session management is handled via the Mcp-Session-Id header.
type StreamableHTTP struct {
	addr string

	// lifecycleMu guards listener and server, which are written by Start and
	// read by Addr/Close concurrently.
	lifecycleMu sync.RWMutex
	listener    net.Listener
	server      *http.Server

	sessions *sessionManager

	// incoming receives messages from HTTP clients for the server loop.
	incoming chan *jsonrpc.Message

	// pending tracks outstanding requests awaiting responses.
	pending   map[string]chan *jsonrpc.Message
	pendingMu sync.Mutex

	// closed signals transport shutdown.
	closed chan struct{}

	// ready is closed by Start once the listener is bound, so callers (notably
	// tests) can wait deterministically rather than polling Addr().
	ready chan struct{}

	// allowedOrigins restricts which Origin headers are accepted.
	// Empty means all origins are allowed.
	allowedOrigins map[string]bool
}

// Option configures a StreamableHTTP transport.
type Option func(*StreamableHTTP)

// WithAllowedOrigins restricts accepted Origin headers for DNS rebinding protection.
func WithAllowedOrigins(origins ...string) Option {
	return func(t *StreamableHTTP) {
		for _, o := range origins {
			t.allowedOrigins[o] = true
		}
	}
}

// New creates a new Streamable HTTP transport.
// The addr is the listen address (e.g., "127.0.0.1:8080" or ":0" for random port).
func New(addr string, opts ...Option) *StreamableHTTP {
	t := &StreamableHTTP{
		addr:           addr,
		sessions:       newSessionManager(),
		incoming:       make(chan *jsonrpc.Message, 64),
		pending:        make(map[string]chan *jsonrpc.Message),
		closed:         make(chan struct{}),
		ready:          make(chan struct{}),
		allowedOrigins: make(map[string]bool),
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

// Start starts the HTTP server. Implements transport.LifecycleTransport.
func (t *StreamableHTTP) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", t.addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", t.addr, err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", t.handleMCP)
	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
		ReadTimeout:       defaultReadTimeout,
		WriteTimeout:      defaultWriteTimeout,
		IdleTimeout:       defaultIdleTimeout,
	}

	t.lifecycleMu.Lock()
	t.listener = ln
	t.server = srv
	t.lifecycleMu.Unlock()
	close(t.ready)

	// Tear down the HTTP server when either the caller-supplied context is
	// canceled OR Transport.Close() is invoked. Selecting on both avoids the
	// goroutine getting stuck on ctx.Done() if Close runs first.
	go func() {
		select {
		case <-ctx.Done():
		case <-t.closed:
		}
		srv.Close()
	}()

	go srv.Serve(ln)

	return nil
}

// Ready returns a channel that is closed when Start has finished binding the
// listener. Callers that need to interact with the transport's address (e.g.,
// tests, or in-process clients) should select on this rather than polling Addr.
func (t *StreamableHTTP) Ready() <-chan struct{} {
	return t.ready
}

// Addr returns the listen address. Implements transport.LifecycleTransport.
func (t *StreamableHTTP) Addr() string {
	t.lifecycleMu.RLock()
	ln := t.listener
	t.lifecycleMu.RUnlock()
	if ln != nil {
		return ln.Addr().String()
	}
	return t.addr
}

// Read reads the next incoming message. Implements transport.Transport.
func (t *StreamableHTTP) Read() (*jsonrpc.Message, error) {
	select {
	case msg := <-t.incoming:
		return msg, nil
	case <-t.closed:
		return nil, io.EOF
	}
}

// Write sends a response message back to the waiting client. Implements transport.Transport.
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

// Close shuts down the transport. Implements transport.Transport.
func (t *StreamableHTTP) Close() error {
	select {
	case <-t.closed:
	default:
		close(t.closed)
	}

	t.lifecycleMu.RLock()
	srv := t.server
	t.lifecycleMu.RUnlock()
	if srv != nil {
		return srv.Close()
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
	r.Body = http.MaxBytesReader(w, r.Body, defaultMaxBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}
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
		sessionID := r.Header.Get(transport.HeaderMCPSessionID)
		if sessionID != "" {
			sess, ok := t.sessions.lookup(sessionID)
			if !ok {
				http.Error(w, "Invalid session", http.StatusBadRequest)
				return
			}

			// Validate MCP-Protocol-Version header matches negotiated version.
			clientPV := r.Header.Get(transport.HeaderMCPProtocolVersion)
			if clientPV != "" && sess.protocolVersion != "" && clientPV != sess.protocolVersion {
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

	// Forward to server. If the client disconnects before the server picks
	// up the message, drop our pending entry so the map can't accumulate.
	select {
	case t.incoming <- &msg:
	case <-r.Context().Done():
		t.pendingMu.Lock()
		delete(t.pending, key)
		t.pendingMu.Unlock()
		return
	}

	// Wait for the response from the server loop or client disconnect.
	// Cleanup deletes the pending entry; Write may have already deleted it
	// (in which case the delete here is a no-op). If the server's Write
	// happens after we delete, the channel send into the 1-buffered
	// respCh succeeds anyway and the channel is GC'd.
	var resp *jsonrpc.Message
	select {
	case resp = <-respCh:
	case <-r.Context().Done():
		t.pendingMu.Lock()
		delete(t.pending, key)
		t.pendingMu.Unlock()
		return
	}

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
			w.Header().Set(transport.HeaderMCPSessionID, sessionID)
		}
	}

	// Check if client accepts SSE.
	accept := r.Header.Get("Accept")
	useSSE := strings.Contains(accept, "text/event-stream")

	if useSSE {
		sse := newSSEWriter(w)
		if sse != nil {
			setSSEHeaders(w)
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

	sessionID := r.Header.Get(transport.HeaderMCPSessionID)
	if sessionID != "" && !t.sessions.valid(sessionID) {
		http.Error(w, "Invalid session", http.StatusBadRequest)
		return
	}

	setSSEHeaders(w)
	w.WriteHeader(http.StatusOK)

	// Keep connection open until client disconnects.
	<-r.Context().Done()
}

// setSSEHeaders writes the standard Server-Sent Events response headers.
func setSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
}

// handleDelete terminates a session.
func (t *StreamableHTTP) handleDelete(w http.ResponseWriter, r *http.Request) {
	sessionID := r.Header.Get(transport.HeaderMCPSessionID)
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
