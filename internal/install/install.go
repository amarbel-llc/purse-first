package install

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

type marketplacePlugin struct {
	Name string `json:"name"`
}

type marketplaceJSON struct {
	Name    string              `json:"name"`
	Plugins []marketplacePlugin `json:"plugins"`
}

func resolveMarketplaceRoot() (string, error) {
	if envDir := os.Getenv("PURSE_FIRST_PLUGINS_DIR"); envDir != "" {
		root := filepath.Dir(envDir)
		path := filepath.Join(root, ".claude-plugin", "marketplace.json")
		if _, err := os.Stat(path); err == nil {
			return root, nil
		}
		return "", fmt.Errorf("marketplace.json not found at %s", path)
	}

	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("finding executable path: %w", err)
	}

	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		resolved = exe
	}

	root := filepath.Dir(filepath.Dir(resolved))
	path := filepath.Join(root, ".claude-plugin", "marketplace.json")
	if _, err := os.Stat(path); err == nil {
		return root, nil
	}

	return "", fmt.Errorf("marketplace.json not found relative to %s", exe)
}

func readMarketplace(root string) (*marketplaceJSON, error) {
	path := filepath.Join(root, ".claude-plugin", "marketplace.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading marketplace.json: %w", err)
	}

	var m marketplaceJSON
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing marketplace.json: %w", err)
	}

	return &m, nil
}

func Run(w io.Writer) error {
	root, err := resolveMarketplaceRoot()
	if err != nil {
		return fmt.Errorf("resolving marketplace: %w", err)
	}

	m, err := readMarketplace(root)
	if err != nil {
		return err
	}

	// Remove marketplace (ignore errors if not present)
	remove := exec.Command("claude", "plugin", "marketplace", "remove", m.Name)
	remove.Run()
	fmt.Fprintf(w, "removed marketplace %q (if present)\n", m.Name)

	// Add marketplace
	add := exec.Command("claude", "plugin", "marketplace", "add", root)
	add.Stdout = w
	add.Stderr = w
	if err := add.Run(); err != nil {
		return fmt.Errorf("adding marketplace: %w", err)
	}

	// Install all plugins from the marketplace
	for _, plugin := range m.Plugins {
		install := exec.Command("claude", "plugin", "install", plugin.Name+"@"+m.Name)
		install.Stdout = w
		install.Stderr = w
		if err := install.Run(); err != nil {
			return fmt.Errorf("installing plugin %q: %w", plugin.Name, err)
		}
		fmt.Fprintf(w, "installed plugin %s\n", plugin.Name)
	}

	return nil
}
