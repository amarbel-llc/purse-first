package command

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type devMcpConfig struct {
	McpServers map[string]pluginMcpServer `json:"mcpServers"`
}

type devMappingFile struct {
	Server     string         `json:"server"`
	ToolPrefix string         `json:"tool_prefix"`
	Mappings   []mappingEntry `json:"mappings"`
}

func resolveBuildDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolving executable: %w", err)
	}

	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("resolving symlinks: %w", err)
	}

	// Binary is at {buildDir}/bin/{name}, so buildDir is two levels up
	return filepath.Dir(filepath.Dir(resolved)), nil
}

func generateDevMCP(buildDir, projectDir, suffix string) error {
	// Read plugin.json
	pluginDirs, err := filepath.Glob(filepath.Join(buildDir, "share", "purse-first", "*", ".claude-plugin", "plugin.json"))
	if err != nil {
		return fmt.Errorf("finding plugin.json: %w", err)
	}
	if len(pluginDirs) == 0 {
		return fmt.Errorf("no plugin.json found in %s/share/purse-first/*/.claude-plugin/", buildDir)
	}

	pluginPath := pluginDirs[0]
	pluginData, err := os.ReadFile(pluginPath)
	if err != nil {
		return fmt.Errorf("reading plugin.json: %w", err)
	}

	var manifest pluginManifest
	if err := json.Unmarshal(pluginData, &manifest); err != nil {
		return fmt.Errorf("parsing plugin.json: %w", err)
	}

	name := manifest.Name
	serverKey := name + "-" + suffix

	// Find the binary
	binDir := filepath.Join(buildDir, "bin")
	entries, err := os.ReadDir(binDir)
	if err != nil {
		return fmt.Errorf("reading bin directory: %w", err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("no binaries found in %s", binDir)
	}
	binaryPath := filepath.Join(binDir, entries[0].Name())

	// Get MCP args from the plugin manifest
	var mcpArgs []string
	for _, server := range manifest.McpServers {
		mcpArgs = server.Args
		break
	}

	// 1. Write .mcp.json
	mcpConfig := devMcpConfig{
		McpServers: map[string]pluginMcpServer{
			serverKey: {
				Type:    "stdio",
				Command: binaryPath,
				Args:    mcpArgs,
			},
		},
	}

	mcpData, err := json.MarshalIndent(mcpConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling .mcp.json: %w", err)
	}
	mcpData = append(mcpData, '\n')

	if err := os.WriteFile(filepath.Join(projectDir, ".mcp.json"), mcpData, 0o644); err != nil {
		return fmt.Errorf("writing .mcp.json: %w", err)
	}

	// 2. Write .purse-first/<name>.json with tool_prefix
	mappingsPath := filepath.Join(buildDir, "share", "purse-first", name, "mappings.json")
	// Write .purse-first/<name>.json if mappings.json exists in build artifacts.
	// Not all plugins have mappings, so a missing file is not an error.
	mappingsData, err := os.ReadFile(mappingsPath)
	if err == nil {
		var sourceMappings mappingFile
		if err := json.Unmarshal(mappingsData, &sourceMappings); err != nil {
			return fmt.Errorf("parsing mappings.json: %w", err)
		}

		devMappings := devMappingFile{
			Server:     sourceMappings.Server,
			ToolPrefix: "mcp__" + serverKey,
			Mappings:   sourceMappings.Mappings,
		}

		purseDir := filepath.Join(projectDir, ".purse-first")
		if err := os.MkdirAll(purseDir, 0o755); err != nil {
			return fmt.Errorf("creating .purse-first/: %w", err)
		}

		devData, err := json.MarshalIndent(devMappings, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling dev mappings: %w", err)
		}
		devData = append(devData, '\n')

		if err := os.WriteFile(filepath.Join(purseDir, name+".json"), devData, 0o644); err != nil {
			return fmt.Errorf("writing dev mappings: %w", err)
		}
	}

	return nil
}

func cleanDevMCP(projectDir string) error {
	os.Remove(filepath.Join(projectDir, ".mcp.json"))
	os.RemoveAll(filepath.Join(projectDir, ".purse-first"))
	return nil
}

func (u *Utility) addDevMCPCommand() {
	u.AddCommand(&Command{
		Name:   "dev-mcp",
		Hidden: true,
		Description: Description{
			Short: "Generate project-local MCP config for dev testing",
		},
		OldParams: []OldParam{
			{Name: "suffix", Type: String, Description: "Suffix for the MCP server name", Default: "dev"},
			{Name: "clean", Type: Bool, Description: "Remove generated dev artifacts", Default: false},
		},
	})
}
