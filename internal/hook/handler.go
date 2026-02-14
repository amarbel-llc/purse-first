package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/friedenberg/purse-first/internal/decision"
	"github.com/friedenberg/purse-first/internal/mapping"
)

func extractFilePath(toolInput map[string]any) string {
	// Read, Edit, Write use "file_path"
	if fp, ok := toolInput["file_path"].(string); ok {
		return fp
	}

	// Grep, Glob use "path" or "pattern" with file paths
	if p, ok := toolInput["path"].(string); ok {
		return p
	}

	// Grep has "pattern" but that's the search pattern, not file path
	// Glob has "pattern" for the glob pattern
	if p, ok := toolInput["pattern"].(string); ok {
		return p
	}

	return ""
}

func formatDenyReason(server string, m mapping.Mapping) string {
	var b strings.Builder

	b.WriteString(m.Reason)
	b.WriteString(":\n")

	for _, t := range m.Tools {
		fmt.Fprintf(&b, "- mcp__%s__%s: %s\n", server, t.Name, t.UseWhen)
	}

	return strings.TrimRight(b.String(), "\n")
}

func HandlePreToolUse(stdin io.Reader, stdout io.Writer, projectDir string) error {
	var input decision.HookInput

	data, err := io.ReadAll(stdin)
	if err != nil {
		// Fail open
		return nil
	}

	if err := json.Unmarshal(data, &input); err != nil {
		// Fail open
		return nil
	}

	files, err := mapping.LoadMappings(projectDir)
	if err != nil || len(files) == 0 {
		// No mappings → passthrough
		return nil
	}

	filePath := extractFilePath(input.ToolInput)
	if filePath == "" {
		// No file path to match against → passthrough
		return nil
	}

	match := mapping.FindMatch(files, input.ToolName, filePath)
	if match == nil {
		// No matching rule → passthrough
		return nil
	}

	output := decision.HookOutput{
		HookSpecificOutput: decision.HookSpecificOutput{
			HookEventName:           "PreToolUse",
			PermissionDecision:      "deny",
			PermissionDecisionReason: formatDenyReason(match.Server, match.Mapping),
		},
	}

	return json.NewEncoder(stdout).Encode(output)
}
