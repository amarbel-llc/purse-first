package command

import (
	"encoding/json"
	"fmt"
	"io"
)

// hookInput is the JSON sent by Claude Code to PreToolUse hooks via stdin.
type hookInput struct {
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
}

type hookOutput struct {
	HookSpecificOutput hookDecision `json:"hookSpecificOutput"`
}

type hookDecision struct {
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
}

// commandMapping pairs a ToolMapping with the canonical name of the command
// it belongs to.
type commandMapping struct {
	commandName string
	mapping     ToolMapping
}

// allToolMappings collects every ToolMapping from every registered command,
// paired with the canonical command name.
func (a *App) allToolMappings() []commandMapping {
	var out []commandMapping

	for name, cmd := range a.AllCommands() {
		for _, tm := range cmd.MapsTools {
			out = append(out, commandMapping{
				commandName: name,
				mapping:     tm,
			})
		}
	}

	return out
}

// HandleHook reads a hookInput from r, checks it against all registered
// ToolMappings, and writes a deny hookOutput to w when a match is found.
// If no mapping matches the tool invocation, nothing is written (implicit allow).
func (a *App) HandleHook(r io.Reader, w io.Writer) error {
	var hi hookInput
	if err := json.NewDecoder(r).Decode(&hi); err != nil {
		return fmt.Errorf("decoding hook input: %w", err)
	}

	filePath := extractString(hi.ToolInput, "file_path", "path", "pattern")
	command := extractString(hi.ToolInput, "command")

	cms := a.allToolMappings()

	var matchedCM *commandMapping
	for i := range cms {
		if FindToolMatch([]ToolMapping{cms[i].mapping}, hi.ToolName, filePath, command) != nil {
			matchedCM = &cms[i]
			break
		}
	}
	if matchedCM == nil {
		return nil
	}

	toolName := fmt.Sprintf("mcp__plugin_%s_%s__%s",
		a.Name, a.Name, matchedCM.commandName)

	reason := fmt.Sprintf("Use the MCP tool instead:\n- %s: %s",
		toolName, matchedCM.mapping.UseWhen)

	out := hookOutput{
		HookSpecificOutput: hookDecision{
			PermissionDecision:       "deny",
			PermissionDecisionReason: reason,
		},
	}

	return json.NewEncoder(w).Encode(out)
}

// extractString returns the first non-empty string value found under any of
// the given keys in the map.
func extractString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}

	return ""
}
