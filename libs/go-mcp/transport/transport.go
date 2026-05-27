// Package transport defines the transport layer interface for MCP servers.
// Different transports can be used depending on the communication channel:
//   - Stdio transport for MCP (newline-delimited JSON)
//   - Stream transport for LSP (Content-Length headers, available via jsonrpc package)
//   - Streamable HTTP transport for MCP V1 (POST + SSE), provided by the
//     transport/streamablehttp subpackage so plugin binaries don't pay for
//     net/http unless they import it explicitly.
package transport

import (
	"context"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
)

// Transport defines the interface for sending and receiving JSON-RPC messages.
// Implementations handle the wire protocol details (framing, encoding, etc.).
type Transport interface {
	// Read reads the next message from the transport.
	// Returns io.EOF when the connection is closed gracefully.
	Read() (*jsonrpc.Message, error)

	// Write sends a message over the transport.
	Write(*jsonrpc.Message) error

	// Close closes the transport and releases any resources.
	Close() error
}

// LifecycleTransport extends Transport with lifecycle management for transports
// that need to start background services (e.g., HTTP servers).
type LifecycleTransport interface {
	Transport

	// Start starts the transport's background services (e.g., HTTP listener).
	// The context controls the transport's lifetime.
	Start(ctx context.Context) error

	// Addr returns the transport's listen address (e.g., "127.0.0.1:8080").
	// Only valid after Start() returns.
	Addr() string
}
