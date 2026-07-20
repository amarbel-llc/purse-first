package command

import "code.linenisgreat.com/purse-first/libs/go-mcp/protocol"

// Result holds the output of a command handler, used by both CLI and MCP runners.
//
// When Content is non-empty it takes precedence over Text/JSON for V1 MCP
// transport: the blocks are forwarded verbatim. Text/JSON remain the simple
// path for the common case where a tool only emits plain or structured text.
type Result struct {
	Text    string                    // plain text output
	JSON    any                       // structured output (marshaled to JSON for display)
	Content []protocol.ContentBlockV1 // rich content blocks; takes precedence over Text/JSON when non-empty
	IsErr   bool                      // marks this result as an error for MCP
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

// ResourceLinkResult creates a Result containing a single resource_link
// content block. Use this to point an agent at an out-of-band resource
// (e.g. a blob in a content-addressable store) without inlining its bytes.
func ResourceLinkResult(uri, name, description string) *Result {
	return &Result{
		Content: []protocol.ContentBlockV1{
			protocol.ResourceLinkContent(uri, name, description, ""),
		},
	}
}

// MultiContentResult creates a Result carrying arbitrary V1 content blocks.
// Use this to mix text with resource_link or embedded-resource blocks.
func MultiContentResult(blocks ...protocol.ContentBlockV1) *Result {
	return &Result{Content: blocks}
}
