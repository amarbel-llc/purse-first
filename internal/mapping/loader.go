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

func LoadMappings(projectDir string) ([]MappingFile, error) {
	globalDir := filepath.Join(xdgStateHome(), "purse-first")
	localDir := filepath.Join(projectDir, ".purse-first")

	globalFiles, _ := loadDir(globalDir)
	localFiles, _ := loadDir(localDir)

	// Index global files by server name
	byServer := make(map[string]MappingFile)
	for _, f := range globalFiles {
		byServer[f.Server] = f
	}

	// Local overrides global per server
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
	Server  string
	Mapping Mapping
}

func FindMatch(files []MappingFile, toolName string, filePath string) *Match {
	ext := strings.ToLower(filepath.Ext(filePath))

	for _, f := range files {
		for _, m := range f.Mappings {
			if m.Replaces != toolName {
				continue
			}

			if len(m.Extensions) == 0 {
				return &Match{Server: f.Server, Mapping: m}
			}

			for _, e := range m.Extensions {
				if strings.ToLower(e) == ext {
					return &Match{Server: f.Server, Mapping: m}
				}
			}
		}
	}

	return nil
}
