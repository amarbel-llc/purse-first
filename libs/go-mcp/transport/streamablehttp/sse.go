package streamablehttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"

	"code.linenisgreat.com/purse-first/libs/go-mcp/jsonrpc"
)

// sseWriter writes Server-Sent Events to an HTTP response.
type sseWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	eventID atomic.Int64
}

// newSSEWriter wraps an http.ResponseWriter for SSE output.
// Returns nil if the ResponseWriter does not support flushing.
func newSSEWriter(w http.ResponseWriter) *sseWriter {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil
	}
	return &sseWriter{w: w, flusher: flusher}
}

// writeMessage writes a JSON-RPC message as an SSE event.
func (s *sseWriter) writeMessage(msg *jsonrpc.Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling SSE event: %w", err)
	}

	id := s.eventID.Add(1)
	_, err = fmt.Fprintf(s.w, "id: %d\nevent: message\ndata: %s\n\n", id, data)
	if err != nil {
		return fmt.Errorf("writing SSE event: %w", err)
	}

	s.flusher.Flush()
	return nil
}
