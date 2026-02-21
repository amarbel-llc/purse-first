package protocol

// ImplementationV1 describes the server or client with V1 (2025-11-25) extensions.
type ImplementationV1 struct {
	// Name is the programmatic identifier.
	Name string `json:"name"`

	// Version is the implementation version string.
	Version string `json:"version,omitempty"`

	// Title is a human-readable display name.
	Title string `json:"title,omitempty"`

	// Description explains the implementation's purpose.
	Description string `json:"description,omitempty"`

	// Icons provides branded images for display.
	Icons []Icon `json:"icons,omitempty"`

	// WebsiteUrl is a link to the implementation's documentation.
	WebsiteUrl string `json:"websiteUrl,omitempty"`
}

// InitializeResultV1 is the V1 server response to initialization.
type InitializeResultV1 struct {
	// ProtocolVersion is the negotiated protocol version.
	ProtocolVersion string `json:"protocolVersion"`

	// Capabilities describes what the server supports.
	Capabilities ServerCapabilitiesV1 `json:"capabilities"`

	// ServerInfo describes the server implementation.
	ServerInfo ImplementationV1 `json:"serverInfo"`

	// Instructions provides server usage hints.
	Instructions string `json:"instructions,omitempty"`
}

// ServerCapabilitiesV1 describes what the server supports in V1.
type ServerCapabilitiesV1 struct {
	// Tools indicates the server supports tools.
	Tools *ToolsCapability `json:"tools,omitempty"`

	// Resources indicates the server supports resources.
	Resources *ResourcesCapability `json:"resources,omitempty"`

	// Prompts indicates the server supports prompts.
	Prompts *PromptsCapability `json:"prompts,omitempty"`

	// Logging indicates the server supports logging.
	Logging *LoggingCapability `json:"logging,omitempty"`

	// Completions indicates the server supports completions.
	Completions *CompletionsCapability `json:"completions,omitempty"`

	// Tasks indicates the server supports async tasks.
	Tasks *TasksCapability `json:"tasks,omitempty"`
}

// LoggingCapability indicates the server supports logging.
type LoggingCapability struct{}

// CompletionsCapability indicates the server supports completions.
type CompletionsCapability struct{}

// TasksCapability indicates the server supports async tasks.
type TasksCapability struct{}

// ClientCapabilitiesV1 describes what the client supports in V1.
type ClientCapabilitiesV1 struct {
	// Roots indicates client support for workspace roots.
	Roots *RootsCapability `json:"roots,omitempty"`

	// Sampling indicates client support for LLM sampling.
	Sampling *SamplingCapability `json:"sampling,omitempty"`

	// Elicitation indicates client support for elicitation.
	Elicitation *ElicitationCapability `json:"elicitation,omitempty"`

	// Tasks indicates client support for async tasks.
	Tasks *TasksCapability `json:"tasks,omitempty"`
}

// ElicitationCapability indicates client support for elicitation.
type ElicitationCapability struct{}

// InitializeParamsV1 are sent by the client during V1 initialization.
type InitializeParamsV1 struct {
	ProtocolVersion string               `json:"protocolVersion"`
	Capabilities    ClientCapabilitiesV1 `json:"capabilities"`
	ClientInfo      ImplementationV1     `json:"clientInfo"`
}
