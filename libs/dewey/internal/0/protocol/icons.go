package protocol

// Icon represents a visual icon for display in user interfaces.
type Icon struct {
	// Src is the URI of the icon image.
	Src string `json:"src"`

	// MimeType overrides the MIME type of the icon.
	MimeType string `json:"mimeType,omitempty"`

	// Sizes indicates available sizes in "WxH" format or "any".
	Sizes []string `json:"sizes,omitempty"`

	// Theme indicates the intended display theme ("light" or "dark").
	Theme string `json:"theme,omitempty"`
}
