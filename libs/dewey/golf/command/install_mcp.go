package command

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// InstallMCP registers this app as an MCP server in ~/.claude.json.
// It resolves the running binary's absolute path, reads the existing
// config, merges an entry using the app's Name and MCPArgs, and writes
// the result back.
func (u *Utility) InstallMCP() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable: %w", err)
	}

	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolving symlinks: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	configPath := filepath.Join(home, ".claude.json")

	return u.installMCPTo(resolved, configPath)
}

func (u *Utility) installMCPTo(binaryPath, configPath string) error {
	config, err := readUserMCPConfig(configPath)
	if err != nil {
		return err
	}

	mcpServers, _ := config["mcpServers"].(map[string]any)
	if mcpServers == nil {
		mcpServers = make(map[string]any)
	}

	args := u.MCPArgs
	if args == nil {
		args = []string{}
	}
	entry := map[string]any{
		"type":    "stdio",
		"command": binaryPath,
		"args":    args,
		"env":     map[string]any{},
	}

	mcpServers[u.Name] = entry
	config["mcpServers"] = mcpServers

	if err := writeUserMCPConfig(configPath, config); err != nil {
		return err
	}

	fmt.Printf("installed %s MCP server to %s\n", u.Name, configPath)
	return nil
}

func readUserMCPConfig(path string) (map[string]any, error) {
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

func writeUserMCPConfig(path string, config map[string]any) error {
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
