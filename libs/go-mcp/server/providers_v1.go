package server

import (
	"context"
	"encoding/json"

	"code.linenisgreat.com/purse-first/libs/go-mcp/protocol"
)

// ToolProviderV1 extends ToolProvider with V1 capabilities.
// Implementations that satisfy this interface will be used for V1 clients.
type ToolProviderV1 interface {
	ToolProvider

	// ListToolsV1 returns all available tools with V1 metadata.
	ListToolsV1(ctx context.Context, cursor string) (*protocol.ToolsListResultV1, error)

	// CallToolV1 invokes a tool and returns a V1 result.
	CallToolV1(ctx context.Context, name string, args json.RawMessage) (*protocol.ToolCallResultV1, error)
}

// ResourceProviderV1 extends ResourceProvider with V1 capabilities.
type ResourceProviderV1 interface {
	ResourceProvider

	// ListResourcesV1 returns all available resources with V1 metadata.
	ListResourcesV1(ctx context.Context, cursor string) (*protocol.ResourcesListResultV1, error)

	// ListResourceTemplatesV1 returns URI templates with V1 metadata.
	ListResourceTemplatesV1(ctx context.Context, cursor string) (*protocol.ResourceTemplatesListResultV1, error)
}

// PromptProviderV1 extends PromptProvider with V1 capabilities.
type PromptProviderV1 interface {
	PromptProvider

	// ListPromptsV1 returns all available prompts with V1 metadata.
	ListPromptsV1(ctx context.Context, cursor string) (*protocol.PromptsListResultV1, error)

	// GetPromptV1 retrieves and renders a prompt with V1 content.
	GetPromptV1(ctx context.Context, name string, args map[string]string) (*protocol.PromptGetResultV1, error)
}

// CompletionProvider provides argument completion suggestions.
type CompletionProvider interface {
	// Complete returns completion suggestions for a reference and argument.
	Complete(ctx context.Context, params protocol.CompletionCompleteParams) (*protocol.CompletionResult, error)
}

// TaskProvider manages async task lifecycle.
type TaskProvider interface {
	// GetTask retrieves a task by ID.
	GetTask(ctx context.Context, taskId string) (*protocol.Task, error)

	// GetTaskResult retrieves a task's result.
	GetTaskResult(ctx context.Context, taskId string) (json.RawMessage, error)

	// CancelTask cancels a running task.
	CancelTask(ctx context.Context, taskId string) error

	// ListTasks returns all tasks.
	ListTasks(ctx context.Context) (*protocol.TaskListResult, error)
}

// ElicitationProvider handles user information requests.
type ElicitationProvider interface {
	// Elicit requests information from the user via the client.
	Elicit(ctx context.Context, params protocol.ElicitRequestParams) (*protocol.ElicitResult, error)
}

// LoggingHandler handles logging level changes.
type LoggingHandler interface {
	// SetLevel sets the server's logging level.
	SetLevel(ctx context.Context, level protocol.LoggingLevel) error
}
