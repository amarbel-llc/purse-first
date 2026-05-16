package command

// Result holds the output of a command handler, used by both CLI and MCP runners.
type Result struct {
	Text  string // plain text output
	JSON  any    // structured output (marshaled to JSON for display)
	IsErr bool   // marks this result as an error for MCP
}

// TextResult creates a Result with plain text.
func TextResult(text string) *Result {
	return &Result{Text: text}
}

// JSONResult creates a Result with structured data.
func JSONResult(v any) *Result {
	return &Result{JSON: v}
}

// TextErrorResult creates an error Result with plain text.
func TextErrorResult(text string) *Result {
	return &Result{Text: text, IsErr: true}
}
