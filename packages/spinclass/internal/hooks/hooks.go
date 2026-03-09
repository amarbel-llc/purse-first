package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/amarbel-llc/spinclass/internal/sweatfile"
)

type hookInput struct {
	HookEventName string         `json:"hook_event_name"`
	SessionID     string         `json:"session_id"`
	ToolName      string         `json:"tool_name"`
	ToolInput     map[string]any `json:"tool_input"`
	CWD           string         `json:"cwd"`
}

func Run(r io.Reader, w io.Writer, boundary string, allowed []string, boundaryNotify bool) error {
	var input hookInput
	if err := json.NewDecoder(r).Decode(&input); err != nil {
		return fmt.Errorf("decoding hook input: %w", err)
	}

	switch input.HookEventName {
	case "Stop":
		return runStopHook(input, w)
	default:
		return runPreToolUse(input, w, boundary, allowed, boundaryNotify)
	}
}

func runStopHook(input hookInput, w io.Writer) error {
	tmpDir := os.TempDir()
	sentinelPath := filepath.Join(tmpDir, "stop-hook-"+input.SessionID)

	if _, err := os.Stat(sentinelPath); err == nil {
		return nil // second invocation -> approve
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil // can't load sweatfile -> approve
	}

	result, err := sweatfile.LoadHierarchy(home, input.CWD)
	stopCmd := result.Merged.StopHookCommand()
	if err != nil || stopCmd == nil || *stopCmd == "" {
		return nil // no stop hook configured -> approve
	}

	cmd := exec.Command("sh", "-c", *stopCmd)
	cmd.Dir = input.CWD
	output, cmdErr := cmd.CombinedOutput()

	if cmdErr == nil {
		return nil // command passed -> approve
	}

	// Command failed -> write output to sentinel and block
	os.WriteFile(sentinelPath, output, 0o644)

	reason := fmt.Sprintf("stop hook failed: %s", *stopCmd)
	systemMsg := fmt.Sprintf(
		"Stop hook failed. Output written to %s. Review the failures and address them before completing.",
		sentinelPath,
	)

	decision := map[string]any{
		"decision":      "block",
		"reason":        reason,
		"systemMessage": systemMsg,
	}

	return json.NewEncoder(w).Encode(decision)
}

func runPreToolUse(input hookInput, w io.Writer, boundary string, allowed []string, boundaryNotify bool) error {
	if !boundaryNotify || boundary == "" {
		return nil
	}

	boundary = evalOrClean(boundary)

	for i, a := range allowed {
		allowed[i] = evalOrClean(a)
	}

	paths := extractPaths(input)
	if paths == nil {
		return nil
	}

	var violations []string
	for _, p := range paths {
		if isInsideAllowed(p, allowed) {
			continue
		}
		if !isInsideBoundary(p, boundary) {
			violations = append(violations, fmt.Sprintf(
				"worktree boundary violation: %s %s is outside %s",
				input.ToolName, p, boundary,
			))
		}
	}

	if len(violations) == 0 {
		return nil
	}

	context := strings.Join(violations, "\n") +
		"\nActivity outside the worktree should only be performed if the user explicitly requested it. Otherwise, work exclusively within the worktree."

	output := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":      "PreToolUse",
			"permissionDecision": "allow",
			"additionalContext":  context,
		},
	}

	return json.NewEncoder(w).Encode(output)
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

func resolvePath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved
	}

	// File may not exist yet — resolve the parent directory instead.
	dir, base := filepath.Split(path)
	if dir != "" {
		if resolvedDir, dirErr := filepath.EvalSymlinks(dir); dirErr == nil {
			return filepath.Join(resolvedDir, base)
		}
	}

	return filepath.Clean(path)
}

func evalOrClean(path string) string {
	return resolvePath(path)
}

func isInsideAllowed(path string, allowed []string) bool {
	resolved := resolvePath(path)

	for _, a := range allowed {
		if resolved == a || strings.HasPrefix(resolved, a+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func isInsideBoundary(path, boundary string) bool {
	resolved := resolvePath(path)

	return resolved == boundary || strings.HasPrefix(resolved, boundary+string(filepath.Separator))
}

