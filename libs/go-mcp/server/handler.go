package server

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
)

// Handler handles MCP protocol method calls.
type Handler struct {
	server            *Server
	mu                sync.RWMutex
	initialized       bool
	negotiatedVersion string
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

	// V1 methods
	case protocol.MethodCompletionComplete:
		return h.handleCompletionComplete(ctx, msg)
	case protocol.MethodLoggingSetLevel:
		return h.handleLoggingSetLevel(ctx, msg)
	case protocol.MethodTasksGet:
		return h.handleTasksGet(ctx, msg)
	case protocol.MethodTasksResult:
		return h.handleTasksResult(ctx, msg)
	case protocol.MethodTasksCancel:
		return h.handleTasksCancel(ctx, msg)
	case protocol.MethodTasksList:
		return h.handleTasksList(ctx, msg)

	// Notifications (no response)
	case protocol.MethodNotificationsProgress,
		protocol.MethodNotificationsCancelled,
		protocol.MethodNotificationsTaskStatus,
		protocol.MethodNotificationsResourcesListChanged,
		protocol.MethodNotificationsResourceUpdated,
		protocol.MethodNotificationsPromptsListChanged,
		protocol.MethodNotificationsToolsListChanged,
		protocol.MethodNotificationsRootsListChanged:
		return nil, nil

	default:
		if msg.IsRequest() {
			return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.MethodNotFound,
				"method not found: "+msg.Method, nil)
		}
		return nil, nil
	}
}

// hasV1Providers returns true if any V1-only providers are configured.
func (h *Handler) hasV1Providers() bool {
	if _, ok := h.server.opts.Tools.(ToolProviderV1); ok {
		return true
	}
	if _, ok := h.server.opts.Resources.(ResourceProviderV1); ok {
		return true
	}
	if _, ok := h.server.opts.Prompts.(PromptProviderV1); ok {
		return true
	}
	if h.server.opts.Completions != nil {
		return true
	}
	if h.server.opts.Tasks != nil {
		return true
	}
	if h.server.opts.Logging != nil {
		return true
	}
	return false
}

// isV1 returns true if V1 was negotiated.
func (h *Handler) isV1() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.negotiatedVersion == protocol.ProtocolVersionV1
}

func (h *Handler) handleInitialize(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	var params protocol.InitializeParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InvalidParams, "invalid params", nil)
	}

	h.mu.Lock()
	h.initialized = true

	// Version negotiation: if client requests V1 and we have V1 providers, negotiate V1.
	clientVersion := params.ProtocolVersion
	if clientVersion == protocol.ProtocolVersionV1 && h.hasV1Providers() {
		h.negotiatedVersion = protocol.ProtocolVersionV1
		h.mu.Unlock()
		return h.handleInitializeV1(ctx, msg)
	}

	// Fall back to V0.
	h.negotiatedVersion = protocol.ProtocolVersionV0
	h.mu.Unlock()

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
		ProtocolVersion: protocol.ProtocolVersionV0,
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

	// If V1 negotiated (or PreferV1Providers) and provider supports V1, use V1 listing.
	if h.isV1() || h.server.opts.PreferV1Providers {
		if p, ok := h.server.opts.Tools.(ToolProviderV1); ok {
			var cursor string
			if msg.Params != nil {
				var pagination protocol.PaginationParams
				if err := json.Unmarshal(msg.Params, &pagination); err == nil {
					cursor = pagination.Cursor
				}
			}
			result, err := p.ListToolsV1(ctx, cursor)
			if err != nil {
				return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, err.Error(), nil)
			}
			return jsonrpc.NewResponse(*msg.ID, result)
		}
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

	// If V1 negotiated (or PreferV1Providers) and provider supports V1, use V1 call.
	if h.isV1() || h.server.opts.PreferV1Providers {
		if p, ok := h.server.opts.Tools.(ToolProviderV1); ok {
			result, err := p.CallToolV1(ctx, params.Name, params.Arguments)
			if err != nil {
				return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, err.Error(), nil)
			}
			return jsonrpc.NewResponse(*msg.ID, result)
		}
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

	if h.isV1() || h.server.opts.PreferV1Providers {
		if p, ok := h.server.opts.Resources.(ResourceProviderV1); ok {
			var cursor string
			if msg.Params != nil {
				var pagination protocol.PaginationParams
				if err := json.Unmarshal(msg.Params, &pagination); err == nil {
					cursor = pagination.Cursor
				}
			}
			result, err := p.ListResourcesV1(ctx, cursor)
			if err != nil {
				return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, err.Error(), nil)
			}
			return jsonrpc.NewResponse(*msg.ID, result)
		}
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

	if h.isV1() || h.server.opts.PreferV1Providers {
		if p, ok := h.server.opts.Resources.(ResourceProviderV1); ok {
			var cursor string
			if msg.Params != nil {
				var pagination protocol.PaginationParams
				if err := json.Unmarshal(msg.Params, &pagination); err == nil {
					cursor = pagination.Cursor
				}
			}
			result, err := p.ListResourceTemplatesV1(ctx, cursor)
			if err != nil {
				return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, err.Error(), nil)
			}
			return jsonrpc.NewResponse(*msg.ID, result)
		}
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

	if h.isV1() || h.server.opts.PreferV1Providers {
		if p, ok := h.server.opts.Prompts.(PromptProviderV1); ok {
			var cursor string
			if msg.Params != nil {
				var pagination protocol.PaginationParams
				if err := json.Unmarshal(msg.Params, &pagination); err == nil {
					cursor = pagination.Cursor
				}
			}
			result, err := p.ListPromptsV1(ctx, cursor)
			if err != nil {
				return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, err.Error(), nil)
			}
			return jsonrpc.NewResponse(*msg.ID, result)
		}
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

	if h.isV1() || h.server.opts.PreferV1Providers {
		if p, ok := h.server.opts.Prompts.(PromptProviderV1); ok {
			result, err := p.GetPromptV1(ctx, params.Name, params.Arguments)
			if err != nil {
				return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, err.Error(), nil)
			}
			return jsonrpc.NewResponse(*msg.ID, result)
		}
	}

	result, err := h.server.opts.Prompts.GetPrompt(ctx, params.Name, params.Arguments)
	if err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, err.Error(), nil)
	}

	return jsonrpc.NewResponse(*msg.ID, result)
}
