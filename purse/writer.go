package purse

// TODO(terminology): rename WritePlugin → WritePackage, plugin.json → package.json (or .toml)
// when breaking change lands.

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// WritePlugin writes a plugin manifest to {dir}/{p.Name}/.claude-plugin/plugin.json.
// This is used during nix postInstall to generate share/purse-first/<name>/.claude-plugin/plugin.json.
func WritePlugin(dir string, p Plugin) error {
	pluginDir := filepath.Join(dir, p.Name, ".claude-plugin")
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

// WriteMappings writes a mappings file to {dir}/{name}/mappings.json.
// This is a no-op if mf is nil.
func WriteMappings(dir, name string, mf *MappingFile) error {
	if mf == nil {
		return nil
	}

	mappingDir := filepath.Join(dir, name)
	if err := os.MkdirAll(mappingDir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(mf, "", "  ")
	if err != nil {
		return err
	}

	data = append(data, '\n')

	return os.WriteFile(filepath.Join(mappingDir, "mappings.json"), data, 0o644)
}
