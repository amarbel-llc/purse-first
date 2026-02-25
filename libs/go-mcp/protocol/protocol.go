// Package protocol defines the MCP (Model Context Protocol) types and constants.
// MCP is a protocol for communication between AI assistants and context providers.
package protocol

// Protocol version constants.
const (
	// ProtocolVersionV0 is the original MCP protocol version (2024-11-05).
	ProtocolVersionV0 = "2024-11-05"

	// ProtocolVersionV1 is the updated MCP protocol version (2025-11-25).
	ProtocolVersionV1 = "2025-11-25"

	// ProtocolVersion is the current default protocol version.
	ProtocolVersion = ProtocolVersionV0
)

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

// Implementation describes the server or client implementation.
type Implementation struct {
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
}

// PingResult is the response to a ping request.
type PingResult struct{}
