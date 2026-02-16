// Package transport defines the transport layer interface for MCP servers.
// Different transports can be used depending on the communication channel:
// - Stdio transport for MCP (newline-delimited JSON)
// - Stream transport for LSP (Content-Length headers, available via jsonrpc package)
package transport

import "github.com/amarbel-llc/go-lib-mcp/jsonrpc"

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
