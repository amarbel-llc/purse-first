package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/amarbel-llc/lux/internal/logfile"
	"github.com/amarbel-llc/lux/internal/lsp"
	"github.com/amarbel-llc/lux/internal/subprocess"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
)

type Handler struct {
	sessions   *SessionRegistry
	workspaces *WorkspaceRegistry
}

func NewHandler(sessions *SessionRegistry, workspaces *WorkspaceRegistry) *Handler {
	return &Handler{
		sessions:   sessions,
		workspaces: workspaces,
	}
}

func (h *Handler) Handle(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	switch msg.Method {
	case MethodSessionRegister:
		return h.handleRegister(ctx, msg)
	case MethodSessionDeregister:
		return h.handleDeregister(ctx, msg)
	case MethodLSPRequest:
		return h.handleLSPRequest(ctx, msg)
	case MethodLSPNotification:
		return h.handleLSPNotification(ctx, msg)
	case MethodPoolStatus:
		return h.handlePoolStatus(ctx, msg)
	case MethodPoolStart:
		return h.handlePoolStart(ctx, msg)
	case MethodPoolStop:
		return h.handlePoolStop(ctx, msg)
	case MethodWarmup:
		return h.handleWarmup(ctx, msg)
	default:
		return h.methodNotFound(msg)
	}
}

func (h *Handler) handleRegister(_ context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	var params RegisterParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return h.invalidParams(msg, err)
	}

	sessionID := h.sessions.Register(params.WorkspaceRoot, params.ClientType)
	h.workspaces.GetOrCreate(params.WorkspaceRoot)

	result := RegisterResult{SessionID: sessionID}
	return jsonrpc.NewResponse(*msg.ID, result)
}

func (h *Handler) handleDeregister(_ context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	var params DeregisterParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return h.invalidParams(msg, err)
	}

	h.sessions.Deregister(params.SessionID)

	return jsonrpc.NewResponse(*msg.ID, map[string]string{"status": "ok"})
}

func (h *Handler) handleLSPRequest(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	var params LSPRequestParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return h.invalidParams(msg, err)
	}

	session, ok := h.sessions.Get(params.SessionID)
	if !ok {
		return h.errorResponse(msg, jsonrpc.InvalidParams, "unknown session")
	}

	ws, ok := h.workspaces.Get(session.WorkspaceRoot)
	if !ok {
		return h.errorResponse(msg, jsonrpc.InternalError, "no workspace for session")
	}

	lspName := ws.Router.Route(params.LSPMethod, params.LSPParams)
	if lspName == "" {
		return h.errorResponse(msg, jsonrpc.InternalError, "no LSP matched for method")
	}

	fmt.Fprintf(logfile.Writer(), "[lux] request %s → %s (state: starting)\n", params.LSPMethod, lspName)

	inst, err := ws.Pool.GetOrStart(ctx, lspName, initParamsForWorkspace(ws.Root))
	if err != nil {
		fmt.Fprintf(logfile.Writer(), "[lux] request %s → %s: start failed: %v\n", params.LSPMethod, lspName, err)
		return h.errorResponse(msg, jsonrpc.InternalError, fmt.Sprintf("starting LSP %s: %v", lspName, err))
	}

	fmt.Fprintf(logfile.Writer(), "[lux] request %s → %s: forwarding\n", params.LSPMethod, lspName)

	result, err := h.callWithRetry(ctx, inst, params.LSPMethod, params.LSPParams)
	if err != nil {
		fmt.Fprintf(logfile.Writer(), "[lux] request %s → %s: failed: %v\n", params.LSPMethod, lspName, err)
		return h.errorResponse(msg, jsonrpc.InternalError, fmt.Sprintf("LSP call failed: %v", err))
	}

	fmt.Fprintf(logfile.Writer(), "[lux] request %s → %s: ok (%d bytes)\n", params.LSPMethod, lspName, len(result))
	return jsonrpc.NewResponse(*msg.ID, result)
}

func isRetryableLSPError(err error) bool {
	var rpcErr *jsonrpc.Error
	if errors.As(err, &rpcErr) {
		return rpcErr.Code == 0 && strings.Contains(rpcErr.Message, "no views")
	}
	return false
}

func (h *Handler) callWithRetry(ctx context.Context, inst *subprocess.LSPInstance, method string, params json.RawMessage) (json.RawMessage, error) {
	const maxAttempts = 8
	delay := 500 * time.Millisecond

	for attempt := 1; ; attempt++ {
		result, err := inst.Call(ctx, method, params)
		if err == nil || !isRetryableLSPError(err) || attempt >= maxAttempts {
			return result, err
		}

		fmt.Fprintf(logfile.Writer(), "[lux] retrying LSP call (attempt %d/%d, waiting %v): %v\n", attempt, maxAttempts, delay, err)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}

		delay *= 2
		if delay > 5*time.Second {
			delay = 5 * time.Second
		}
	}
}

