package server

// Options configures an MCP server.
type Options struct {
	// ServerName is the name of this MCP server.
	ServerName string

	// ServerVersion is the version of this MCP server (optional).
	ServerVersion string

	// Instructions provides server usage hints to clients (V1, optional).
	Instructions string

	// Tools is the tool provider (optional).
	// If nil, the server will not advertise tool capabilities.
	Tools ToolProvider

	// Resources is the resource provider (optional).
	// If nil, the server will not advertise resource capabilities.
	Resources ResourceProvider

	// Prompts is the prompt provider (optional).
	// If nil, the server will not advertise prompt capabilities.
	Prompts PromptProvider

	// Completions is the completion provider (V1, optional).
	Completions CompletionProvider

	// Tasks is the task provider (V1, optional).
	Tasks TaskProvider

	// Logging is the logging handler (V1, optional).
	Logging LoggingHandler
}
