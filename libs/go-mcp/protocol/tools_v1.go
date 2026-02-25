package protocol

import "encoding/json"

// ToolAnnotations provides hints about tool behavior for clients.
type ToolAnnotations struct {
	// Title is a human-readable display name for the tool.
	Title string `json:"title,omitempty"`

	// ReadOnlyHint indicates the tool does not modify state.
	ReadOnlyHint *bool `json:"readOnlyHint,omitempty"`

	// DestructiveHint indicates the tool may perform destructive operations.
	DestructiveHint *bool `json:"destructiveHint,omitempty"`

	// IdempotentHint indicates repeated calls with same args have no additional effect.
	IdempotentHint *bool `json:"idempotentHint,omitempty"`

	// OpenWorldHint indicates the tool interacts with external entities.
	OpenWorldHint *bool `json:"openWorldHint,omitempty"`
}

// TaskSupport values for tool execution.
const (
	// TaskSupportRequired means clients must invoke the tool as a task.
	TaskSupportRequired = "required"

	// TaskSupportOptional means clients may invoke the tool as a task or normally.
	TaskSupportOptional = "optional"

	// TaskSupportForbidden means clients must not invoke the tool as a task.
	TaskSupportForbidden = "forbidden"
)

// ToolExecution describes task execution support for a tool.
type ToolExecution struct {
	// TaskSupport indicates whether the tool supports task-augmented invocation.
	// Values: "required", "optional", "forbidden".
	// If absent, defaults to "forbidden".
	TaskSupport string `json:"taskSupport,omitempty"`
}

// ToolV1 describes a tool with V1 (2025-11-25) extensions.
type ToolV1 struct {
	// Name is the unique identifier for the tool.
	Name string `json:"name"`

	// Title is a human-readable display name for the tool.
	Title string `json:"title,omitempty"`

	// Description explains what the tool does.
	Description string `json:"description,omitempty"`

	// Icons provides visual icons for display in user interfaces.
	Icons []Icon `json:"icons,omitempty"`

	// InputSchema is a JSON Schema describing the tool's input parameters.
	InputSchema json.RawMessage `json:"inputSchema"`

	// OutputSchema is a JSON Schema describing the tool's output structure.
	OutputSchema json.RawMessage `json:"outputSchema,omitempty"`

	// Annotations provides hints about tool behavior.
	Annotations *ToolAnnotations `json:"annotations,omitempty"`

	// Execution describes task execution support for this tool.
	Execution *ToolExecution `json:"execution,omitempty"`
}

// ToolsListResultV1 is the V1 response to tools/list with pagination.
type ToolsListResultV1 struct {
	Tools      []ToolV1 `json:"tools"`
	NextCursor string   `json:"nextCursor,omitempty"`
}

// ToolCallResultV1 is the V1 result of invoking a tool.
type ToolCallResultV1 struct {
	// Content contains the tool's unstructured output.
	Content []ContentBlockV1 `json:"content,omitempty"`

	// StructuredContent contains the tool's structured output.
	StructuredContent json.RawMessage `json:"structuredContent,omitempty"`

	// IsError indicates whether the tool execution failed.
	IsError bool `json:"isError,omitempty"`

	// Meta contains protocol-level metadata (e.g., related-task associations).
	Meta map[string]any `json:"_meta,omitempty"`
}

// ErrorResultV1 creates a ToolCallResultV1 representing an error.
func ErrorResultV1(msg string) *ToolCallResultV1 {
	return &ToolCallResultV1{
		Content: []ContentBlockV1{TextContentV1(msg)},
		IsError: true,
	}
}

// BoolPtr returns a pointer to b, for use with ToolAnnotations hint fields.
func BoolPtr(b bool) *bool { return &b }
