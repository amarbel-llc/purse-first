package protocol

import "encoding/json"

// Tool describes a tool that can be invoked by the client.
type Tool struct {
	// Name is the unique identifier for the tool.
	Name string `json:"name"`

	// Description explains what the tool does (optional but recommended).
	Description string `json:"description,omitempty"`

	// InputSchema is a JSON Schema describing the tool's input parameters.
	InputSchema json.RawMessage `json:"inputSchema"`
}

// ToolsListResult is the response to tools/list.
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// ToolCallParams contains the parameters for invoking a tool.
type ToolCallParams struct {
	// Name is the tool to invoke.
	Name string `json:"name"`

	// Arguments are the JSON-encoded tool arguments.
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// ToolCallResult is the result of invoking a tool.
type ToolCallResult struct {
	// Content contains the tool's output.
	Content []ContentBlock `json:"content"`

	// IsError indicates whether the tool execution failed.
	IsError bool `json:"isError,omitempty"`
}

// ErrorResult creates a ToolCallResult representing an error.
func ErrorResult(msg string) *ToolCallResult {
	return &ToolCallResult{
		Content: []ContentBlock{TextContent(msg)},
		IsError: true,
	}
}
