package service

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/amarbel-llc/lux/internal/subprocess"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
)

// fakeExecutor implements subprocess.Executor using in-process pipes.
// Build is a no-op; Execute spawns a goroutine that speaks JSON-RPC.
type fakeExecutor struct{}

func (e *fakeExecutor) Build(_ context.Context, _, _ string) (string, error) {
	return "/fake/lsp", nil
}

func (e *fakeExecutor) Execute(_ context.Context, _ string, _ []string, _ map[string]string, _ string) (*subprocess.Process, error) {
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	done := make(chan struct{})

	go func() {
		defer close(done)
		conn := jsonrpc.NewConn(stdinR, stdoutW, func(_ context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
			return handleFakeLSP(msg)
		})
		conn.Run(context.Background())
	}()

	return &subprocess.Process{
		Stdin:  stdinW,
		Stdout: stdoutR,
		Stderr: io.NopCloser(strings.NewReader("")),
		Wait: func() error {
			<-done
			return nil
		},
		Kill: func() error {
			stdinR.Close()
			stdoutW.Close()
			return nil
		},
	}, nil
}

// notifyingFakeExecutor is like fakeExecutor but the fake LSP sends a
// textDocument/publishDiagnostics notification after receiving didOpen.
// This exercises the daemon → client notification broadcast path.
type notifyingFakeExecutor struct{}

func (e *notifyingFakeExecutor) Build(_ context.Context, _, _ string) (string, error) {
	return "/fake/lsp", nil
}

func (e *notifyingFakeExecutor) Execute(_ context.Context, _ string, _ []string, _ map[string]string, _ string) (*subprocess.Process, error) {
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	done := make(chan struct{})

	go func() {
		defer close(done)
		var conn *jsonrpc.Conn
		conn = jsonrpc.NewConn(stdinR, stdoutW, func(_ context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
			return handleNotifyingFakeLSP(conn, msg)
		})
		conn.Run(context.Background())
	}()

	return &subprocess.Process{
		Stdin:  stdinW,
		Stdout: stdoutR,
		Stderr: io.NopCloser(strings.NewReader("")),
		Wait: func() error {
			<-done
			return nil
		},
		Kill: func() error {
			stdinR.Close()
			stdoutW.Close()
			return nil
		},
	}, nil
}

func handleNotifyingFakeLSP(conn *jsonrpc.Conn, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	if msg.Method == "textDocument/didOpen" {
		var params struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		if err := json.Unmarshal(msg.Params, &params); err == nil {
			conn.Notify("textDocument/publishDiagnostics", map[string]any{
				"uri":         params.TextDocument.URI,
				"diagnostics": []any{},
			})
		}
		return nil, nil
	}

	return handleFakeLSP(msg)
}

func handleFakeLSP(msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	if msg.ID == nil {
		return nil, nil
	}
	switch msg.Method {
	case "initialize":
		result := map[string]any{
			"capabilities": map[string]any{},
		}
		return jsonrpc.NewResponse(*msg.ID, result)
	case "shutdown":
		return jsonrpc.NewResponse(*msg.ID, nil)
	default:
		result := map[string]any{
			"echo": msg.Method,
		}
		return jsonrpc.NewResponse(*msg.ID, result)
	}
}
