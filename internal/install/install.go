package install

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
)

var styleCode = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#E88388")).
	Background(lipgloss.Color("#1D1F21")).
	Padding(0, 1)

type marketplacePlugin struct {
	Name string `json:"name"`
}

type marketplaceJSON struct {
	Name    string              `json:"name"`
	Plugins []marketplacePlugin `json:"plugins"`
}

// rootFromPluginsDir returns the marketplace root given PURSE_FIRST_PLUGINS_DIR.
// The layout is: <root>/share/purse-first (plugins dir) with marketplace.json
// at <root>/.claude-plugin/marketplace.json — two levels up.
func rootFromPluginsDir(pluginsDir string) string {
	return filepath.Dir(filepath.Dir(pluginsDir))
}

func resolveMarketplaceRoot() (string, error) {
	if envDir := os.Getenv("PURSE_FIRST_PLUGINS_DIR"); envDir != "" {
		root := rootFromPluginsDir(envDir)
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

	// TAP header: 4 fixed steps + one per plugin
	total := 4 + len(m.Plugins)
	fmt.Fprintf(w, "TAP version 14\n")
	fmt.Fprintf(w, "1..%d\n", total)

	n := 1

	// 1. Resolve marketplace root
	fmt.Fprintf(w, "ok %d - resolve marketplace root\n", n)
	n++

	// 2. Read marketplace.json
	fmt.Fprintf(w, "ok %d - read marketplace.json (%d plugins)\n", n, len(m.Plugins))
	n++

	// 3. Remove marketplace (ignore errors if not present)
	remove := exec.Command("claude", "plugin", "marketplace", "remove", m.Name)
	remove.Run()
	fmt.Fprintf(w, "ok %d - remove marketplace %s\n", n, styleCode.Render(m.Name))
	n++

	// 4. Add marketplace
	add := exec.Command("claude", "plugin", "marketplace", "add", root)
	if err := add.Run(); err != nil {
		fmt.Fprintf(w, "not ok %d - add marketplace %s\n", n, styleCode.Render(m.Name))
		return fmt.Errorf("adding marketplace: %w", err)
	}
	fmt.Fprintf(w, "ok %d - add marketplace %s\n", n, styleCode.Render(m.Name))
	n++

	// 5..N Install each plugin
	for _, plugin := range m.Plugins {
		ref := plugin.Name + "@" + m.Name
		install := exec.Command("claude", "plugin", "install", ref)
		if err := install.Run(); err != nil {
			fmt.Fprintf(w, "not ok %d - install plugin %s\n", n, styleCode.Render(plugin.Name))
			return fmt.Errorf("installing plugin %q: %w", plugin.Name, err)
		}
		fmt.Fprintf(w, "ok %d - install plugin %s\n", n, styleCode.Render(plugin.Name))
		n++
	}

	return nil
}
