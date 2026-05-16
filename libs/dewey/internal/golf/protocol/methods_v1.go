package protocol

// V1 method name constants for new protocol methods.
const (
	// MethodCompletionComplete requests argument completions.
	MethodCompletionComplete = "completion/complete"

	// MethodLoggingSetLevel sets the server's logging level.
	MethodLoggingSetLevel = "logging/setLevel"

	// MethodElicitationCreate requests information from the user.
	MethodElicitationCreate = "elicitation/create"

	// MethodTasksGet retrieves a task by ID.
	MethodTasksGet = "tasks/get"

	// MethodTasksResult retrieves a task's result.
	MethodTasksResult = "tasks/result"

	// MethodTasksCancel cancels a running task.
	MethodTasksCancel = "tasks/cancel"

	// MethodTasksList lists all tasks.
	MethodTasksList = "tasks/list"

	// MethodRootsList requests the client's root URIs.
	MethodRootsList = "roots/list"

	// MethodNotificationsProgress is sent to report progress.
	MethodNotificationsProgress = "notifications/progress"

	// MethodNotificationsCancelled is sent when a request is cancelled.
	MethodNotificationsCancelled = "notifications/cancelled"

	// MethodNotificationsTaskStatus is sent when a task's status changes.
	MethodNotificationsTaskStatus = "notifications/task/status"

	// MethodNotificationsResourcesListChanged is sent when resources change.
	MethodNotificationsResourcesListChanged = "notifications/resources/list_changed"

	// MethodNotificationsResourceUpdated is sent when a resource is updated.
	MethodNotificationsResourceUpdated = "notifications/resources/updated"

	// MethodNotificationsPromptsListChanged is sent when prompts change.
	MethodNotificationsPromptsListChanged = "notifications/prompts/list_changed"

	// MethodNotificationsToolsListChanged is sent when tools change.
	MethodNotificationsToolsListChanged = "notifications/tools/list_changed"

	// MethodNotificationsMessage is a logging notification.
	MethodNotificationsMessage = "notifications/message"

	// MethodNotificationsElicitationComplete is sent when a URL elicitation completes.
	MethodNotificationsElicitationComplete = "notifications/elicitation/complete"

	// MethodNotificationsRootsListChanged is sent when roots change.
	MethodNotificationsRootsListChanged = "notifications/roots/list_changed"

	// MethodSamplingCreateMessage requests the client to sample an LLM.
	MethodSamplingCreateMessage = "sampling/createMessage"
)
