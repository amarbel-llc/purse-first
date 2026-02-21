package protocol

import "encoding/json"

// Task status constants.
const (
	TaskStatusPending   = "pending"
	TaskStatusProgress  = "progress"
	TaskStatusSucceeded = "succeeded"
	TaskStatusErrored   = "errored"
	TaskStatusCancelled = "cancelled"
)

// Task represents an async task tracked by the server.
type Task struct {
	// TaskId is the unique identifier for this task.
	TaskId string `json:"taskId"`

	// Status is the current task state.
	Status string `json:"status"`

	// Type is the request type that created this task.
	Type string `json:"type"`

	// Progress is the completion percentage (0-100).
	Progress *float64 `json:"progress,omitempty"`

	// Result contains the payload when status is "succeeded".
	Result json.RawMessage `json:"result,omitempty"`

	// Error contains details when status is "errored".
	Error *TaskError `json:"error,omitempty"`
}

// TaskError describes why a task failed.
type TaskError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// TaskGetParams specifies which task to retrieve.
type TaskGetParams struct {
	TaskId string `json:"taskId"`
}

// TaskResultParams specifies which task's result to retrieve.
type TaskResultParams struct {
	TaskId string `json:"taskId"`
}

// TaskCancelParams specifies which task to cancel.
type TaskCancelParams struct {
	TaskId string `json:"taskId"`
}

// TaskListResult contains a list of tasks.
type TaskListResult struct {
	Tasks []Task `json:"tasks"`
}

// TaskStatusNotificationParams contains a task status update.
type TaskStatusNotificationParams struct {
	Task Task `json:"task"`
}
