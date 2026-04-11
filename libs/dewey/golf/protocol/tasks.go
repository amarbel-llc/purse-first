package protocol

// Task status constants per 2025-11-25 spec.
const (
	TaskStatusWorking       = "working"
	TaskStatusInputRequired = "input_required"
	TaskStatusCompleted     = "completed"
	TaskStatusFailed        = "failed"
	TaskStatusCancelled     = "cancelled"
)

// Task represents an async task tracked by the server.
type Task struct {
	// TaskId is the unique identifier for this task.
	TaskId string `json:"taskId"`

	// Status is the current task state.
	Status string `json:"status"`

	// StatusMessage is a human-readable description of the current state.
	StatusMessage string `json:"statusMessage,omitempty"`

	// CreatedAt is an ISO 8601 timestamp of when the task was created.
	CreatedAt string `json:"createdAt"`

	// LastUpdatedAt is an ISO 8601 timestamp of the last status update.
	LastUpdatedAt string `json:"lastUpdatedAt"`

	// TTL is the time in milliseconds from creation before task may be deleted.
	TTL *int64 `json:"ttl,omitempty"`

	// PollInterval is the suggested time in milliseconds between status checks.
	PollInterval *int64 `json:"pollInterval,omitempty"`

	// Meta contains protocol-level metadata.
	Meta map[string]any `json:"_meta,omitempty"`
}

// CreateTaskResult is returned when a task-augmented request is accepted.
type CreateTaskResult struct {
	Task Task `json:"task"`
}

// TaskParams is included in request params to create a task.
type TaskParams struct {
	// TTL is the requested duration in milliseconds to retain the task.
	TTL *int64 `json:"ttl,omitempty"`
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

// TaskListParams contains optional pagination for task listing.
type TaskListParams struct {
	Cursor string `json:"cursor,omitempty"`
}

// TaskListResult contains a paginated list of tasks.
type TaskListResult struct {
	Tasks      []Task `json:"tasks"`
	NextCursor string `json:"nextCursor,omitempty"`
}

// TaskStatusNotificationParams contains a task status update.
type TaskStatusNotificationParams struct {
	TaskId        string `json:"taskId"`
	Status        string `json:"status"`
	StatusMessage string `json:"statusMessage,omitempty"`
	CreatedAt     string `json:"createdAt"`
	LastUpdatedAt string `json:"lastUpdatedAt"`
	TTL           *int64 `json:"ttl,omitempty"`
	PollInterval  *int64 `json:"pollInterval,omitempty"`
}
