package protocol

// InitializeParamsV0 are sent by the client during initialization.
type InitializeParamsV0 struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      Implementation     `json:"clientInfo"`
}

// InitializeParams is a type alias for backward compatibility.
type InitializeParams = InitializeParamsV0

// InitializeResultV0 is returned by the server in response to initialization.
type InitializeResultV0 struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      Implementation     `json:"serverInfo"`
}

// InitializeResult is a type alias for backward compatibility.
type InitializeResult = InitializeResultV0

// ClientCapabilitiesV0 describes what the client supports.
type ClientCapabilitiesV0 struct {
	Roots    *RootsCapability    `json:"roots,omitempty"`
	Sampling *SamplingCapability `json:"sampling,omitempty"`
}

// ClientCapabilities is a type alias for backward compatibility.
type ClientCapabilities = ClientCapabilitiesV0

// RootsCapability indicates client support for workspace roots.
type RootsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// SamplingCapability indicates client support for LLM sampling.
type SamplingCapability struct{}

// ServerCapabilitiesV0 describes what the server supports.
type ServerCapabilitiesV0 struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
}

// ServerCapabilities is a type alias for backward compatibility.
type ServerCapabilities = ServerCapabilitiesV0

// ToolsCapability indicates the server supports tools.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability indicates the server supports resources.
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptsCapability indicates the server supports prompts.
type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}
