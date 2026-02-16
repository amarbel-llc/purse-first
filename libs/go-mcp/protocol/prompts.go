package protocol

// Prompt describes a prompt template available from the server.
type Prompt struct {
	// Name uniquely identifies the prompt.
	Name string `json:"name"`

	// Description explains what the prompt does (optional).
	Description string `json:"description,omitempty"`

	// Arguments describes the parameters the prompt accepts (optional).
	Arguments []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument describes a parameter that can be passed to a prompt.
type PromptArgument struct {
	// Name is the parameter name.
	Name string `json:"name"`

	// Description explains what the parameter is for (optional).
	Description string `json:"description,omitempty"`

	// Required indicates whether this parameter must be provided.
	Required bool `json:"required,omitempty"`
}

// PromptsListResult is the response to prompts/list.
type PromptsListResult struct {
	Prompts []Prompt `json:"prompts"`
}

// PromptGetParams specifies which prompt to retrieve and its arguments.
type PromptGetParams struct {
	// Name is the prompt to retrieve.
	Name string `json:"name"`

	// Arguments are the values for the prompt's parameters.
	Arguments map[string]string `json:"arguments,omitempty"`
}

// PromptGetResult contains the rendered prompt.
type PromptGetResult struct {
	// Description explains the prompt (optional).
	Description string `json:"description,omitempty"`

	// Messages contains the prompt messages.
	Messages []PromptMessage `json:"messages"`
}

// PromptMessage is a message in a prompt template.
type PromptMessage struct {
	// Role is either "user" or "assistant".
	Role string `json:"role"`

	// Content is the message content.
	Content ContentBlock `json:"content"`
}
