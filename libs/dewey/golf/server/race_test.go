package server

import (
	"encoding/json"
	"io"
	"sync"
	"testing"

	"github.com/amarbel-llc/purse-first/libs/dewey/golf/jsonrpc"
	"github.com/amarbel-llc/purse-first/libs/dewey/golf/protocol"
)

// chanTransport is an in-memory transport backed by channels, suitable for
// testing the server's concurrent message dispatch.
type chanTransport struct {
	in      chan *jsonrpc.Message
	out     chan *jsonrpc.Message
	closeCh chan struct{}
	once    sync.Once
}

func newChanTransport() *chanTransport {
	return &chanTransport{
		in:      make(chan *jsonrpc.Message, 64),
		out:     make(chan *jsonrpc.Message, 64),
		closeCh: make(chan struct{}),
	}
}

func (t *chanTransport) Read() (*jsonrpc.Message, error) {
	select {
	case msg, ok := <-t.in:
		if !ok {
			return nil, io.EOF
		}
		return msg, nil
	case <-t.closeCh:
		return nil, io.EOF
	}
}

func (t *chanTransport) Write(msg *jsonrpc.Message) error {
	select {
	case t.out <- msg:
	case <-t.closeCh:
	}
	return nil
}

func (t *chanTransport) Close() error {
	t.once.Do(func() { close(t.closeCh) })
	return nil
}

// TestConcurrentInitializeAndToolsList sends initialize and tools/list
// simultaneously through Server.Run to trigger the data race on
// Handler.negotiatedVersion. Run with -race to detect it.
func TestConcurrentInitializeAndToolsList(t *testing.T) {
	ct := newChanTransport()

	s, err := New(ct, Options{
		ServerName:    "race-test",
		ServerVersion: "0.1",
		Tools:         &stubToolProvider{},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Build messages.
	initParams := protocol.InitializeParams{
		ProtocolVersion: protocol.ProtocolVersionV0,
		ClientInfo:      protocol.Implementation{Name: "test", Version: "1.0"},
	}
	initJSON, _ := json.Marshal(initParams)
	initID := jsonrpc.NewNumberID(1)
	initMsg := &jsonrpc.Message{
		JSONRPC: "2.0",
		ID:      &initID,
		Method:  protocol.MethodInitialize,
		Params:  initJSON,
	}

	toolsID := jsonrpc.NewNumberID(2)
	toolsMsg := &jsonrpc.Message{
		JSONRPC: "2.0",
		ID:      &toolsID,
		Method:  protocol.MethodToolsList,
	}

	// Enqueue both messages before the server starts reading, so they
	// are dispatched into separate goroutines concurrently.
	ct.in <- initMsg
	ct.in <- toolsMsg
	close(ct.in) // Signal EOF after both messages — server will shut down.

	// Run the server synchronously. It processes both messages then hits
	// EOF on Read and returns.
	if err := s.Run(t.Context()); err != nil {
		t.Fatal(err)
	}

	// Drain responses to confirm both were handled.
	for i := 0; i < 2; i++ {
		select {
		case resp := <-ct.out:
			if resp == nil {
				t.Fatal("nil response")
			}
		default:
			t.Fatalf("expected response %d, got nothing", i+1)
		}
	}
}
