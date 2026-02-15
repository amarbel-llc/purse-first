package purse

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// WritePlugin writes a plugin manifest to {dir}/{p.Name}/plugin.json.
// This is used during nix postInstall to generate share/purse-first/<name>/plugin.json.
func WritePlugin(dir string, p Plugin) error {
	pluginDir := filepath.Join(dir, p.Name)
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}

	data = append(data, '\n')

	return os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0o644)
}
