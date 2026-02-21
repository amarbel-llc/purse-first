/// V1 method name constants for new protocol methods.

/// Requests argument completions.
pub const METHOD_COMPLETION_COMPLETE: &str = "completion/complete";

/// Sets the server's logging level.
pub const METHOD_LOGGING_SET_LEVEL: &str = "logging/setLevel";

/// Requests information from the user.
pub const METHOD_ELICITATION_CREATE: &str = "elicitation/create";

/// Retrieves a task by ID.
pub const METHOD_TASKS_GET: &str = "tasks/get";

/// Retrieves a task's result.
pub const METHOD_TASKS_RESULT: &str = "tasks/result";

/// Cancels a running task.
pub const METHOD_TASKS_CANCEL: &str = "tasks/cancel";

/// Lists all tasks.
pub const METHOD_TASKS_LIST: &str = "tasks/list";

/// Requests the client's root URIs.
pub const METHOD_ROOTS_LIST: &str = "roots/list";

/// Reports progress.
pub const METHOD_NOTIFICATIONS_PROGRESS: &str = "notifications/progress";

/// Indicates a request was cancelled.
pub const METHOD_NOTIFICATIONS_CANCELLED: &str = "notifications/cancelled";

/// Indicates a task's status changed.
pub const METHOD_NOTIFICATIONS_TASK_STATUS: &str = "notifications/task/status";

/// Indicates resources changed.
pub const METHOD_NOTIFICATIONS_RESOURCES_LIST_CHANGED: &str = "notifications/resources/list_changed";

/// Indicates a resource was updated.
pub const METHOD_NOTIFICATIONS_RESOURCE_UPDATED: &str = "notifications/resources/updated";

/// Indicates prompts changed.
pub const METHOD_NOTIFICATIONS_PROMPTS_LIST_CHANGED: &str = "notifications/prompts/list_changed";

/// Indicates tools changed.
pub const METHOD_NOTIFICATIONS_TOOLS_LIST_CHANGED: &str = "notifications/tools/list_changed";

/// A logging notification.
pub const METHOD_NOTIFICATIONS_MESSAGE: &str = "notifications/message";

/// Indicates roots changed.
pub const METHOD_NOTIFICATIONS_ROOTS_LIST_CHANGED: &str = "notifications/roots/list_changed";

/// Requests the client to sample an LLM.
pub const METHOD_SAMPLING_CREATE_MESSAGE: &str = "sampling/createMessage";
