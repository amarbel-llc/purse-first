package protocol

// ContentAnnotations provides metadata about content blocks.
type ContentAnnotations struct {
	// Audience indicates the intended recipients of the content.
	Audience []string `json:"audience,omitempty"`

	// Priority indicates the importance level (0.0 to 1.0).
	Priority *float64 `json:"priority,omitempty"`

	// LastModified is the ISO 8601 timestamp of the last modification.
	LastModified string `json:"lastModified,omitempty"`
}

// TextResourceContents holds text content for an EmbeddedResource content block.
// Per MCP spec: required uri + text, optional mimeType.
type TextResourceContents struct {
	URI      string `json:"uri"`
	Text     string `json:"text"`
	MimeType string `json:"mimeType,omitempty"`
}

// BlobResourceContents holds base64-encoded binary content for an EmbeddedResource content block.
// Per MCP spec: required uri + blob, optional mimeType.
type BlobResourceContents struct {
	URI      string `json:"uri"`
	Blob     string `json:"blob"`
	MimeType string `json:"mimeType,omitempty"`
}

// EmbeddedResourceContents is the union of TextResourceContents and BlobResourceContents.
// Exactly one of Text or Blob should be populated.
type EmbeddedResourceContents struct {
	URI      string `json:"uri"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// ContentBlockV1 represents a piece of content with optional annotations.
// This is the V1 (2025-11-25) content block type.
type ContentBlockV1 struct {
	// Type is the content type (e.g., "text", "image", "audio", "resource", "resource_link").
	Type string `json:"type"`

	// Text is the text content (for type="text").
	Text string `json:"text,omitempty"`

	// MimeType is the MIME type for non-text content.
	MimeType string `json:"mimeType,omitempty"`

	// Data is base64-encoded binary data (for type="image", "audio").
	Data string `json:"data,omitempty"`

	// Resource contains embedded resource data (for type="resource").
	// Use EmbeddedTextResourceContent or EmbeddedBlobResourceContent helpers to construct.
	Resource *EmbeddedResourceContents `json:"resource,omitempty"`

	// URI is the resource URI (for type="resource_link").
	URI string `json:"uri,omitempty"`

	// Name is a human-readable name (for type="resource_link").
	Name string `json:"name,omitempty"`

	// Description is a human-readable description (for type="resource_link").
	Description string `json:"description,omitempty"`

	// Annotations provides metadata about this content block.
	Annotations *ContentAnnotations `json:"annotations,omitempty"`
}

// AudioContent creates a ContentBlockV1 containing audio data.
func AudioContent(data, mimeType string) ContentBlockV1 {
	return ContentBlockV1{Type: "audio", Data: data, MimeType: mimeType}
}

// ResourceLinkContent creates a ContentBlockV1 containing a resource link.
func ResourceLinkContent(uri, name, description, mimeType string) ContentBlockV1 {
	return ContentBlockV1{
		Type:        "resource_link",
		URI:         uri,
		Name:        name,
		Description: description,
		MimeType:    mimeType,
	}
}

// EmbeddedTextResourceContent creates a ContentBlockV1 containing an embedded text resource.
func EmbeddedTextResourceContent(uri, text, mimeType string) ContentBlockV1 {
	return ContentBlockV1{
		Type: "resource",
		Resource: &EmbeddedResourceContents{
			URI:      uri,
			Text:     text,
			MimeType: mimeType,
		},
	}
}

// EmbeddedBlobResourceContent creates a ContentBlockV1 containing an embedded blob resource.
func EmbeddedBlobResourceContent(uri, blob, mimeType string) ContentBlockV1 {
	return ContentBlockV1{
		Type: "resource",
		Resource: &EmbeddedResourceContents{
			URI:      uri,
			Blob:     blob,
			MimeType: mimeType,
		},
	}
}

// TextContentV1 creates a ContentBlockV1 containing plain text.
func TextContentV1(text string) ContentBlockV1 {
	return ContentBlockV1{Type: "text", Text: text}
}

// ImageContentV1 creates a ContentBlockV1 containing image data.
func ImageContentV1(data, mimeType string) ContentBlockV1 {
	return ContentBlockV1{Type: "image", Data: data, MimeType: mimeType}
}
