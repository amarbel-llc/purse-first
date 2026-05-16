package protocol

import "encoding/json"

// ProgressToken identifies a request for progress tracking.
// It can be either a string or an integer.
type ProgressToken = json.RawMessage

// ProgressNotificationParams contains progress information.
type ProgressNotificationParams struct {
	// ProgressToken identifies the request being tracked.
	ProgressToken ProgressToken `json:"progressToken"`

	// Progress is the current progress value.
	Progress float64 `json:"progress"`

	// Total is the total expected value.
	Total *float64 `json:"total,omitempty"`

	// Message describes the current progress stage.
	Message string `json:"message,omitempty"`
}

// CancelledNotificationParams indicates a request was cancelled.
type CancelledNotificationParams struct {
	// RequestId is the ID of the cancelled request.
	RequestId json.RawMessage `json:"requestId"`

	// Reason explains why the request was cancelled.
	Reason string `json:"reason,omitempty"`
}
