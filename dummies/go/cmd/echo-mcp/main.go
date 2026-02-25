package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/server"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/transport"
)

type logEntry struct {
	Direction string          `json:"direction"`
	Timestamp string          `json:"timestamp"`
	Message   json.RawMessage `json:"message"`
}

type loggingTransport struct {
	inner transport.Transport
	file  *os.File
	mu    sync.Mutex
}

func newLoggingTransport(inner transport.Transport, path string) (*loggingTransport, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening log file: %w", err)
	}
	return &loggingTransport{inner: inner, file: f}, nil
}

func (t *loggingTransport) log(direction string, msg *jsonrpc.Message) {
	raw, err := json.Marshal(msg)
	if err != nil {
		return
	}
	entry := logEntry{
		Direction: direction,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Message:   raw,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	fmt.Fprintf(t.file, "%s\n", data)
}

func (t *loggingTransport) Read() (*jsonrpc.Message, error) {
	msg, err := t.inner.Read()
	if err != nil {
		return nil, err
	}
	t.log("recv", msg)
	return msg, nil
}

func (t *loggingTransport) Write(msg *jsonrpc.Message) error {
	t.log("send", msg)
	return t.inner.Write(msg)
}

func (t *loggingTransport) Close() error {
	if err := t.file.Close(); err != nil {
		log.Printf("closing log file: %v", err)
	}
	return t.inner.Close()
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	stdio := transport.NewStdio(os.Stdin, os.Stdout)
	lt, err := newLoggingTransport(stdio, "echo-mcp.jsonl")
	if err != nil {
		log.Fatalf("creating logging transport: %v", err)
	}

	registry := server.NewToolRegistryV1()
	registry.Register(
		protocol.ToolV1{
			Name:        "echo",
			Title:       "Echo",
			Description: "Echoes back the provided arguments with metadata (tool name, timestamp, argument count).",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"message": {
						"type": "string",
						"description": "A message to echo back"
					}
				},
				"additionalProperties": true
			}`),
			Annotations: &protocol.ToolAnnotations{
				ReadOnlyHint:   protocol.BoolPtr(true),
				IdempotentHint: protocol.BoolPtr(true),
			},
		},
		func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResultV1, error) {
			var argsMap map[string]json.RawMessage
			if err := json.Unmarshal(args, &argsMap); err != nil {
				argsMap = nil
			}

			response := map[string]any{
				"tool":      "echo",
				"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
				"argCount":  len(argsMap),
				"arguments": json.RawMessage(args),
			}

			data, err := json.MarshalIndent(response, "", "  ")
			if err != nil {
				return protocol.ErrorResultV1(fmt.Sprintf("marshaling response: %v", err)), nil
			}

			return &protocol.ToolCallResultV1{
				Content: []protocol.ContentBlockV1{
					protocol.TextContentV1(string(data)),
				},
			}, nil
		},
	)

	srv, err := server.New(lt, server.Options{
		ServerName:    "echo-mcp",
		ServerVersion: "0.1.0",
		Instructions:  "Echo MCP server for testing. Echoes back all tool arguments with metadata and logs all JSON-RPC traffic to echo-mcp.jsonl.",
		Tools:         registry,
	})
	if err != nil {
		log.Fatalf("creating server: %v", err)
	}

	if err := srv.Run(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
