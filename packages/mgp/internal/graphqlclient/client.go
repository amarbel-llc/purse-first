package graphqlclient

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
)

type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
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

	return &Client{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewScanner(stdout),
	}, nil
}

type graphqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

func (c *Client) Query(ctx context.Context, query string, variables map[string]any) (json.RawMessage, error) {
	req := graphqlRequest{
		Query:     query,
		Variables: variables,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	data = append(data, '\n')
	if _, err := c.stdin.Write(data); err != nil {
		return nil, fmt.Errorf("writing request: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if !c.stdout.Scan() {
		if err := c.stdout.Err(); err != nil {
			return nil, fmt.Errorf("reading response: %w", err)
		}
		return nil, fmt.Errorf("reading response: unexpected EOF")
	}

	// Copy bytes since Scanner reuses the buffer
	line := make([]byte, len(c.stdout.Bytes()))
	copy(line, c.stdout.Bytes())
	return json.RawMessage(line), nil
}

func (c *Client) Close() error {
	c.stdin.Close()
	if c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}
	err := c.cmd.Wait()
	// Ignore "signal: killed" since we just killed it intentionally
	if err != nil && c.cmd.ProcessState != nil && !c.cmd.ProcessState.Exited() {
		return nil
	}
	return err
}
