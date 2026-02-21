package protocol

// ContentBlockV0 represents a piece of content in a tool response or prompt message.
// This is the V0 (2024-11-05) content block type.
type ContentBlockV0 struct {
	// Type is the content type (e.g., "text", "image", "resource").
	Type string `json:"type"`

	// Text is the text content (for type="text").
	Text string `json:"text"`

	// MimeType is the MIME type for non-text content.
	MimeType string `json:"mimeType,omitempty"`

	// Data is base64-encoded binary data (for type="blob").
	Data string `json:"data,omitempty"`
}

// ContentBlock is a type alias for backward compatibility.
type ContentBlock = ContentBlockV0

// TextContent creates a ContentBlock containing plain text.
func TextContent(text string) ContentBlock {
	return ContentBlock{Type: "text", Text: text}
}
