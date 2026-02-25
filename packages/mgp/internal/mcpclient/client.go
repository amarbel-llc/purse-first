package mcpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/transport"
)

type Client struct {
	transport *transport.Stdio
	cmd       *exec.Cmd
}

func Spawn(ctx context.Context, command string, args ...string) (*Client, error) {
	cmd := exec.CommandContext(ctx, command, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting process %s: %w", command, err)
	}

	t := transport.NewStdio(stdout, stdin)

	return &Client{
		transport: t,
		cmd:       cmd,
	}, nil
}

func (c *Client) Initialize(ctx context.Context) error {
	params := protocol.InitializeParamsV1{
		ProtocolVersion: protocol.ProtocolVersionV1,
		Capabilities:    protocol.ClientCapabilitiesV1{},
		ClientInfo: protocol.ImplementationV1{
			Name:    "mgp",
			Version: "0.1.0",
		},
	}

	req, err := c.buildRequest("initialize", params)
	if err != nil {
		return fmt.Errorf("building initialize request: %w", err)
	}

	if err := c.transport.Write(req); err != nil {
		return fmt.Errorf("sending initialize request: %w", err)
	}

	if _, err := c.readResponse(ctx); err != nil {
		return fmt.Errorf("reading initialize response: %w", err)
	}

	notif, err := jsonrpc.NewNotification("notifications/initialized", nil)
	if err != nil {
		return fmt.Errorf("building initialized notification: %w", err)
	}

	if err := c.transport.Write(notif); err != nil {
		return fmt.Errorf("sending initialized notification: %w", err)
	}

	return nil
}

func (c *Client) ListTools(ctx context.Context) ([]protocol.ToolV1, error) {
	req, err := c.buildRequest("tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("building tools/list request: %w", err)
	}

	if err := c.transport.Write(req); err != nil {
		return nil, fmt.Errorf("sending tools/list request: %w", err)
	}

	raw, err := c.readResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading tools/list response: %w", err)
	}

	var result protocol.ToolsListResultV1
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parsing tools/list result: %w", err)
	}

	return result.Tools, nil
}

func (c *Client) CallTool(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	params := protocol.ToolCallParams{
		Name:      name,
		Arguments: args,
	}

	req, err := c.buildRequest("tools/call", params)
	if err != nil {
		return nil, fmt.Errorf("building tools/call request: %w", err)
	}

	if err := c.transport.Write(req); err != nil {
		return nil, fmt.Errorf("sending tools/call request: %w", err)
	}

	raw, err := c.readResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading tools/call response: %w", err)
	}

	return raw, nil
}

func (c *Client) Close() error {
	c.transport.Close()
	if c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}
	return c.cmd.Wait()
}

var nextID int64

func (c *Client) buildRequest(method string, params any) (*jsonrpc.Message, error) {
	nextID++
	id := jsonrpc.NewNumberID(nextID)
	return jsonrpc.NewRequest(id, method, params)
}

func (c *Client) readResponse(ctx context.Context) (json.RawMessage, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		msg, err := c.transport.Read()
		if err != nil {
			return nil, err
		}

		if msg.IsNotification() {
			continue
		}

		if msg.Error != nil {
			return nil, msg.Error
		}

		return msg.Result, nil
	}
}
