package server

import (
	"context"
	"encoding/json"

	"github.com/amarbel-llc/go-lib-mcp/jsonrpc"
	"github.com/amarbel-llc/go-lib-mcp/protocol"
)

// Handler handles MCP protocol method calls.
type Handler struct {
	server      *Server
	initialized bool
}

// NewHandler creates a new handler for the given server.
func NewHandler(s *Server) *Handler {
	return &Handler{server: s}
}

// Handle dispatches an incoming message to the appropriate handler method.
func (h *Handler) Handle(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	switch msg.Method {
	case protocol.MethodInitialize:
		return h.handleInitialize(ctx, msg)
	case protocol.MethodInitialized:
		return nil, nil // Notification, no response
	case protocol.MethodPing:
		return h.handlePing(ctx, msg)
	case protocol.MethodToolsList:
		return h.handleToolsList(ctx, msg)
	case protocol.MethodToolsCall:
		return h.handleToolsCall(ctx, msg)
	case protocol.MethodResourcesList:
		return h.handleResourcesList(ctx, msg)
	case protocol.MethodResourcesRead:
		return h.handleResourcesRead(ctx, msg)
	case protocol.MethodResourcesTemplates:
		return h.handleResourcesTemplates(ctx, msg)
	case protocol.MethodPromptsList:
		return h.handlePromptsList(ctx, msg)
	case protocol.MethodPromptsGet:
		return h.handlePromptsGet(ctx, msg)
	default:
		if msg.IsRequest() {
			return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.MethodNotFound,
				"method not found: "+msg.Method, nil)
		}
		return nil, nil
	}
}

func (h *Handler) handleInitialize(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	var params protocol.InitializeParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InvalidParams, "invalid params", nil)
	}

	h.initialized = true

	capabilities := protocol.ServerCapabilities{}
	if h.server.opts.Tools != nil {
		capabilities.Tools = &protocol.ToolsCapability{}
	}
	if h.server.opts.Resources != nil {
		capabilities.Resources = &protocol.ResourcesCapability{}
	}
	if h.server.opts.Prompts != nil {
		capabilities.Prompts = &protocol.PromptsCapability{}
	}

	result := protocol.InitializeResult{
		ProtocolVersion: protocol.ProtocolVersion,
		Capabilities:    capabilities,
		ServerInfo: protocol.Implementation{
			Name:    h.server.opts.ServerName,
			Version: h.server.opts.ServerVersion,
		},
	}

	return jsonrpc.NewResponse(*msg.ID, result)
}

func (h *Handler) handlePing(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	return jsonrpc.NewResponse(*msg.ID, protocol.PingResult{})
}

func (h *Handler) handleToolsList(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	if h.server.opts.Tools == nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, "tools not supported", nil)
	}

	tools, err := h.server.opts.Tools.ListTools(ctx)
	if err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, err.Error(), nil)
	}

	result := protocol.ToolsListResult{Tools: tools}
	return jsonrpc.NewResponse(*msg.ID, result)
}

func (h *Handler) handleToolsCall(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	if h.server.opts.Tools == nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, "tools not supported", nil)
	}

	var params protocol.ToolCallParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InvalidParams, "invalid params", nil)
	}

	result, err := h.server.opts.Tools.CallTool(ctx, params.Name, params.Arguments)
	if err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, err.Error(), nil)
	}

	return jsonrpc.NewResponse(*msg.ID, result)
}

func (h *Handler) handleResourcesList(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	if h.server.opts.Resources == nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, "resources not supported", nil)
	}

	resources, err := h.server.opts.Resources.ListResources(ctx)
	if err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, err.Error(), nil)
	}

	result := protocol.ResourcesListResult{Resources: resources}
	return jsonrpc.NewResponse(*msg.ID, result)
}

func (h *Handler) handleResourcesRead(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	if h.server.opts.Resources == nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, "resources not supported", nil)
	}

	var params protocol.ResourceReadParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InvalidParams, "invalid params", nil)
	}

	result, err := h.server.opts.Resources.ReadResource(ctx, params.URI)
	if err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, err.Error(), nil)
	}

	return jsonrpc.NewResponse(*msg.ID, result)
}

func (h *Handler) handleResourcesTemplates(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	if h.server.opts.Resources == nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, "resources not supported", nil)
	}

	templates, err := h.server.opts.Resources.ListResourceTemplates(ctx)
	if err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, err.Error(), nil)
	}

	result := protocol.ResourceTemplatesListResult{ResourceTemplates: templates}
	return jsonrpc.NewResponse(*msg.ID, result)
}

func (h *Handler) handlePromptsList(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	if h.server.opts.Prompts == nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, "prompts not supported", nil)
	}

	prompts, err := h.server.opts.Prompts.ListPrompts(ctx)
	if err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, err.Error(), nil)
	}

	result := protocol.PromptsListResult{Prompts: prompts}
	return jsonrpc.NewResponse(*msg.ID, result)
}

func (h *Handler) handlePromptsGet(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	if h.server.opts.Prompts == nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, "prompts not supported", nil)
	}

	var params protocol.PromptGetParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InvalidParams, "invalid params", nil)
	}

	result, err := h.server.opts.Prompts.GetPrompt(ctx, params.Name, params.Arguments)
	if err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, err.Error(), nil)
	}

	return jsonrpc.NewResponse(*msg.ID, result)
}