func initParamsForWorkspace(root string) *lsp.InitializeParams {
	rootURI := lsp.URIFromPath(root)
	pid := os.Getpid()
	return &lsp.InitializeParams{
		ProcessID: &pid,
		RootURI:   &rootURI,
		RootPath:  &root,
		ClientInfo: &lsp.ClientInfo{
			Name:    "lux-daemon",
			Version: "0.1.0",
		},
		Capabilities: lsp.ClientCapabilities{
			Workspace: &lsp.WorkspaceClientCapabilities{
				WorkspaceFolders: true,
			},
			TextDocument: &lsp.TextDocumentClientCapabilities{
				Hover:          &lsp.HoverClientCaps{},
				Definition:     &lsp.DefinitionClientCaps{},
				References:     &lsp.ReferencesClientCaps{},
				Completion:     &lsp.CompletionClientCaps{},
				DocumentSymbol: &lsp.DocumentSymbolClientCaps{},
				CodeAction:     &lsp.CodeActionClientCaps{},
				Formatting:     &lsp.FormattingClientCaps{},
			},
		},
		WorkspaceFolders: []lsp.WorkspaceFolder{
			{URI: rootURI, Name: root},
		},
	}
}

func (h *Handler) handleLSPNotification(_ context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	var params LSPNotificationParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		// Notifications don't get responses, but log the error
		return nil, fmt.Errorf("invalid notification params: %w", err)
	}

	session, ok := h.sessions.Get(params.SessionID)
	if !ok {
		return nil, nil
	}

	ws, ok := h.workspaces.Get(session.WorkspaceRoot)
	if !ok {
		return nil, nil
	}

	lspName := ws.Router.Route(params.LSPMethod, params.LSPParams)
	if lspName == "" {
		return nil, nil
	}

	fmt.Fprintf(logfile.Writer(), "[lux] notification %s → %s\n", params.LSPMethod, lspName)

	inst, err := ws.Pool.GetOrStart(context.Background(), lspName, initParamsForWorkspace(ws.Root))
	if err != nil {
		fmt.Fprintf(logfile.Writer(), "[lux] notification %s → %s: start failed: %v\n", params.LSPMethod, lspName, err)
		return nil, nil
	}

	inst.Notify(params.LSPMethod, params.LSPParams)
	fmt.Fprintf(logfile.Writer(), "[lux] notification %s → %s: sent\n", params.LSPMethod, lspName)

	return nil, nil
}

type poolStatusResult struct {
	SessionCount   int                    `json:"session_count"`
	WorkspaceCount int                    `json:"workspace_count"`
	Workspaces     []workspaceStatusEntry `json:"workspaces"`
}

type workspaceStatusEntry struct {
	Root string                `json:"root"`
	LSPs []subprocess.LSPStatus `json:"lsps"`
}

func (h *Handler) handlePoolStatus(_ context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	h.workspaces.mu.RLock()
	workspaces := make([]workspaceStatusEntry, 0, len(h.workspaces.workspaces))
	for root, ws := range h.workspaces.workspaces {
		workspaces = append(workspaces, workspaceStatusEntry{
			Root: root,
			LSPs: ws.Pool.Status(),
		})
	}
	h.workspaces.mu.RUnlock()

	result := poolStatusResult{
		SessionCount:   h.sessions.ActiveCount(),
		WorkspaceCount: len(workspaces),
		Workspaces:     workspaces,
	}

	return jsonrpc.NewResponse(*msg.ID, result)
}

func (h *Handler) handlePoolStart(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	var params PoolStartParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return h.invalidParams(msg, err)
	}

	h.workspaces.mu.RLock()
	var startErrors []string
	for _, ws := range h.workspaces.workspaces {
		if _, err := ws.Pool.GetOrStart(ctx, params.Name, initParamsForWorkspace(ws.Root)); err != nil {
			startErrors = append(startErrors, fmt.Sprintf("%s: %v", ws.Root, err))
		}
	}
	h.workspaces.mu.RUnlock()

	if len(startErrors) > 0 {
		return h.errorResponse(msg, jsonrpc.InternalError, fmt.Sprintf("start errors: %v", startErrors))
	}

	return jsonrpc.NewResponse(*msg.ID, map[string]string{"status": "ok"})
}

func (h *Handler) handlePoolStop(_ context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	var params PoolStopParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return h.invalidParams(msg, err)
	}

	h.workspaces.mu.RLock()
	var stopErrors []string
	for _, ws := range h.workspaces.workspaces {
		if err := ws.Pool.Stop(params.Name); err != nil {
			stopErrors = append(stopErrors, fmt.Sprintf("%s: %v", ws.Root, err))
		}
	}
	h.workspaces.mu.RUnlock()

	if len(stopErrors) > 0 {
		return h.errorResponse(msg, jsonrpc.InternalError, fmt.Sprintf("stop errors: %v", stopErrors))
	}

	return jsonrpc.NewResponse(*msg.ID, map[string]string{"status": "ok"})
}

func (h *Handler) handleWarmup(_ context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	var params WarmupParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return h.invalidParams(msg, err)
	}

	h.workspaces.GetOrCreate(params.Dir)

	return jsonrpc.NewResponse(*msg.ID, map[string]string{"status": "ok"})
}

func (h *Handler) invalidParams(msg *jsonrpc.Message, err error) (*jsonrpc.Message, error) {
	if msg.ID == nil {
		return nil, nil
	}
	return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InvalidParams, fmt.Sprintf("invalid params: %v", err), nil)
}

func (h *Handler) errorResponse(msg *jsonrpc.Message, code int, message string) (*jsonrpc.Message, error) {
	if msg.ID == nil {
		return nil, nil
	}
	return jsonrpc.NewErrorResponse(*msg.ID, code, message, nil)
}

func (h *Handler) methodNotFound(msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	if msg.ID == nil {
		return nil, nil
	}
	return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.MethodNotFound, fmt.Sprintf("unknown method: %s", msg.Method), nil)
}
