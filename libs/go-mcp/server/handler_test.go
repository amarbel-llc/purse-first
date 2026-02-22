package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
)

// stubToolProvider is a minimal V0 tool provider for tests.
type stubToolProvider struct{}

func (s *stubToolProvider) ListTools(ctx context.Context) ([]protocol.Tool, error) {
	return []protocol.Tool{
		{Name: "echo", Description: "Echo tool", InputSchema: json.RawMessage(`{"type":"object"}`)},
	}, nil
}

func (s *stubToolProvider) CallTool(ctx context.Context, name string, args json.RawMessage) (*protocol.ToolCallResult, error) {
	return &protocol.ToolCallResult{
		Content: []protocol.ContentBlock{protocol.TextContent("ok")},
	}, nil
}

func TestVersionNegotiationV0(t *testing.T) {
	s := &Server{
		opts: Options{
			ServerName:    "test",
			ServerVersion: "1.0",
			Tools:         &stubToolProvider{},
		},
	}
	s.handler = NewHandler(s)

	// Client sends V0 initialize.
	initMsg := makeInitialize(t, protocol.ProtocolVersionV0)
	resp, err := s.handler.Handle(context.Background(), initMsg)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	var result protocol.InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result.ProtocolVersion != protocol.ProtocolVersionV0 {
		t.Errorf("negotiated version = %q, want V0", result.ProtocolVersion)
	}
}

func TestVersionNegotiationV1FallbackWhenNoV1Providers(t *testing.T) {
	s := &Server{
		opts: Options{
			ServerName:    "test",
			ServerVersion: "1.0",
			Tools:         &stubToolProvider{}, // V0 only
		},
	}
	s.handler = NewHandler(s)

	// Client requests V1, but server has no V1 providers — should fall back to V0.
	initMsg := makeInitialize(t, protocol.ProtocolVersionV1)
	resp, err := s.handler.Handle(context.Background(), initMsg)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	var result protocol.InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result.ProtocolVersion != protocol.ProtocolVersionV0 {
		t.Errorf("negotiated version = %q, want V0 fallback", result.ProtocolVersion)
	}
}

func TestVersionNegotiationV1WithLoggingProvider(t *testing.T) {
	s := &Server{
		opts: Options{
			ServerName:    "test",
			ServerVersion: "1.0",
			Tools:         &stubToolProvider{},
			Logging:       &stubLoggingHandler{},
		},
	}
	s.handler = NewHandler(s)

	// Client requests V1 and server has a V1 provider (logging).
	initMsg := makeInitialize(t, protocol.ProtocolVersionV1)
	resp, err := s.handler.Handle(context.Background(), initMsg)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	var result protocol.InitializeResultV1
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal V1 result: %v", err)
	}

	if result.ProtocolVersion != protocol.ProtocolVersionV1 {
		t.Errorf("negotiated version = %q, want V1", result.ProtocolVersion)
	}
	if result.Capabilities.Tools == nil {
		t.Error("tools capability should be present")
	}
	if result.Capabilities.Logging == nil {
		t.Error("logging capability should be present")
	}
}

func TestVersionNegotiationV1WithToolRegistryV1(t *testing.T) {
	registry := NewToolRegistryV1()
	registry.Register(protocol.ToolV1{
		Name:        "greet",
		Description: "Greeting tool",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}, func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResultV1, error) {
		return &protocol.ToolCallResultV1{
			Content: []protocol.ContentBlockV1{{Type: "text", Text: "hello"}},
		}, nil
	})

	s := &Server{
		opts: Options{
			ServerName:    "test",
			ServerVersion: "1.0",
			Instructions:  "Test server instructions",
			Tools:         registry,
		},
	}
	s.handler = NewHandler(s)

	// Client requests V1 and server has a V1 tool provider.
	initMsg := makeInitialize(t, protocol.ProtocolVersionV1)
	resp, err := s.handler.Handle(context.Background(), initMsg)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	var result protocol.InitializeResultV1
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal V1 result: %v", err)
	}

	if result.ProtocolVersion != protocol.ProtocolVersionV1 {
		t.Errorf("negotiated version = %q, want V1", result.ProtocolVersion)
	}
	if result.Instructions != "Test server instructions" {
		t.Errorf("instructions = %q, want %q", result.Instructions, "Test server instructions")
	}
	if result.Capabilities.Tools == nil {
		t.Error("tools capability should be present")
	}
}

func TestPingHandler(t *testing.T) {
	s := &Server{
		opts: Options{ServerName: "test"},
	}
	s.handler = NewHandler(s)

	id := jsonrpc.NewNumberID(1)
	msg := &jsonrpc.Message{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "ping",
	}

	resp, err := s.handler.Handle(context.Background(), msg)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestUnknownMethodReturnsError(t *testing.T) {
	s := &Server{
		opts: Options{ServerName: "test"},
	}
	s.handler = NewHandler(s)

	id := jsonrpc.NewNumberID(1)
	msg := &jsonrpc.Message{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "nonexistent/method",
	}

	resp, err := s.handler.Handle(context.Background(), msg)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != jsonrpc.MethodNotFound {
		t.Errorf("error code = %d, want %d", resp.Error.Code, jsonrpc.MethodNotFound)
	}
}

func TestToolsListV0(t *testing.T) {
	s := &Server{
		opts: Options{
			ServerName: "test",
			Tools:      &stubToolProvider{},
		},
	}
	s.handler = NewHandler(s)

	// Initialize first.
	initMsg := makeInitialize(t, protocol.ProtocolVersionV0)
	s.handler.Handle(context.Background(), initMsg)

	// List tools.
	id := jsonrpc.NewNumberID(2)
	msg := &jsonrpc.Message{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "tools/list",
	}

	resp, err := s.handler.Handle(context.Background(), msg)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var result protocol.ToolsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(result.Tools) != 1 {
		t.Errorf("tools count = %d, want 1", len(result.Tools))
	}
}

// Helpers

func makeInitialize(t *testing.T, version string) *jsonrpc.Message {
	t.Helper()
	params := protocol.InitializeParams{
		ProtocolVersion: version,
		ClientInfo: protocol.Implementation{
			Name:    "test-client",
			Version: "1.0",
		},
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	id := jsonrpc.NewNumberID(1)
	return &jsonrpc.Message{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "initialize",
		Params:  paramsJSON,
	}
}

// stubLoggingHandler satisfies LoggingHandler for tests.
type stubLoggingHandler struct{}

func (s *stubLoggingHandler) SetLevel(ctx context.Context, level protocol.LoggingLevel) error {
	return nil
}
