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
