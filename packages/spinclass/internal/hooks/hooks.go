package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

type hookInput struct {
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
	CWD       string         `json:"cwd"`
}

func Run(r io.Reader, w io.Writer, boundary string) error {
	var input hookInput
	if err := json.NewDecoder(r).Decode(&input); err != nil {
		return fmt.Errorf("decoding hook input: %w", err)
	}

	if boundary == "" {
		return nil
	}

	boundary = filepath.Clean(boundary)

	paths := extractPaths(input)
	if paths == nil {
		return nil
	}

	for _, p := range paths {
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
		return []string{"__task_denied__"}
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

func isInsideBoundary(path, boundary string) bool {
	if path == "__task_denied__" {
		return false
	}

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		resolved = filepath.Clean(path)
	}

	return resolved == boundary || strings.HasPrefix(resolved, boundary+string(filepath.Separator))
}

func writeDeny(w io.Writer, toolName, path, boundary string) error {
	var reason string
	if toolName == "Task" {
		reason = fmt.Sprintf(
			"subagents are denied in sandboxed worktrees because they may access files outside the boundary; use the worktree at %s instead of the main worktree",
			boundary,
		)
	} else {
		reason = fmt.Sprintf(
			"path %s is outside the worktree boundary; use the worktree at %s instead of the main worktree",
			path, boundary,
		)
	}

	output := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       "deny",
			"permissionDecisionReason": reason,
		},
	}

	return json.NewEncoder(w).Encode(output)
}
