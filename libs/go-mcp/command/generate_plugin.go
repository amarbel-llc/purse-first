package command

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type pluginMcpServer struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

type pluginManifest struct {
	Name       string                     `json:"name"`
	McpServers map[string]pluginMcpServer `json:"mcpServers"`
}

// GeneratePlugin writes a plugin.json manifest to {dir}/{app.Name}/plugin.json.
func (a *App) GeneratePlugin(dir string) error {
	cmdName := a.Name
	if a.MCPBinary != "" {
		cmdName = a.MCPBinary
	}

	manifest := pluginManifest{
		Name: a.Name,
		McpServers: map[string]pluginMcpServer{
			a.Name: {
				Type:    "stdio",
				Command: cmdName,
				Args:    a.MCPArgs,
			},
		},
	}

	pluginDir := filepath.Join(dir, a.Name)
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0o644)
}
