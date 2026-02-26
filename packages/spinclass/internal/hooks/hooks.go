package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

type hookInput struct {
	HookEventName string         `json:"hook_event_name"`
	SessionID     string         `json:"session_id"`
	ToolName      string         `json:"tool_name"`
	ToolInput     map[string]any `json:"tool_input"`
	CWD           string         `json:"cwd"`
}

func Run(r io.Reader, w io.Writer, boundary string, allowed []string) error {
	var input hookInput
	if err := json.NewDecoder(r).Decode(&input); err != nil {
		return fmt.Errorf("decoding hook input: %w", err)
	}

	switch input.HookEventName {
	case "Stop":
		return runStopHook(input, w)
	default:
		return runPreToolUse(input, w, boundary)
	}
}

func runStopHook(input hookInput, w io.Writer) error {
	return nil // stub: no stop_hook configured -> approve
}

func runPreToolUse(input hookInput, w io.Writer, boundary string) error {
	if boundary == "" {
		return nil
	}

	boundary = filepath.Clean(boundary)

	for i, a := range allowed {
		allowed[i] = filepath.Clean(a)
	}

	paths := extractPaths(input)
	if paths == nil {
		return nil
	}

	for _, p := range paths {
		if isInsideAllowed(p, allowed) {
			continue
		}
		if !isInsideBoundary(p, boundary) {
			return writeDeny(w, input.ToolName, p, boundary)
		}
	}

	return nil
}

func extractPaths(input hookInput) []string {
	switch input.ToolName {
	case "Read", "Write", "Edit":
		if fp, ok := input.ToolInput["file_path"].(string); ok && fp != "" {
			return []string{fp}
		}
	case "Glob", "Grep":
		if p, ok := input.ToolInput["path"].(string); ok && p != "" {
			return []string{p}
		}
	case "Bash":
		return extractAbsolutePathsFromCommand(input)
	case "Task":
		return nil
	}
	return nil
}

func extractAbsolutePathsFromCommand(input hookInput) []string {
	cmd, ok := input.ToolInput["command"].(string)
	if !ok || cmd == "" {
		return nil
	}

	var paths []string
	for _, token := range strings.Fields(cmd) {
		if strings.HasPrefix(token, "/") {
			paths = append(paths, token)
		}
	}
	return paths
}

func isInsideAllowed(path string, allowed []string) bool {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		resolved = filepath.Clean(path)
	}

	for _, a := range allowed {
		if resolved == a || strings.HasPrefix(resolved, a+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func isInsideBoundary(path, boundary string) bool {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		resolved = filepath.Clean(path)
	}

	return resolved == boundary || strings.HasPrefix(resolved, boundary+string(filepath.Separator))
}

func writeDeny(w io.Writer, toolName, path, boundary string) error {
	reason := fmt.Sprintf(
		"path %s is outside the worktree boundary; use the worktree at %s instead of the main worktree",
		path, boundary,
	)

	output := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       "deny",
			"permissionDecisionReason": reason,
		},
	}

	return json.NewEncoder(w).Encode(output)
}
