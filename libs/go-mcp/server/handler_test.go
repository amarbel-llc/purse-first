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

// stubToolProviderV1 implements both ToolProvider (V0) and ToolProviderV1.
// It tracks which method was called.
type stubToolProviderV1 struct {
	calledV0 bool
	calledV1 bool
}

func (s *stubToolProviderV1) ListTools(ctx context.Context) ([]protocol.Tool, error) {
	s.calledV0 = true
	return []protocol.Tool{
		{Name: "echo", Description: "Echo tool", InputSchema: json.RawMessage(`{"type":"object"}`)},
	}, nil
}

func (s *stubToolProviderV1) CallTool(ctx context.Context, name string, args json.RawMessage) (*protocol.ToolCallResult, error) {
	s.calledV0 = true
	return &protocol.ToolCallResult{
		Content: []protocol.ContentBlock{protocol.TextContent("v0")},
	}, nil
}

func (s *stubToolProviderV1) ListToolsV1(ctx context.Context, cursor string) (*protocol.ToolsListResultV1, error) {
	s.calledV1 = true
	return &protocol.ToolsListResultV1{
		Tools: []protocol.ToolV1{
			{Name: "echo", Description: "Echo tool", InputSchema: json.RawMessage(`{"type":"object"}`)},
		},
	}, nil
}

func (s *stubToolProviderV1) CallToolV1(ctx context.Context, name string, args json.RawMessage) (*protocol.ToolCallResultV1, error) {
	s.calledV1 = true
	return &protocol.ToolCallResultV1{
		Content: []protocol.ContentBlockV1{{Type: "text", Text: "v1"}},
	}, nil
}

func TestPreferV1ProvidersCallTool(t *testing.T) {
	tools := &stubToolProviderV1{}
	s := &Server{
		opts: Options{
			ServerName:        "test",
			ServerVersion:     "1.0",
			Tools:             tools,
			PreferV1Providers: true,
		},
	}
	s.handler = NewHandler(s)

	// Client negotiates V0.
	initMsg := makeInitialize(t, protocol.ProtocolVersionV0)
	resp, err := s.handler.Handle(context.Background(), initMsg)
	if err != nil {
		t.Fatalf("Handle initialize: %v", err)
	}

	var initResult protocol.InitializeResult
	if err := json.Unmarshal(resp.Result, &initResult); err != nil {
		t.Fatalf("unmarshal init result: %v", err)
	}
	if initResult.ProtocolVersion != protocol.ProtocolVersionV0 {
		t.Fatalf("expected V0 negotiation, got %q", initResult.ProtocolVersion)
	}

	// Call a tool — should use V1 provider despite V0 negotiation.
	callParams, _ := json.Marshal(protocol.ToolCallParams{Name: "echo"})
	callID := jsonrpc.NewNumberID(2)
	callMsg := &jsonrpc.Message{
		JSONRPC: "2.0",
		ID:      &callID,
		Method:  "tools/call",
		Params:  callParams,
	}

	resp, err = s.handler.Handle(context.Background(), callMsg)
	if err != nil {
		t.Fatalf("Handle tools/call: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("tools/call error: %v", resp.Error)
	}

	if !tools.calledV1 {
		t.Error("expected CallToolV1 to be called, but it was not")
	}
	if tools.calledV0 {
		t.Error("expected CallTool (V0) NOT to be called, but it was")
	}
}

func TestPreferV1ProvidersListTools(t *testing.T) {
	tools := &stubToolProviderV1{}
	s := &Server{
		opts: Options{
			ServerName:        "test",
			ServerVersion:     "1.0",
			Tools:             tools,
			PreferV1Providers: true,
		},
	}
	s.handler = NewHandler(s)

	// Client negotiates V0.
	initMsg := makeInitialize(t, protocol.ProtocolVersionV0)
	s.handler.Handle(context.Background(), initMsg)

	// List tools — should use V1 provider despite V0 negotiation.
	listID := jsonrpc.NewNumberID(2)
	listMsg := &jsonrpc.Message{
		JSONRPC: "2.0",
		ID:      &listID,
		Method:  "tools/list",
	}

	resp, err := s.handler.Handle(context.Background(), listMsg)
	if err != nil {
		t.Fatalf("Handle tools/list: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("tools/list error: %v", resp.Error)
	}

	if !tools.calledV1 {
		t.Error("expected ListToolsV1 to be called, but it was not")
	}
	if tools.calledV0 {
		t.Error("expected ListTools (V0) NOT to be called, but it was")
	}
}

// stubLoggingHandler satisfies LoggingHandler for tests.
type stubLoggingHandler struct{}

func (s *stubLoggingHandler) SetLevel(ctx context.Context, level protocol.LoggingLevel) error {
	return nil
}
