package localplugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// installMCPServers reads mcpServers from plugin.json and writes them to
// settingsPath (.claude/settings.json) with commands rewritten to use
// "go run ./cmd/<name>".
func installMCPServers(pluginPath, settingsPath string) (int, error) {
	data, err := os.ReadFile(pluginPath)
	if err != nil {
		return 0, fmt.Errorf("reading plugin.json: %w", err)
	}

	var plugin map[string]any
	if err := json.Unmarshal(data, &plugin); err != nil {
		return 0, fmt.Errorf("parsing plugin.json: %w", err)
	}

	mcpServersRaw, ok := plugin["mcpServers"].(map[string]any)
	if !ok || len(mcpServersRaw) == 0 {
		return 0, nil
	}

	// Read existing settings
	settings := make(map[string]any)
	if settingsData, err := os.ReadFile(settingsPath); err == nil {
		json.Unmarshal(settingsData, &settings)
	}

	existingServers, _ := settings["mcpServers"].(map[string]any)
	if existingServers == nil {
		existingServers = make(map[string]any)
	}

	count := 0
	for name, serverRaw := range mcpServersRaw {
		serverMap, ok := serverRaw.(map[string]any)
		if !ok {
			continue
		}

		// Rewrite command to "go run ./cmd/<name>"
		goArgs := []any{"run", "./cmd/" + name}

		// Append original args
		if origArgs, ok := serverMap["args"].([]any); ok {
			goArgs = append(goArgs, origArgs...)
		}

		existingServers[name] = map[string]any{
			"type":    "stdio",
			"command": "go",
			"args":    goArgs,
			"env":     map[string]any{},
		}
		count++
	}

	settings["mcpServers"] = existingServers

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return 0, fmt.Errorf("creating settings directory: %w", err)
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return 0, fmt.Errorf("marshaling settings: %w", err)
	}
	out = append(out, '\n')

	return count, os.WriteFile(settingsPath, out, 0o644)
}
