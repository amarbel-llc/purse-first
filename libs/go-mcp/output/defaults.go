package output

// Defaults holds standard limits applied when user-supplied limits are zero.
type Defaults struct {
	MaxBytes int `json:"max_bytes"`
	MaxLines int `json:"max_lines"`
	MaxItems int `json:"max_items"`
}

// StandardDefaults returns the standard default values matching nix-mcp-server.
func StandardDefaults() Defaults {
	return Defaults{
		MaxBytes: 100_000,
		MaxLines: 2000,
		MaxItems: 100,
	}
}

// MergeTextLimits fills zero-valued MaxBytes and MaxLines from the defaults.
// Head and Tail are never defaulted.
func (d Defaults) MergeTextLimits(user TextLimits) TextLimits {
	if user.MaxBytes == 0 {
		user.MaxBytes = d.MaxBytes
	}

	if user.MaxLines == 0 {
		user.MaxLines = d.MaxLines
	}

	return user
}

// MergeArrayLimits fills a zero-valued Limit from MaxItems.
// Offset is never defaulted.
func (d Defaults) MergeArrayLimits(user ArrayLimits) ArrayLimits {
	if user.Limit == 0 {
		user.Limit = d.MaxItems
	}

	return user
}
