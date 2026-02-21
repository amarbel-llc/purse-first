package protocol

import "encoding/json"

// Elicitation action constants.
const (
	ElicitationActionAccept  = "accept"
	ElicitationActionDecline = "decline"
	ElicitationActionCancel  = "cancel"
)

// ElicitRequestParams contains the parameters for an elicitation request.
// The Mode field determines whether this is a form or URL elicitation.
type ElicitRequestParams struct {
	// Mode is either "form" or "url".
	Mode string `json:"mode"`

	// Message explains what information is being requested.
	Message string `json:"message"`

	// RequestedSchema is a JSON Schema for form elicitation (mode="form").
	RequestedSchema json.RawMessage `json:"requestedSchema,omitempty"`

	// URL is the target URL for URL elicitation (mode="url").
	URL string `json:"url,omitempty"`

	// ElicitationId is a unique identifier for URL elicitation (mode="url").
	ElicitationId string `json:"elicitationId,omitempty"`
}

// ElicitResult is the result of an elicitation request.
type ElicitResult struct {
	// Action indicates the user's response ("accept", "decline", "cancel").
	Action string `json:"action"`

	// Content contains the form data when action is "accept".
	Content json.RawMessage `json:"content,omitempty"`
}
