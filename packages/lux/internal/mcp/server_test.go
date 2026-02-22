package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/transport"
	"github.com/amarbel-llc/lux/internal/config"
)

func TestMCPInitialize(t *testing.T) {
	initMsg := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test"}}}`

	resp := runMCPTest(t, initMsg)

	if resp.ID.String() != "1" {
		t.Errorf("expected id 1, got %s", resp.ID.String())
	}
	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}

	var result protocol.InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if result.ProtocolVersion != protocol.ProtocolVersion {
		t.Errorf("expected protocol version %s, got %s", protocol.ProtocolVersion, result.ProtocolVersion)
	}
	if result.ServerInfo.Name != "lux" {
		t.Errorf("expected server name 'lux', got %s", result.ServerInfo.Name)
	}
	if result.Capabilities.Tools == nil {
		t.Error("expected tools capability to be present")
	}
}

func TestMCPToolsList(t *testing.T) {
	initMsg := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test"}}}`
	toolsMsg := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`

	responses := runMCPTestMulti(t, initMsg, toolsMsg)
	if len(responses) < 2 {
		t.Fatalf("expected 2 responses, got %d", len(responses))
	}

	// Find the tools/list response by ID (order may vary due to goroutines)
	resp := findResponseByID(responses, "2")
	if resp == nil {
		t.Fatal("could not find response with id 2")
	}
	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}

	var result protocol.ToolsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	expectedTools := []string{
		"hover",
		"definition",
		"references",
		"completion",
		"format",
		"document_symbols",
		"code_action",
		"rename",
		"workspace_symbols",
		"diagnostics",
	}

	if len(result.Tools) != len(expectedTools) {
		t.Errorf("expected %d tools, got %d", len(expectedTools), len(result.Tools))
	}

	toolNames := make(map[string]bool)
	for _, tool := range result.Tools {
		toolNames[tool.Name] = true
	}

	for _, name := range expectedTools {
		if !toolNames[name] {
			t.Errorf("missing expected tool: %s", name)
		}
	}
}

func TestMCPPing(t *testing.T) {
	initMsg := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test"}}}`
	pingMsg := `{"jsonrpc":"2.0","id":2,"method":"ping","params":{}}`

	responses := runMCPTestMulti(t, initMsg, pingMsg)
	if len(responses) < 2 {
		t.Fatalf("expected 2 responses, got %d", len(responses))
	}

	resp := findResponseByID(responses, "2")
	if resp == nil {
		t.Fatal("could not find response with id 2")
	}
	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}
}

func TestMCPUnknownMethod(t *testing.T) {
	initMsg := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test"}}}`
	unknownMsg := `{"jsonrpc":"2.0","id":2,"method":"unknown/method","params":{}}`

	responses := runMCPTestMulti(t, initMsg, unknownMsg)
	if len(responses) < 2 {
		t.Fatalf("expected 2 responses, got %d", len(responses))
	}

	resp := findResponseByID(responses, "2")
	if resp == nil {
		t.Fatal("could not find response with id 2")
	}
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != jsonrpc.MethodNotFound {
		t.Errorf("expected MethodNotFound error code, got %d", resp.Error.Code)
	}
}

func runMCPTest(t *testing.T, msg string) *jsonrpc.Message {
	responses := runMCPTestMulti(t, msg)
	if len(responses) == 0 {
		t.Fatal("expected at least one response")
	}
	return responses[0]
}

func runMCPTestMulti(t *testing.T, msgs ...string) []*jsonrpc.Message {
	t.Helper()

	var input bytes.Buffer
	for _, msg := range msgs {
		input.WriteString(msg)
		input.WriteString("\n")
	}

	var output bytes.Buffer
	cfg := &config.Config{}
	tr := transport.NewStdio(strings.NewReader(input.String()), &output)

	srv, err := New(cfg, tr)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Run will return when input is exhausted (EOF)
	// Server waits for in-flight requests before returning
	srv.Run(context.Background())

	return parseResponses(t, output.String())
}

func findResponseByID(responses []*jsonrpc.Message, id string) *jsonrpc.Message {
	for _, r := range responses {
		if r.ID != nil && r.ID.String() == id {
			return r
		}
	}
	return nil
}

func parseResponses(t *testing.T, data string) []*jsonrpc.Message {
	t.Helper()

	var responses []*jsonrpc.Message
	lines := strings.Split(data, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var msg jsonrpc.Message
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			t.Logf("failed to parse response: %v (line: %s)", err, line)
			continue
		}
		responses = append(responses, &msg)
	}

	return responses
}
