// Package protocol defines the MCP (Model Context Protocol) types and constants.
// MCP is a protocol for communication between AI assistants and context providers.
package protocol

// ProtocolVersion is the MCP protocol version this library implements.
const ProtocolVersion = "2024-11-05"

// MCP method name constants define the available protocol methods.
const (
	// MethodInitialize is sent by the client to initialize the connection.
	MethodInitialize = "initialize"

	// MethodInitialized is a notification from client confirming initialization.
	MethodInitialized = "notifications/initialized"

	// MethodPing is used to check if the server is alive.
	MethodPing = "ping"

	// MethodToolsList requests the list of available tools.
	MethodToolsList = "tools/list"

	// MethodToolsCall invokes a tool with arguments.
	MethodToolsCall = "tools/call"

	// MethodResourcesList requests the list of available resources.
	MethodResourcesList = "resources/list"

	// MethodResourcesRead reads the content of a resource.
	MethodResourcesRead = "resources/read"

	// MethodResourcesTemplates lists resource URI templates.
	MethodResourcesTemplates = "resources/templates/list"

	// MethodPromptsList requests the list of available prompts.
	MethodPromptsList = "prompts/list"

	// MethodPromptsGet retrieves a prompt with arguments.
	MethodPromptsGet = "prompts/get"
)

// ContentBlock represents a piece of content in a tool response or prompt message.
type ContentBlock struct {
	// Type is the content type (e.g., "text", "image", "resource").
	Type string `json:"type"`

	// Text is the text content (for type="text").
	Text string `json:"text"`

	// MimeType is the MIME type for non-text content.
	MimeType string `json:"mimeType,omitempty"`

	// Data is base64-encoded binary data (for type="blob").
	Data string `json:"data,omitempty"`
}

// TextContent creates a ContentBlock containing plain text.
func TextContent(text string) ContentBlock {
	return ContentBlock{Type: "text", Text: text}
}

// Implementation describes the server or client implementation.
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// PingResult is the response to a ping request.
type PingResult struct{}
