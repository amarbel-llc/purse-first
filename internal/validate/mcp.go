package validate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
)

const mcpValidationTimeout = 10 * time.Second

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

	conn := jsonrpc.NewConn(stdout, stdin, nil)

	connErr := make(chan error, 1)
	go func() {
		connErr <- conn.Run(ctx)
	}()

	r := &Result{}

	if err := validateMCPInitialize(ctx, conn, r); err != nil {
		stdin.Close()
		cmd.Wait()
		return nil, fmt.Errorf("initialize: %w", err)
	}

	validateMCPToolsList(ctx, conn, r)
	validateMCPResourcesList(ctx, conn, r)
	validateMCPResourceTemplatesList(ctx, conn, r)

	conn.Close()
	stdin.Close()
	cmd.Wait()

	return r, nil
}

func validateMCPInitialize(ctx context.Context, conn *jsonrpc.Conn, r *Result) error {
	params := protocol.InitializeParamsV1{
		ProtocolVersion: "2025-03-26",
		Capabilities:    protocol.ClientCapabilitiesV1{},
		ClientInfo: protocol.ImplementationV1{
			Name:    "purse-first-validate",
			Version: "0.1.0",
		},
	}

	raw, err := conn.Call(ctx, protocol.MethodInitialize, params)
	if err != nil {
		return fmt.Errorf("calling initialize: %w", err)
	}

	var result protocol.InitializeResultV1
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("decoding initialize result: %w", err)
	}

	if result.ProtocolVersion == "" {
		r.addError("initialize", "response missing protocolVersion")
	}

	if err := conn.Notify(protocol.MethodInitialized, nil); err != nil {
		return fmt.Errorf("sending initialized notification: %w", err)
	}

	return nil
}

func validateMCPToolsList(ctx context.Context, conn *jsonrpc.Conn, r *Result) {
	raw, err := conn.Call(ctx, protocol.MethodToolsList, nil)
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
}

func validateMCPResourcesList(ctx context.Context, conn *jsonrpc.Conn, r *Result) {
	raw, err := conn.Call(ctx, protocol.MethodResourcesList, nil)
	if err != nil {
		if isMethodNotFound(err) {
			r.addInfo("resources/list", "method not supported by server")
			return
		}
		r.addError("resources/list", fmt.Sprintf("call failed: %s", err))
		return
	}

	var result protocol.ResourcesListResultV1
	if err := json.Unmarshal(raw, &result); err != nil {
		r.addError("resources/list", fmt.Sprintf("invalid response: %s", err))
	}
}

func validateMCPResourceTemplatesList(ctx context.Context, conn *jsonrpc.Conn, r *Result) {
	raw, err := conn.Call(ctx, protocol.MethodResourcesTemplates, nil)
	if err != nil {
		if isMethodNotFound(err) {
			r.addInfo("resources/templates/list", "method not supported by server")
			return
		}
		r.addError("resources/templates/list", fmt.Sprintf("call failed: %s", err))
		return
	}

	var result protocol.ResourceTemplatesListResultV1
	if err := json.Unmarshal(raw, &result); err != nil {
		r.addError("resources/templates/list", fmt.Sprintf("invalid response: %s", err))
	}
}

func isMethodNotFound(err error) bool {
	var rpcErr *jsonrpc.Error
	if errors.As(err, &rpcErr) {
		return rpcErr.Code == jsonrpc.MethodNotFound
	}
	return false
}
