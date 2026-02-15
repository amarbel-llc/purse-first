package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Marketplace struct {
	Servers []ServerEntry `json:"servers"`
}

func DiscoverManifest() (string, error) {
	if envPath := os.Getenv("PURSE_FIRST_MARKETPLACE"); envPath != "" {
		if _, err := os.Stat(envPath); err != nil {
			return "", fmt.Errorf("manifest from PURSE_FIRST_MARKETPLACE not found: %w", err)
		}
		return envPath, nil
	}

	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("finding executable path: %w", err)
	}

	// Binary is at <root>/bin/purse-first, manifest at <root>/share/purse-first/marketplace.json
	root := filepath.Dir(filepath.Dir(exe))
	manifest := filepath.Join(root, "share", "purse-first", "marketplace.json")

	if _, err := os.Stat(manifest); err == nil {
		return manifest, nil
	}

	return "", fmt.Errorf("marketplace manifest not found relative to %s", exe)
}

func ReadManifest(path string) (*Marketplace, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	var m Marketplace
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	return &m, nil
}
