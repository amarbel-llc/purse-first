package transport

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/amarbel-llc/go-lib-mcp/jsonrpc"
)

// Stdio implements MCP stdio transport using newline-delimited JSON.
// This differs from LSP which uses Content-Length headers.
// Each JSON-RPC message is written on a single line, terminated by a newline.
type Stdio struct {
	scanner *bufio.Scanner
	writer  io.Writer
	closer  io.Closer
	mu      sync.Mutex
}

// NewStdio creates a new stdio transport.
func NewStdio(r io.Reader, w io.Writer) *Stdio {
	scanner := bufio.NewScanner(r)
	// Increase buffer size for large messages (64KB initial, 1MB max)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	return &Stdio{
		scanner: scanner,
		writer:  w,
	}
}

// NewStdioWithCloser creates a new stdio transport with a closer.
// The closer will be called when Close() is invoked.
func NewStdioWithCloser(r io.Reader, w io.Writer, c io.Closer) *Stdio {
	t := NewStdio(r, w)
	t.closer = c
	return t
}

// Read reads a newline-delimited JSON message from the transport.
func (t *Stdio) Read() (*jsonrpc.Message, error) {
	if !t.scanner.Scan() {
		if err := t.scanner.Err(); err != nil {
			return nil, fmt.Errorf("reading message: %w", err)
		}
		return nil, io.EOF
	}

	line := t.scanner.Bytes()
	if len(line) == 0 {
		// Skip empty lines and try again
		return t.Read()
	}

	var msg jsonrpc.Message
	if err := json.Unmarshal(line, &msg); err != nil {
		return nil, fmt.Errorf("parsing message: %w", err)
	}

	return &msg, nil
}

// Write writes a newline-delimited JSON message to the transport.
func (t *Stdio) Write(msg *jsonrpc.Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling message: %w", err)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if _, err := fmt.Fprintf(t.writer, "%s\n", data); err != nil {
		return fmt.Errorf("writing message: %w", err)
	}

	return nil
}

// Close closes the transport.
func (t *Stdio) Close() error {
	if t.closer != nil {
		return t.closer.Close()
	}
	return nil
}
