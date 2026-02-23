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

type pluginAuthor struct {
	Name string `json:"name"`
}

type pluginManifest struct {
	Name        string                     `json:"name"`
	Description string                     `json:"description,omitempty"`
	Author      *pluginAuthor              `json:"author,omitempty"`
	McpServers  map[string]pluginMcpServer `json:"mcpServers,omitempty"`
	Skills      []string                   `json:"skills,omitempty"`
}

// GeneratePlugin writes a plugin.json manifest to {dir}/{app.Name}/plugin.json.
func (a *App) GeneratePlugin(dir string) error {
	cmdName := a.Name
	if a.MCPBinary != "" {
		cmdName = a.MCPBinary
	}

	manifest := pluginManifest{
		Name:        a.Name,
		Description: a.PluginDescription,
		McpServers: map[string]pluginMcpServer{
			a.Name: {
				Type:    "stdio",
				Command: cmdName,
				Args:    a.MCPArgs,
			},
		},
	}

	if a.PluginAuthor != "" {
		manifest.Author = &pluginAuthor{Name: a.PluginAuthor}
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
