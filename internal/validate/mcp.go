package validate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/amarbel-llc/purse-first/libs/dewey/golf/jsonrpc"
	"github.com/amarbel-llc/purse-first/libs/dewey/golf/protocol"
	"github.com/amarbel-llc/purse-first/libs/dewey/golf/transport"
)

const mcpValidationTimeout = 10 * time.Second

// mcpClient wraps a transport.Stdio to provide Call and Notify over
// newline-delimited JSON, which is what MCP servers use.
type mcpClient struct {
	t       *transport.Stdio
	pending map[string]chan *jsonrpc.Message
	mu      sync.Mutex
	nextID  atomic.Int64
	closed  atomic.Bool
}

func newMCPClient(r io.Reader, w io.Writer) *mcpClient {
	return &mcpClient{
		t:       transport.NewStdio(r, w),
		pending: make(map[string]chan *jsonrpc.Message),
	}
}

func (c *mcpClient) Run(ctx context.Context) error {
	for {
		msg, err := c.t.Read()
		if err != nil {
			if c.closed.Load() {
				return nil
			}
			return fmt.Errorf("reading message: %w", err)
		}

		if msg.IsResponse() {
			c.mu.Lock()
			ch, ok := c.pending[msg.ID.String()]
			if ok {
				delete(c.pending, msg.ID.String())
			}
			c.mu.Unlock()
			if ok {
				ch <- msg
				close(ch)
			}
		}
	}
}

func (c *mcpClient) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := jsonrpc.NewNumberID(c.nextID.Add(1))

	msg, err := jsonrpc.NewRequest(id, method, params)
	if err != nil {
		return nil, err
	}

	ch := make(chan *jsonrpc.Message, 1)
	c.mu.Lock()
	c.pending[id.String()] = ch
	c.mu.Unlock()

	if err := c.t.Write(msg); err != nil {
		c.mu.Lock()
		delete(c.pending, id.String())
		c.mu.Unlock()
		return nil, err
	}

	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id.String())
		c.mu.Unlock()
		return nil, ctx.Err()
	case resp := <-ch:
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	}
}

func (c *mcpClient) Notify(method string, params any) error {
	msg, err := jsonrpc.NewNotification(method, params)
	if err != nil {
		return err
	}
	return c.t.Write(msg)
}

func (c *mcpClient) Close() error {
	c.closed.Store(true)
	return c.t.Close()
}

func ValidateMCP(ctx context.Context, binary string, args ...string) (*Result, error) {
	ctx, cancel := context.WithTimeout(ctx, mcpValidationTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting process: %w", err)
	}

	client := newMCPClient(stdout, stdin)

	connErr := make(chan error, 1)
	go func() {
		connErr <- client.Run(ctx)
	}()

	r := &Result{}

	if err := validateMCPInitialize(ctx, client, r); err != nil {
		stdin.Close()
		cmd.Wait()
		return nil, fmt.Errorf("initialize: %w", err)
	}

	validateMCPToolsList(ctx, client, r)
	validateMCPResourcesList(ctx, client, r)
	validateMCPResourceTemplatesList(ctx, client, r)

	client.Close()
	stdin.Close()

	if err := cmd.Wait(); err != nil {
		r.addWarning("mcp", fmt.Sprintf("process exited with: %v", err))
	}

	select {
	case err := <-connErr:
		if err != nil && !isBenignConnErr(err) {
			r.addWarning("mcp", fmt.Sprintf("connection error: %v", err))
		}
	default:
	}

	return r, nil
}

