package server

import (
	"context"
	"encoding/json"

	"github.com/amarbel-llc/purse-first/libs/dewey/internal/0/jsonrpc"
	"github.com/amarbel-llc/purse-first/libs/dewey/internal/0/protocol"
)

func (h *Handler) handleInitializeV1(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	capabilities := protocol.ServerCapabilitiesV1{}

	if h.server.opts.Tools != nil {
		capabilities.Tools = &protocol.ToolsCapability{}
	}
	if h.server.opts.Resources != nil {
		capabilities.Resources = &protocol.ResourcesCapability{}
	}
	if h.server.opts.Prompts != nil {
		capabilities.Prompts = &protocol.PromptsCapability{}
	}
	if h.server.opts.Logging != nil {
		capabilities.Logging = &protocol.LoggingCapability{}
	}
	if h.server.opts.Completions != nil {
		capabilities.Completions = &protocol.CompletionsCapability{}
	}
	if h.server.opts.Tasks != nil {
		capabilities.Tasks = &protocol.TasksCapability{}
	}

	result := protocol.InitializeResultV1{
		ProtocolVersion: protocol.ProtocolVersionV1,
		Capabilities:    capabilities,
		ServerInfo: protocol.ImplementationV1{
			Name:    h.server.opts.ServerName,
			Version: h.server.opts.ServerVersion,
		},
		Instructions: h.server.opts.Instructions,
	}

	return jsonrpc.NewResponse(*msg.ID, result)
}

func (h *Handler) handleCompletionComplete(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	if h.server.opts.Completions == nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, "completions not supported", nil)
	}

	var params protocol.CompletionCompleteParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InvalidParams, "invalid params", nil)
	}

	result, err := h.server.opts.Completions.Complete(ctx, params)
	if err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, err.Error(), nil)
	}

	return jsonrpc.NewResponse(*msg.ID, result)
}

func (h *Handler) handleLoggingSetLevel(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	if h.server.opts.Logging == nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, "logging not supported", nil)
	}

	var params protocol.SetLevelParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InvalidParams, "invalid params", nil)
	}

	if err := h.server.opts.Logging.SetLevel(ctx, params.Level); err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, err.Error(), nil)
	}

	return jsonrpc.NewResponse(*msg.ID, struct{}{})
}

func (h *Handler) handleTasksGet(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	if h.server.opts.Tasks == nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, "tasks not supported", nil)
	}

	var params protocol.TaskGetParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InvalidParams, "invalid params", nil)
	}

	task, err := h.server.opts.Tasks.GetTask(ctx, params.TaskId)
	if err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, err.Error(), nil)
	}

	return jsonrpc.NewResponse(*msg.ID, task)
}

func (h *Handler) handleTasksResult(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	if h.server.opts.Tasks == nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, "tasks not supported", nil)
	}

	var params protocol.TaskResultParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InvalidParams, "invalid params", nil)
	}

	result, err := h.server.opts.Tasks.GetTaskResult(ctx, params.TaskId)
	if err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, err.Error(), nil)
	}

	return jsonrpc.NewResponse(*msg.ID, result)
}

func (h *Handler) handleTasksCancel(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	if h.server.opts.Tasks == nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, "tasks not supported", nil)
	}

	var params protocol.TaskCancelParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InvalidParams, "invalid params", nil)
	}

	if err := h.server.opts.Tasks.CancelTask(ctx, params.TaskId); err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, err.Error(), nil)
	}

	return jsonrpc.NewResponse(*msg.ID, struct{}{})
}

func (h *Handler) handleTasksList(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, error) {
	if h.server.opts.Tasks == nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, "tasks not supported", nil)
	}

	result, err := h.server.opts.Tasks.ListTasks(ctx)
	if err != nil {
		return jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError, err.Error(), nil)
	}

	return jsonrpc.NewResponse(*msg.ID, result)
}
