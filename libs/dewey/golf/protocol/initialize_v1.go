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

// TasksCapability indicates support for task-augmented requests.
type TasksCapability struct {
	// List indicates support for the tasks/list operation.
	List *TasksListCapability `json:"list,omitempty"`

	// Cancel indicates support for the tasks/cancel operation.
	Cancel *TasksCancelCapability `json:"cancel,omitempty"`

	// Requests indicates which request types support task augmentation.
	Requests *TasksRequestsCapability `json:"requests,omitempty"`
}

// TasksListCapability indicates support for tasks/list.
type TasksListCapability struct{}

// TasksCancelCapability indicates support for tasks/cancel.
type TasksCancelCapability struct{}

// TasksRequestsCapability indicates which request types support task augmentation.
type TasksRequestsCapability struct {
	// Tools indicates which tool operations support tasks.
	Tools *TasksToolsCapability `json:"tools,omitempty"`

	// Sampling indicates which sampling operations support tasks (client-side).
	Sampling *TasksSamplingCapability `json:"sampling,omitempty"`

	// Elicitation indicates which elicitation operations support tasks (client-side).
	Elicitation *TasksElicitationCapability `json:"elicitation,omitempty"`
}

// TasksToolsCapability indicates which tool operations support tasks.
type TasksToolsCapability struct {
	// Call indicates tools/call supports task augmentation.
	Call *TasksCallCapability `json:"call,omitempty"`
}

// TasksSamplingCapability indicates which sampling operations support tasks.
type TasksSamplingCapability struct {
	// CreateMessage indicates sampling/createMessage supports task augmentation.
	CreateMessage *TasksCreateMessageCapability `json:"createMessage,omitempty"`
}

// TasksElicitationCapability indicates which elicitation operations support tasks.
type TasksElicitationCapability struct {
	// Create indicates elicitation/create supports task augmentation.
	Create *TasksCreateCapability `json:"create,omitempty"`
}

// TasksCallCapability marker for tools/call task support.
type TasksCallCapability struct{}

// TasksCreateMessageCapability marker for sampling/createMessage task support.
type TasksCreateMessageCapability struct{}

// TasksCreateCapability marker for elicitation/create task support.
type TasksCreateCapability struct{}

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
// An empty object is equivalent to form-only support for backwards compatibility.
type ElicitationCapability struct {
	// Form indicates support for form-based elicitation.
	Form *ElicitationFormCapability `json:"form,omitempty"`

	// URL indicates support for URL-based elicitation.
	URL *ElicitationURLCapability `json:"url,omitempty"`
}

// ElicitationFormCapability indicates support for form-based elicitation.
type ElicitationFormCapability struct{}

// ElicitationURLCapability indicates support for URL-based elicitation.
type ElicitationURLCapability struct{}

// InitializeParamsV1 are sent by the client during V1 initialization.
type InitializeParamsV1 struct {
	ProtocolVersion string               `json:"protocolVersion"`
	Capabilities    ClientCapabilitiesV1 `json:"capabilities"`
	ClientInfo      ImplementationV1     `json:"clientInfo"`
	Meta            map[string]any       `json:"_meta,omitempty"`
}
