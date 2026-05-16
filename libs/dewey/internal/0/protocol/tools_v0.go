package protocol

import "encoding/json"

// ToolV0 describes a tool that can be invoked by the client.
// This is the V0 (2024-11-05) tool type.
type ToolV0 struct {
	// Name is the unique identifier for the tool.
	Name string `json:"name"`

	// Description explains what the tool does (optional but recommended).
	Description string `json:"description,omitempty"`

	// InputSchema is a JSON Schema describing the tool's input parameters.
	InputSchema json.RawMessage `json:"inputSchema"`
}

// Tool is a type alias for backward compatibility.
type Tool = ToolV0

// ToolsListResultV0 is the response to tools/list.
type ToolsListResultV0 struct {
	Tools []Tool `json:"tools"`
}

// ToolsListResult is a type alias for backward compatibility.
type ToolsListResult = ToolsListResultV0

// ToolCallParamsV0 contains the parameters for invoking a tool.
type ToolCallParamsV0 struct {
	// Name is the tool to invoke.
	Name string `json:"name"`

	// Arguments are the JSON-encoded tool arguments.
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// ToolCallParams is a type alias for backward compatibility.
type ToolCallParams = ToolCallParamsV0

// ToolCallResultV0 is the result of invoking a tool.
type ToolCallResultV0 struct {
	// Content contains the tool's output.
	Content []ContentBlock `json:"content"`

	// IsError indicates whether the tool execution failed.
	IsError bool `json:"isError,omitempty"`
}

// ToolCallResult is a type alias for backward compatibility.
type ToolCallResult = ToolCallResultV0

// ErrorResult creates a ToolCallResult representing an error.
func ErrorResult(msg string) *ToolCallResult {
	return &ToolCallResult{
		Content: []ContentBlock{TextContent(msg)},
		IsError: true,
	}
}
