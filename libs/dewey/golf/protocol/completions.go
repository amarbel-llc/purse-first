package protocol

// CompletionReference identifies what is being completed.
type CompletionReference struct {
	// Type is "ref/prompt" or "ref/resource".
	Type string `json:"type"`

	// Name is the prompt name (for type="ref/prompt").
	Name string `json:"name,omitempty"`

	// URI is the resource URI (for type="ref/resource").
	URI string `json:"uri,omitempty"`
}

// CompletionArgument identifies the argument being completed.
type CompletionArgument struct {
	// Name is the argument name.
	Name string `json:"name"`

	// Value is the current partial value.
	Value string `json:"value"`
}

// CompletionCompleteParams contains the parameters for a completion request.
type CompletionCompleteParams struct {
	// Ref identifies what is being completed.
	Ref CompletionReference `json:"ref"`

	// Argument identifies the argument being completed.
	Argument CompletionArgument `json:"argument"`

	// Context provides previously-resolved variables for contextual completions.
	Context map[string]string `json:"context,omitempty"`
}

// CompletionResult contains completion suggestions.
type CompletionResult struct {
	// Completion contains the completion values.
	Completion CompletionValues `json:"completion"`

	// Meta contains protocol-level metadata.
	Meta map[string]any `json:"_meta,omitempty"`
}

// CompletionValues contains the actual completion suggestions.
type CompletionValues struct {
	// Values is the list of suggested completions.
	Values []string `json:"values"`

	// Total is the total number of available completions.
	Total *int `json:"total,omitempty"`

	// HasMore indicates whether more completions are available.
	HasMore bool `json:"hasMore,omitempty"`
}

// LoggingLevel represents a log severity level.
type LoggingLevel string

// Logging level constants.
const (
	LoggingLevelDebug     LoggingLevel = "debug"
	LoggingLevelInfo      LoggingLevel = "info"
	LoggingLevelNotice    LoggingLevel = "notice"
	LoggingLevelWarning   LoggingLevel = "warning"
	LoggingLevelError     LoggingLevel = "error"
	LoggingLevelCritical  LoggingLevel = "critical"
	LoggingLevelAlert     LoggingLevel = "alert"
	LoggingLevelEmergency LoggingLevel = "emergency"
)

// SetLevelParams contains the parameters for setting the logging level.
type SetLevelParams struct {
	Level LoggingLevel `json:"level"`
}

// LoggingMessageParams contains a logging notification.
type LoggingMessageParams struct {
	// Level is the severity level.
	Level LoggingLevel `json:"level"`

	// Data is the log data.
	Data interface{} `json:"data"`

	// Logger identifies the source.
	Logger string `json:"logger,omitempty"`
}
