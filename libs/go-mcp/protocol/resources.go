package protocol

// Resource describes a resource available from the server.
type Resource struct {
	// URI uniquely identifies the resource.
	URI string `json:"uri"`

	// Name is a human-readable name.
	Name string `json:"name"`

	// Description explains what the resource provides (optional).
	Description string `json:"description,omitempty"`

	// MimeType indicates the resource content type (optional).
	MimeType string `json:"mimeType,omitempty"`
}

// ResourcesListResult is the response to resources/list.
type ResourcesListResult struct {
	Resources []Resource `json:"resources"`
}

// ResourceReadParams specifies which resource to read.
type ResourceReadParams struct {
	URI string `json:"uri"`
}

// ResourceReadResult contains the resource contents.
type ResourceReadResult struct {
	Contents []ResourceContent `json:"contents"`
}

// ResourceContent holds the actual resource data.
type ResourceContent struct {
	// URI is the resource URI.
	URI string `json:"uri"`

	// MimeType indicates the content type (optional).
	MimeType string `json:"mimeType,omitempty"`

	// Text contains text content (mutually exclusive with Blob).
	Text string `json:"text,omitempty"`

	// Blob contains base64-encoded binary content (mutually exclusive with Text).
	Blob string `json:"blob,omitempty"`
}

// ResourceTemplate describes a parameterized resource URI pattern.
type ResourceTemplate struct {
	// URITemplate is a URI template (RFC 6570).
	URITemplate string `json:"uriTemplate"`

	// Name is a human-readable name.
	Name string `json:"name"`

	// Description explains what resources match this template (optional).
	Description string `json:"description,omitempty"`

	// MimeType indicates the resource content type (optional).
	MimeType string `json:"mimeType,omitempty"`
}

// ResourceTemplatesListResult is the response to resources/templates/list.
type ResourceTemplatesListResult struct {
	ResourceTemplates []ResourceTemplate `json:"resourceTemplates"`
}
