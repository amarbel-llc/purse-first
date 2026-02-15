package hook

import (
	"encoding/json"
	"io"

	"github.com/friedenberg/purse-first/internal/decision"
	"github.com/friedenberg/purse-first/internal/mcp"
)

func HandlePostToolUse(stdin io.Reader, stdout io.Writer) error {
	plugins, err := mcp.DiscoverPlugins()
	if err != nil || len(plugins) == 0 {
		return nil
	}

	data, err := io.ReadAll(stdin)
	if err != nil {
		return nil
	}

	var input decision.HookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil
	}

	filePath := extractFilePath(input.ToolInput)

	vars := map[string]string{
		"file_path": filePath,
		"tool_name": input.ToolName,
	}

	fireNotificationsForEvent("post_tool_use", plugins, vars)

	return nil
}