func validateMCPInitialize(ctx context.Context, client *mcpClient, r *Result) error {
	// Request V1 — servers that only support V0 will negotiate down.
	params := protocol.InitializeParamsV1{
		ProtocolVersion: protocol.ProtocolVersionV1,
		Capabilities:    protocol.ClientCapabilitiesV1{},
		ClientInfo: protocol.ImplementationV1{
			Name:    "purse-first-validate",
			Version: "0.1.0",
		},
	}

	raw, err := client.Call(ctx, protocol.MethodInitialize, params)
	if err != nil {
		return fmt.Errorf("calling initialize: %w", err)
	}

	var result protocol.InitializeResultV1
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("decoding initialize result: %w", err)
	}

	switch result.ProtocolVersion {
	case protocol.ProtocolVersionV1:
		r.addInfo("initialize", "negotiated protocol V1 (2025-11-25)")
	case protocol.ProtocolVersionV0:
		r.addInfo("initialize", "negotiated protocol V0 (2024-11-05)")
	case "":
		r.addError("initialize", "response missing protocolVersion")
	default:
		r.addWarning("initialize", fmt.Sprintf("unknown protocolVersion %q", result.ProtocolVersion))
	}

	if err := client.Notify(protocol.MethodInitialized, nil); err != nil {
		return fmt.Errorf("sending initialized notification: %w", err)
	}

	return nil
}

func validateMCPToolsList(ctx context.Context, client *mcpClient, r *Result) {
	raw, err := client.Call(ctx, protocol.MethodToolsList, nil)
	if err != nil {
		r.addError("tools/list", fmt.Sprintf("call failed: %s", err))
		return
	}

	var result protocol.ToolsListResultV1
	if err := json.Unmarshal(raw, &result); err != nil {
		r.addError("tools/list", fmt.Sprintf("invalid response: %s", err))
		return
	}

	if len(result.Tools) == 0 {
		r.addError("tools/list", "server returned no tools")
		return
	}

	for _, tool := range result.Tools {
		if tool.Annotations == nil {
			r.addWarning("tools/list", fmt.Sprintf("tool %q has no annotations", tool.Name))
		}
	}

	r.addInfo("tools/list", fmt.Sprintf("server exposes %d tool(s)", len(result.Tools)))
}

func validateMCPResourcesList(ctx context.Context, client *mcpClient, r *Result) {
	raw, err := client.Call(ctx, protocol.MethodResourcesList, nil)
	if err != nil {
		if isMethodNotSupported(err) {
			r.addInfo("resources/list", "method not supported by server")
			return
		}
		r.addError("resources/list", fmt.Sprintf("call failed: %s", err))
		return
	}

	var result protocol.ResourcesListResultV1
	if err := json.Unmarshal(raw, &result); err != nil {
		r.addError("resources/list", fmt.Sprintf("invalid response: %s", err))
		return
	}
}

func validateMCPResourceTemplatesList(ctx context.Context, client *mcpClient, r *Result) {
	raw, err := client.Call(ctx, protocol.MethodResourcesTemplates, nil)
	if err != nil {
		if isMethodNotSupported(err) {
			r.addInfo("resources/templates/list", "method not supported by server")
			return
		}
		r.addError("resources/templates/list", fmt.Sprintf("call failed: %s", err))
		return
	}

	var result protocol.ResourceTemplatesListResultV1
	if err := json.Unmarshal(raw, &result); err != nil {
		r.addError("resources/templates/list", fmt.Sprintf("invalid response: %s", err))
		return
	}
}

func isBenignConnErr(err error) bool {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) {
		return true
	}

	msg := err.Error()
	return strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "closed pipe") ||
		strings.Contains(msg, "use of closed") ||
		strings.Contains(msg, "file already closed")
}

func isMethodNotSupported(err error) bool {
	var rpcErr *jsonrpc.Error
	if errors.As(err, &rpcErr) {
		if rpcErr.Code == jsonrpc.MethodNotFound {
			return true
		}
		// Some servers return InternalError with "not supported" message
		// instead of MethodNotFound.
		if rpcErr.Code == jsonrpc.InternalError &&
			strings.Contains(strings.ToLower(rpcErr.Message), "not supported") {
			return true
		}
	}
	return false
}
