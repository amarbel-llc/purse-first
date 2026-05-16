package protocol

// PromptV1 describes a prompt template with V1 (2025-11-25) extensions.
type PromptV1 struct {
	// Name uniquely identifies the prompt.
	Name string `json:"name"`

	// Title is a human-readable display name.
	Title string `json:"title,omitempty"`

	// Description explains what the prompt does.
	Description string `json:"description,omitempty"`

	// Icons provides visual icons for display in user interfaces.
	Icons []Icon `json:"icons,omitempty"`

	// Arguments describes the parameters the prompt accepts.
	Arguments []PromptArgument `json:"arguments,omitempty"`
}

// PromptMessageV1 is a V1 message in a prompt template using V1 content blocks.
type PromptMessageV1 struct {
	// Role is either "user" or "assistant".
	Role string `json:"role"`

	// Content is the message content.
	Content ContentBlockV1 `json:"content"`
}

// PromptsListResultV1 is the V1 response to prompts/list with pagination.
type PromptsListResultV1 struct {
	Prompts    []PromptV1 `json:"prompts"`
	NextCursor string     `json:"nextCursor,omitempty"`
}

// PromptGetResultV1 contains the V1 rendered prompt.
type PromptGetResultV1 struct {
	// Description explains the prompt.
	Description string `json:"description,omitempty"`

	// Messages contains the prompt messages.
	Messages []PromptMessageV1 `json:"messages"`
}
