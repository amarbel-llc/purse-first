package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/amarbel-llc/purse-first/purse"
)

func DiscoverPlugins() ([]ServerEntry, error) {
	pluginsDir, root, err := resolvePluginsDir()
	if err != nil {
		return nil, err
	}

	entries, err := discoverFromPluginDir(pluginsDir, root)
	if err != nil {
		return nil, err
	}

	if len(entries) > 0 {
		return entries, nil
	}

	return nil, fmt.Errorf("no plugin manifests found in %s", pluginsDir)
}

func resolvePluginsDir() (string, string, error) {
	if envDir := os.Getenv("PURSE_FIRST_PLUGINS_DIR"); envDir != "" {
		if info, err := os.Stat(envDir); err == nil && info.IsDir() {
			root := filepath.Dir(filepath.Dir(envDir))
			return envDir, root, nil
		}
		return "", "", fmt.Errorf("PURSE_FIRST_PLUGINS_DIR not found: %s", envDir)
	}

	exe, err := os.Executable()
	if err != nil {
		return "", "", fmt.Errorf("finding executable path: %w", err)
	}

	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		resolved = exe
	}

	// Binary at <root>/bin/purse-first, plugins at <root>/share/purse-first/
	root := filepath.Dir(filepath.Dir(resolved))
	pluginsDir := filepath.Join(root, "share", "purse-first")

	if info, err := os.Stat(pluginsDir); err == nil && info.IsDir() {
		return pluginsDir, root, nil
	}

	return "", "", fmt.Errorf("plugins directory not found relative to %s", exe)
}

func discoverFromPluginDir(pluginsDir, root string) ([]ServerEntry, error) {
	matches, err := filepath.Glob(filepath.Join(pluginsDir, "*", "plugin.json"))
	if err != nil {
		return nil, fmt.Errorf("globbing plugin manifests: %w", err)
	}

	var entries []ServerEntry
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var p purse.Plugin
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}

		for name, srv := range p.McpServers {
			cmd := srv.Command
			if cmd != "" && !filepath.IsAbs(cmd) {
				cmd = filepath.Join(root, "bin", cmd)
			}

			entries = append(entries, ServerEntry{
				Name:    name,
				Type:    srv.Type,
				Command: cmd,
				Args:    srv.Args,
			})
		}
	}

	return entries, nil
}
