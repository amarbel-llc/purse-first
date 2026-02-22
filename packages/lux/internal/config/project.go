package config

import (
	"fmt"
	"os"
	"path/filepath"
)

var projectMarkers = []string{
	".lux",
	".git",
	"go.mod",
	"package.json",
	"Cargo.toml",
	"pyproject.toml",
}

// FindProjectRoot walks up from a file path to find project markers
func FindProjectRoot(filePath string) (string, error) {
	dir := filePath
	if !isDir(dir) {
		dir = filepath.Dir(dir)
	}

	homeDir, _ := os.UserHomeDir()

	for {
		// Check for project markers
		for _, marker := range projectMarkers {
			markerPath := filepath.Join(dir, marker)
			if exists(markerPath) {
				return dir, nil
			}
		}

		// Stop at home directory
		if dir == homeDir || dir == "/" {
			return "", fmt.Errorf("no project root found")
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("no project root found")
}

// ProjectConfigPath returns the path to project config if it exists
func ProjectConfigPath(projectRoot string) string {
	// Check .lux/lsps.toml first
	luxConfig := filepath.Join(projectRoot, ".lux", "lsps.toml")
	if exists(luxConfig) {
		return luxConfig
	}

	// Fallback to lux.toml in root
	rootConfig := filepath.Join(projectRoot, "lux.toml")
	if exists(rootConfig) {
		return rootConfig
	}

	return ""
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
