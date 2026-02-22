package transport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
)

// SSE implements the MCP SSE transport over HTTP.
// GET /sse establishes a Server-Sent Events stream for server-to-client messages.
// POST /message sends client-to-server JSON-RPC messages.
type SSE struct {
	server   *http.Server
	listener net.Listener

	sessionID  string
	sseWriter  io.Writer
	sseFlusher http.Flusher
	sseReady   chan struct{}
	incoming   chan *jsonrpc.Message
	done       chan struct{}
	closeOnce  sync.Once
	mu         sync.Mutex
}

// NewSSE creates a new SSE transport that listens on the given address.
func NewSSE(addr string) *SSE {
	return &SSE{
		sseReady: make(chan struct{}),
		incoming: make(chan *jsonrpc.Message, 16),
		done:     make(chan struct{}),
		server: &http.Server{
			Addr: addr,
		},
	}
}

// Addr returns the listener's address. Only valid after Start returns.
func (s *SSE) Addr() net.Addr {
	return s.listener.Addr()
}

// Start begins serving HTTP on the configured address.
// It blocks until the listener is ready, then serves in the background.
func (s *SSE) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /sse", s.handleSSE)
	mux.HandleFunc("POST /message", s.handleMessage)
	s.server.Handler = mux
	s.server.BaseContext = func(_ net.Listener) context.Context { return ctx }

	ln, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", s.server.Addr, err)
	}

	s.listener = ln

	go func() {
		if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Printf("SSE server error: %v\n", err)
		}
	}()

	return nil
}

func (s *SSE) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	s.mu.Lock()
	// Reject if a session is already active.
	select {
	case <-s.sseReady:
		s.mu.Unlock()
		http.Error(w, "session already established", http.StatusConflict)
		return
	default:
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		s.mu.Unlock()
		http.Error(w, "generating session id", http.StatusInternalServerError)
		return
	}

	s.sessionID = hex.EncodeToString(b)
	s.sseWriter = w
	s.sseFlusher = flusher
	close(s.sseReady)
	s.mu.Unlock()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send the endpoint event so the client knows where to POST.
	fmt.Fprintf(w, "event: endpoint\ndata: /message?sessionId=%s\n\n", s.sessionID)
	flusher.Flush()

	// Block until shutdown or client disconnect.
	select {
	case <-s.done:
	case <-r.Context().Done():
	}

	// Client disconnected — tear down the session.
	s.mu.Lock()
	s.sseWriter = nil
	s.sseFlusher = nil
	s.mu.Unlock()

	s.closeDone()
}

func (s *SSE) handleMessage(w http.ResponseWriter, r *http.Request) {
	// Ensure SSE stream is established first.
	select {
	case <-s.sseReady:
	default:
		http.Error(w, "SSE stream not established", http.StatusServiceUnavailable)
		return
	}

	qid := r.URL.Query().Get("sessionId")
	if qid == "" || qid != s.sessionID {
		http.Error(w, "invalid or missing session ID", http.StatusForbidden)
		return
	}

	var msg jsonrpc.Message
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	select {
	case s.incoming <- &msg:
		w.WriteHeader(http.StatusAccepted)
	case <-s.done:
		http.Error(w, "server shutting down", http.StatusServiceUnavailable)
	}
}

// Read returns the next client message, or io.EOF when the connection closes.
func (s *SSE) Read() (*jsonrpc.Message, error) {
	select {
	case msg := <-s.incoming:
		return msg, nil
	case <-s.done:
		return nil, io.EOF
	}
}

// Write sends a JSON-RPC message to the client as an SSE event.
func (s *SSE) Write(msg *jsonrpc.Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling message: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sseWriter == nil {
		return fmt.Errorf("SSE stream not connected")
	}

	if _, err := fmt.Fprintf(s.sseWriter, "event: message\ndata: %s\n\n", data); err != nil {
		return fmt.Errorf("writing SSE event: %w", err)
	}

	s.sseFlusher.Flush()
	return nil
}

// Close shuts down the HTTP server and signals all goroutines to stop.
func (s *SSE) Close() error {
	s.closeDone()
	return s.server.Shutdown(context.Background())
}

func (s *SSE) closeDone() {
	s.closeOnce.Do(func() { close(s.done) })
}
