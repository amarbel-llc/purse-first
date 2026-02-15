package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/friedenberg/purse-first/internal/decision"
)

func HandlePostToolUse(stdin io.Reader, stdout io.Writer) error {
	data, err := io.ReadAll(stdin)
	if err != nil {
		return nil
	}

	var input decision.HookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil
	}

	filePath := extractFilePath(input.ToolInput)
	if filePath == "" {
		return nil
	}

	// Only open files that look like real file paths
	if !strings.HasPrefix(filePath, "/") {
		return nil
	}

	uri := fmt.Sprintf("file://%s", filePath)

	// Fire and forget -- fail open
	postToLux("/documents/open", map[string]string{"uri": uri})

	return nil
}
