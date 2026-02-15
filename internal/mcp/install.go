package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type ServerEntry struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

func Install(servers []ServerEntry) error {
	configPath, err := mcpConfigPath()
	if err != nil {
		return err
	}

	existing, err := readMCPConfig(configPath)
	if err != nil {
		return err
	}

	mcpServers, _ := existing["mcpServers"].(map[string]any)
	if mcpServers == nil {
		mcpServers = make(map[string]any)
	}

	for _, s := range servers {
		serverType := s.Type
		if serverType == "" {
			serverType = "stdio"
		}

		mcpServers[s.Name] = map[string]any{
			"type":    serverType,
			"command": s.Command,
			"args":    s.Args,
			"env":     map[string]any{},
		}
	}

	existing["mcpServers"] = mcpServers

	return writeMCPConfig(configPath, existing)
}

func InstallFromMarketplace(manifestPath string) (int, error) {
	m, err := ReadManifest(manifestPath)
	if err != nil {
		return 0, err
	}

	if len(m.Servers) == 0 {
		return 0, nil
	}

	return len(m.Servers), Install(m.Servers)
}

func mcpConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".claude.json"), nil
}

func readMCPConfig(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return make(map[string]any), nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	return config, nil
}

func writeMCPConfig(path string, config map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating directory for %s: %w", path, err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling mcp config: %w", err)
	}

	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
