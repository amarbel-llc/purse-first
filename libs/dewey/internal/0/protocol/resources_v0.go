package protocol

// ResourceV0 describes a resource available from the server.
// This is the V0 (2024-11-05) resource type.
type ResourceV0 struct {
	// URI uniquely identifies the resource.
	URI string `json:"uri"`

	// Name is a human-readable name.
	Name string `json:"name"`

	// Description explains what the resource provides (optional).
	Description string `json:"description,omitempty"`

	// MimeType indicates the resource content type (optional).
	MimeType string `json:"mimeType,omitempty"`
}

// Resource is a type alias for backward compatibility.
type Resource = ResourceV0

// ResourcesListResultV0 is the response to resources/list.
type ResourcesListResultV0 struct {
	Resources []Resource `json:"resources"`
}

// ResourcesListResult is a type alias for backward compatibility.
type ResourcesListResult = ResourcesListResultV0

// ResourceReadParamsV0 specifies which resource to read.
type ResourceReadParamsV0 struct {
	URI string `json:"uri"`
}

// ResourceReadParams is a type alias for backward compatibility.
type ResourceReadParams = ResourceReadParamsV0

// ResourceReadResultV0 contains the resource contents.
type ResourceReadResultV0 struct {
	Contents []ResourceContent `json:"contents"`
}

// ResourceReadResult is a type alias for backward compatibility.
type ResourceReadResult = ResourceReadResultV0

// ResourceContentV0 holds the actual resource data.
type ResourceContentV0 struct {
	// URI is the resource URI.
	URI string `json:"uri"`

	// MimeType indicates the content type (optional).
	MimeType string `json:"mimeType,omitempty"`

	// Text contains text content (mutually exclusive with Blob).
	Text string `json:"text,omitempty"`

	// Blob contains base64-encoded binary content (mutually exclusive with Text).
	Blob string `json:"blob,omitempty"`
}

// ResourceContent is a type alias for backward compatibility.
type ResourceContent = ResourceContentV0

// ResourceTemplateV0 describes a parameterized resource URI pattern.
type ResourceTemplateV0 struct {
	// URITemplate is a URI template (RFC 6570).
	URITemplate string `json:"uriTemplate"`

	// Name is a human-readable name.
	Name string `json:"name"`

	// Description explains what resources match this template (optional).
	Description string `json:"description,omitempty"`

	// MimeType indicates the resource content type (optional).
	MimeType string `json:"mimeType,omitempty"`
}

// ResourceTemplate is a type alias for backward compatibility.
type ResourceTemplate = ResourceTemplateV0

// ResourceTemplatesListResultV0 is the response to resources/templates/list.
type ResourceTemplatesListResultV0 struct {
	ResourceTemplates []ResourceTemplate `json:"resourceTemplates"`
}

// ResourceTemplatesListResult is a type alias for backward compatibility.
type ResourceTemplatesListResult = ResourceTemplatesListResultV0
