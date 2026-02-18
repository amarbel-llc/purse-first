package mapping

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func xdgStateHome() string {
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state")
}

func StateDir() string {
	return filepath.Join(xdgStateHome(), "purse-first")
}

func loadDir(dir string) ([]MappingFile, error) {
	entries, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}

	var files []MappingFile
	for _, entry := range entries {
		data, err := os.ReadFile(entry)
		if err != nil {
			continue
		}

		var mf MappingFile
		if err := json.Unmarshal(data, &mf); err != nil {
			continue
		}

		files = append(files, mf)
	}

	return files, nil
}

func resolvePluginsDir() string {
	if envDir := os.Getenv("PURSE_FIRST_PLUGINS_DIR"); envDir != "" {
		return envDir
	}

	exe, err := os.Executable()
	if err != nil {
		return ""
	}

	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		resolved = exe
	}

	return filepath.Join(filepath.Dir(filepath.Dir(resolved)), "share", "purse-first")
}

func loadPluginMappings(pluginsDir string) []MappingFile {
	if pluginsDir == "" {
		return nil
	}

	matches, _ := filepath.Glob(filepath.Join(pluginsDir, "*", "mappings.json"))

	var files []MappingFile
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var mf MappingFile
		if err := json.Unmarshal(data, &mf); err != nil {
			continue
		}

		files = append(files, mf)
	}

	return files
}

func LoadMappings(projectDir string) ([]MappingFile, error) {
	pluginsDir := resolvePluginsDir()
	globalDir := filepath.Join(xdgStateHome(), "purse-first")
	localDir := filepath.Join(projectDir, ".purse-first")

	pluginFiles := loadPluginMappings(pluginsDir)
	globalFiles, _ := loadDir(globalDir)
	localFiles, _ := loadDir(localDir)

	// Merge with priority: plugin < global < local
	byServer := make(map[string]MappingFile)
	for _, f := range pluginFiles {
		byServer[f.Server] = f
	}
	for _, f := range globalFiles {
		byServer[f.Server] = f
	}
	for _, f := range localFiles {
		byServer[f.Server] = f
	}

	result := make([]MappingFile, 0, len(byServer))
	for _, f := range byServer {
		result = append(result, f)
	}

	return result, nil
}

type Match struct {
	Server     string
	ToolPrefix string
	Mapping    Mapping
}

func matchesCriteria(m Mapping, filePath, command string) bool {
	hasExtensions := len(m.Extensions) > 0
	hasPrefixes := len(m.CommandPrefixes) > 0

	if !hasExtensions && !hasPrefixes {
		return true
	}

	if hasExtensions && filePath != "" {
		ext := strings.ToLower(filepath.Ext(filePath))
		for _, e := range m.Extensions {
			if strings.ToLower(e) == ext {
				return true
			}
		}
	}

	if hasPrefixes && command != "" {
		for _, prefix := range m.CommandPrefixes {
			if strings.HasPrefix(command, prefix) {
				return true
			}
		}
	}

	return false
}

func FindMatch(files []MappingFile, toolName, filePath, command string) *Match {
	for _, f := range files {
		for _, m := range f.Mappings {
			if m.Replaces != toolName {
				continue
			}

			if matchesCriteria(m, filePath, command) {
				return &Match{Server: f.Server, ToolPrefix: f.ToolPrefix, Mapping: m}
			}
		}
	}

	return nil
}
