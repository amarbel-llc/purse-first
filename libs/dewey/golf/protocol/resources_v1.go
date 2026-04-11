package protocol

// ResourceV1 describes a resource with V1 (2025-11-25) extensions.
type ResourceV1 struct {
	// URI uniquely identifies the resource.
	URI string `json:"uri"`

	// Name is a human-readable name.
	Name string `json:"name"`

	// Title is a human-readable display name.
	Title string `json:"title,omitempty"`

	// Description explains what the resource provides.
	Description string `json:"description,omitempty"`

	// MimeType indicates the resource content type.
	MimeType string `json:"mimeType,omitempty"`

	// Size indicates the resource size in bytes.
	Size *int64 `json:"size,omitempty"`

	// Icons provides visual icons for display in user interfaces.
	Icons []Icon `json:"icons,omitempty"`

	// Annotations provides metadata about this resource.
	Annotations *ContentAnnotations `json:"annotations,omitempty"`
}

// ResourceTemplateV1 describes a parameterized resource URI pattern with V1 extensions.
type ResourceTemplateV1 struct {
	// URITemplate is a URI template (RFC 6570).
	URITemplate string `json:"uriTemplate"`

	// Name is a human-readable name.
	Name string `json:"name"`

	// Title is a human-readable display name.
	Title string `json:"title,omitempty"`

	// Description explains what resources match this template.
	Description string `json:"description,omitempty"`

	// MimeType indicates the resource content type.
	MimeType string `json:"mimeType,omitempty"`

	// Icons provides visual icons for display in user interfaces.
	Icons []Icon `json:"icons,omitempty"`

	// Annotations provides metadata about this resource template.
	Annotations *ContentAnnotations `json:"annotations,omitempty"`
}

// ResourcesListResultV1 is the V1 response to resources/list with pagination.
type ResourcesListResultV1 struct {
	Resources  []ResourceV1 `json:"resources"`
	NextCursor string       `json:"nextCursor,omitempty"`
}

// ResourceTemplatesListResultV1 is the V1 response to resources/templates/list with pagination.
type ResourceTemplatesListResultV1 struct {
	ResourceTemplates []ResourceTemplateV1 `json:"resourceTemplates"`
	NextCursor        string               `json:"nextCursor,omitempty"`
}
