package decision

type HookInput struct {
	SessionID     string         `json:"session_id"`
	ToolName      string         `json:"tool_name"`
	ToolInput     map[string]any `json:"tool_input"`
	ToolUseID     string         `json:"tool_use_id"`
	CWD           string         `json:"cwd"`
	HookEventName string         `json:"hook_event_name"`
}

type HookSpecificOutput struct {
	HookEventName          string `json:"hookEventName"`
	PermissionDecision     string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason"`
}

type HookOutput struct {
	HookSpecificOutput HookSpecificOutput `json:"hookSpecificOutput"`
}
