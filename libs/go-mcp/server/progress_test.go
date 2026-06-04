package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
)

// TestToolCallEmitsProgress drives a tools/call carrying
// _meta.progressToken through Server.Run and asserts the handler's
// mid-call emit produces a notifications/progress message on the
// transport, echoing the token, alongside the final response.
func TestToolCallEmitsProgress(t *testing.T) {
	ct := newChanTransport()

	registry := NewToolRegistryV1()
	registry.Register(
		protocol.ToolV1{Name: "prog", InputSchema: json.RawMessage(`{}`)},
		func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResultV1, error) {
			emit, ok := ProgressFromContext(ctx)
			if !ok {
				t.Error("ProgressFromContext: ok = false, want true for a tokened call")
			}
			total := 2.0
			if err := emit(1, &total, "step"); err != nil {
				t.Errorf("emit: %v", err)
			}
			return &protocol.ToolCallResultV1{
				Content: []protocol.ContentBlockV1{protocol.TextContentV1("done")},
			}, nil
		},
	)

	s, err := New(ct, Options{
		ServerName:        "progress-test",
		ServerVersion:     "0.1",
		Tools:             registry,
		PreferV1Providers: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	callParams := protocol.ToolCallParams{
		Name: "prog",
		Meta: &protocol.ToolCallMeta{ProgressToken: json.RawMessage(`"tok-1"`)},
	}
	callJSON, _ := json.Marshal(callParams)
	callID := jsonrpc.NewNumberID(1)
	ct.in <- &jsonrpc.Message{
		JSONRPC: "2.0",
		ID:      &callID,
		Method:  protocol.MethodToolsCall,
		Params:  callJSON,
	}
	close(ct.in)

	if err := s.Run(t.Context()); err != nil {
		t.Fatal(err)
	}

	var gotProgress, gotResponse bool
	for drain := true; drain; {
		select {
		case msg := <-ct.out:
			switch {
			case msg.Method == protocol.MethodNotificationsProgress:
				var p protocol.ProgressNotificationParams
				if err := json.Unmarshal(msg.Params, &p); err != nil {
					t.Fatalf("unmarshal progress params: %v", err)
				}
				if got := string(p.ProgressToken); got != `"tok-1"` {
					t.Errorf("progress token = %s, want %q", got, `"tok-1"`)
				}
				if p.Progress != 1 {
					t.Errorf("progress = %v, want 1", p.Progress)
				}
				gotProgress = true
			case msg.IsResponse():
				gotResponse = true
			}
		default:
			drain = false
		}
	}

	if !gotProgress {
		t.Error("expected a notifications/progress message on the transport")
	}
	if !gotResponse {
		t.Error("expected a tools/call response on the transport")
	}
}

// TestToolCallWithoutTokenSkipsProgress confirms a token-less call
// leaves the no-op emitter in place: the handler sees ok = false, the
// no-op is safe to call, and nothing is written to the progress channel.
func TestToolCallWithoutTokenSkipsProgress(t *testing.T) {
	ct := newChanTransport()

	registry := NewToolRegistryV1()
	registry.Register(
		protocol.ToolV1{Name: "prog", InputSchema: json.RawMessage(`{}`)},
		func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResultV1, error) {
			emit, ok := ProgressFromContext(ctx)
			if ok {
				t.Error("ProgressFromContext: ok = true, want false for a token-less call")
			}
			if err := emit(0.5, nil, "noop"); err != nil {
				t.Errorf("no-op emit returned error: %v", err)
			}
			return &protocol.ToolCallResultV1{
				Content: []protocol.ContentBlockV1{protocol.TextContentV1("done")},
			}, nil
		},
	)

	s, err := New(ct, Options{
		ServerName:        "progress-test",
		Tools:             registry,
		PreferV1Providers: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	callJSON, _ := json.Marshal(protocol.ToolCallParams{Name: "prog"})
	callID := jsonrpc.NewNumberID(1)
	ct.in <- &jsonrpc.Message{
		JSONRPC: "2.0",
		ID:      &callID,
		Method:  protocol.MethodToolsCall,
		Params:  callJSON,
	}
	close(ct.in)

	if err := s.Run(t.Context()); err != nil {
		t.Fatal(err)
	}

	for drain := true; drain; {
		select {
		case msg := <-ct.out:
			if msg.Method == protocol.MethodNotificationsProgress {
				t.Error("unexpected notifications/progress for a token-less call")
			}
		default:
			drain = false
		}
	}
}
