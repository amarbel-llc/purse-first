# Echo MCP Dummy Package Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Create a minimal echo MCP server at `dummies/go/` that echoes tool arguments back with metadata and logs all JSON-RPC traffic to a file.

**Architecture:** Raw go-mcp server layer (no `command.App`). A logging transport wraps `transport.NewStdio` to intercept all JSON-RPC messages. A single `echo` tool returns arguments plus metadata.

**Tech Stack:** Go, `github.com/amarbel-llc/purse-first/libs/go-mcp` (server, transport, protocol, jsonrpc packages)

---

### Task 1: Create go.mod and add to go.work

**Files:**
- Create: `dummies/go/go.mod`
- Modify: `go.work`

**Step 1: Create go.mod**

Create `dummies/go/go.mod`:

```go
module github.com/amarbel-llc/purse-first/dummies/go

go 1.25.6

require github.com/amarbel-llc/purse-first/libs/go-mcp v0.0.3-0.20260222205500-74480472530e

replace github.com/amarbel-llc/purse-first/libs/go-mcp => ../../libs/go-mcp
```

**Step 2: Add to go.work**

Add `./dummies/go` to the `use` block in `go.work`.

**Step 3: Verify workspace resolves**

Run: `go work sync` from the project root.

Expected: No errors.

**Step 4: Commit**

```
feat(dummies): scaffold go module for echo-mcp dummy
```

---

### Task 2: Create the logging transport

**Files:**
- Create: `dummies/go/cmd/echo-mcp/main.go`

**Step 1: Write the logging transport and echo tool**

Create `dummies/go/cmd/echo-mcp/main.go` with:

1. A `loggingTransport` struct that wraps `transport.Transport`, opens a file (`echo-mcp.jsonl` in the current directory), and writes every `Read()` and `Write()` message as a JSON line with direction and timestamp metadata.

2. A single `echo` tool registered on a `ToolRegistryV1` that:
   - Accepts a single optional `message` parameter (string)
   - Returns a JSON object containing: the original arguments (raw), tool name, timestamp, and a count of top-level argument keys

3. Server setup using `server.New` with stdio transport wrapped in the logging transport.

```go
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
	t.file.Close()
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
				ReadOnlyHint:    protocol.BoolPtr(true),
				IdempotentHint:  protocol.BoolPtr(true),
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
```

**Step 2: Verify it compiles**

Run: `go build ./dummies/go/cmd/echo-mcp`

Expected: No errors, binary produced at `./echo-mcp` (or in build dir).

**Step 3: Commit**

```
feat(dummies): add echo-mcp server with logging transport
```

---

### Task 3: Clean up build artifact

**Step 1: Remove the built binary**

The `go build` in task 2 produces a binary in the repo root. Remove it.

Run: `rm -f echo-mcp`

**Step 2: Verify go vet passes**

Run: `go vet ./dummies/go/...`

Expected: No issues.
