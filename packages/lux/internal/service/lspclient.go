package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
	"github.com/amarbel-llc/lux/internal/lsp"
)

type LSPClient struct {
	socketPath    string
	workspaceRoot string
	sessionID     string
	serviceConn   *jsonrpc.Conn
	clientConn    *jsonrpc.Conn
}

func NewLSPClient(socketPath, workspaceRoot string) *LSPClient {
	return &LSPClient{
		socketPath:    socketPath,
		workspaceRoot: workspaceRoot,
	}
}

func (c *LSPClient) Run(ctx context.Context) error {
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return fmt.Errorf("connecting to service: %w", err)
	}
	defer conn.Close()

	c.serviceConn = jsonrpc.NewConn(conn, conn, c.handleServiceMessage)

	go c.serviceConn.Run(ctx)

	sessionID, err := c.registerSession(ctx)
	if err != nil {
		return fmt.Errorf("registering session: %w", err)
	}
	c.sessionID = sessionID

	c.clientConn = jsonrpc.NewConn(os.Stdin, os.Stdout, c.handleClientMessage)

	err = c.clientConn.Run(ctx)

	c.Close(ctx)

	return err
}

func (c *LSPClient) registerSession(ctx context.Context) (string, error) {
	result, err := c.serviceConn.Call(ctx, MethodSessionRegister, RegisterParams{
		WorkspaceRoot: c.workspaceRoot,
		ClientType:    ClientTypeLSP,
	})
	if err != nil {
		return "", fmt.Errorf("register call: %w", err)
	}

	var reg RegisterResult
	if err := json.Unmarshal(result, &reg); err != nil {
		return "", fmt.Errorf("unmarshaling register result: %w", err)
	}

	return reg.SessionID, nil
}

func (c *LSPClient) handleClientMessage(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	switch msg.Method {
	case lsp.MethodInitialize:
		return c.handleInitialize(msg)
	case lsp.MethodInitialized, lsp.MethodExit:
		return nil, nil
	case lsp.MethodShutdown:
		return jsonrpc.NewResponse(*msg.ID, nil)
	}

	if strings.HasPrefix(msg.Method, "$/") {
		return nil, nil
	}

	if msg.IsRequest() {
		return c.proxyRequestToService(ctx, msg)
	}

	if msg.IsNotification() {
		return c.proxyNotificationToService(ctx, msg)
	}

	return nil, nil
}

func (c *LSPClient) handleInitialize(msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	result := lsp.InitializeResult{
		Capabilities: lsp.ServerCapabilities{
			TextDocumentSync: 1,
			HoverProvider:    true,
			CompletionProvider: &lsp.CompletionOptions{
				TriggerCharacters: []string{"."},
			},
			DefinitionProvider:              true,
			TypeDefinitionProvider:          true,
			ImplementationProvider:          true,
			ReferencesProvider:              true,
			DocumentSymbolProvider:          true,
			CodeActionProvider:              true,
			DocumentFormattingProvider:      true,
			DocumentRangeFormattingProvider: true,
			RenameProvider:                  true,
			FoldingRangeProvider:            true,
			SelectionRangeProvider:          true,
			WorkspaceSymbolProvider:         true,
		},
		ServerInfo: &lsp.ServerInfo{
			Name:    "lux",
			Version: "0.1.0",
		},
	}

	return jsonrpc.NewResponse(*msg.ID, result)
}

func (c *LSPClient) proxyRequestToService(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	wrapped := c.wrapRequest(msg.Method, msg.Params)

	result, err := c.serviceConn.Call(ctx, MethodLSPRequest, wrapped)
	if err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, err.Error(), nil)
	}

	return jsonrpc.NewResponse(*msg.ID, json.RawMessage(result))
}

func (c *LSPClient) proxyNotificationToService(_ context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	wrapped := c.wrapNotification(msg.Method, msg.Params)

	c.serviceConn.Notify(MethodLSPNotification, wrapped)

	return nil, nil
}

func (c *LSPClient) handleServiceMessage(_ context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	if msg.Method != MethodLSPNotification {
		return nil, nil
	}

	var notification LSPNotificationParams
	if err := json.Unmarshal(msg.Params, &notification); err != nil {
		return nil, nil
	}

	c.clientConn.Notify(notification.LSPMethod, json.RawMessage(notification.LSPParams))

	return nil, nil
}

func (c *LSPClient) wrapRequest(method string, params json.RawMessage) LSPRequestParams {
	return LSPRequestParams{
		SessionID: c.sessionID,
		LSPMethod: method,
		LSPParams: params,
	}
}

func (c *LSPClient) wrapNotification(method string, params json.RawMessage) LSPNotificationParams {
	return LSPNotificationParams{
		SessionID: c.sessionID,
		LSPMethod: method,
		LSPParams: params,
	}
}

func (c *LSPClient) Close(ctx context.Context) {
	if c.sessionID == "" || c.serviceConn == nil {
		return
	}

	c.serviceConn.Call(ctx, MethodSessionDeregister, DeregisterParams{
		SessionID: c.sessionID,
	})

	c.sessionID = ""
}
