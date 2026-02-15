package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/amarbel-llc/go-lib-mcp/purse"
	"github.com/friedenberg/purse-first/internal/mapping"
)

type ServerEntry = purse.Plugin
type Notification = purse.Notification
type NotifyCondition = purse.NotifyCondition
type HTTPPostAction = purse.HTTPPostAction

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

func InstallFromPlugins(servers []ServerEntry) (int, error) {
	if len(servers) == 0 {
		return 0, nil
	}

	if err := Install(servers); err != nil {
		return 0, err
	}

	installDefaultMappings(servers)

	return len(servers), nil
}

func installDefaultMappings(servers []ServerEntry) {
	stateDir := mapping.StateDir()
	os.MkdirAll(stateDir, 0o755)

	for _, s := range servers {
		if len(s.Mappings) == 0 {
			continue
		}

		dest := filepath.Join(stateDir, s.Name+".json")

		// Don't overwrite user-managed mappings
		if _, err := os.Stat(dest); err == nil {
			continue
		}

		mf := mapping.MappingFile{
			Server:   s.Name,
			Mappings: s.Mappings,
		}

		data, err := json.MarshalIndent(mf, "", "  ")
		if err != nil {
			continue
		}

		os.WriteFile(dest, append(data, '\n'), 0o644)
	}
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
