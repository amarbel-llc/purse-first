package protocol

// PromptV0 describes a prompt template available from the server.
// This is the V0 (2024-11-05) prompt type.
type PromptV0 struct {
	// Name uniquely identifies the prompt.
	Name string `json:"name"`

	// Description explains what the prompt does (optional).
	Description string `json:"description,omitempty"`

	// Arguments describes the parameters the prompt accepts (optional).
	Arguments []PromptArgument `json:"arguments,omitempty"`
}

// Prompt is a type alias for backward compatibility.
type Prompt = PromptV0

// PromptArgumentV0 describes a parameter that can be passed to a prompt.
type PromptArgumentV0 struct {
	// Name is the parameter name.
	Name string `json:"name"`

	// Description explains what the parameter is for (optional).
	Description string `json:"description,omitempty"`

	// Required indicates whether this parameter must be provided.
	Required bool `json:"required,omitempty"`
}

// PromptArgument is a type alias for backward compatibility.
type PromptArgument = PromptArgumentV0

// PromptsListResultV0 is the response to prompts/list.
type PromptsListResultV0 struct {
	Prompts []Prompt `json:"prompts"`
}

// PromptsListResult is a type alias for backward compatibility.
type PromptsListResult = PromptsListResultV0

// PromptGetParamsV0 specifies which prompt to retrieve and its arguments.
type PromptGetParamsV0 struct {
	// Name is the prompt to retrieve.
	Name string `json:"name"`

	// Arguments are the values for the prompt's parameters.
	Arguments map[string]string `json:"arguments,omitempty"`
}

// PromptGetParams is a type alias for backward compatibility.
type PromptGetParams = PromptGetParamsV0

// PromptGetResultV0 contains the rendered prompt.
type PromptGetResultV0 struct {
	// Description explains the prompt (optional).
	Description string `json:"description,omitempty"`

	// Messages contains the prompt messages.
	Messages []PromptMessage `json:"messages"`
}

// PromptGetResult is a type alias for backward compatibility.
type PromptGetResult = PromptGetResultV0

// PromptMessageV0 is a message in a prompt template.
type PromptMessageV0 struct {
	// Role is either "user" or "assistant".
	Role string `json:"role"`

	// Content is the message content.
	Content ContentBlock `json:"content"`
}

// PromptMessage is a type alias for backward compatibility.
type PromptMessage = PromptMessageV0
