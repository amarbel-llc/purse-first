package marketplace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/amarbel-llc/purse-first/purse"
)

func ReadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("reading config: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("parsing config: %w", err)
	}

	return config, nil
}

func DiscoverPlugins(pluginsDir string) ([]DiscoveredPlugin, error) {
	matches, err := filepath.Glob(filepath.Join(pluginsDir, "*", "plugin.json"))
	if err != nil {
		return nil, fmt.Errorf("globbing plugin manifests: %w", err)
	}

	var plugins []DiscoveredPlugin
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var p purse.Plugin
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}

		storePath := resolveStorePath(path)

		for name, srv := range p.McpServers {
			plugins = append(plugins, DiscoveredPlugin{
				Name:      name,
				Type:      srv.Type,
				Command:   srv.Command,
				Args:      srv.Args,
				StorePath: storePath,
			})
		}
	}

	return plugins, nil
}

func resolveStorePath(manifestPath string) string {
	resolved, err := filepath.EvalSymlinks(manifestPath)
	if err != nil {
		return ""
	}

	// Nix store paths are /nix/store/<hash>-<name>/...
	// Walk up from the resolved path to find the store entry.
	dir := filepath.Dir(resolved)
	nixStore := filepath.Clean("/nix/store")
	for dir != "/" && dir != "." {
		parent := filepath.Dir(dir)
		if parent == nixStore {
			return dir
		}
		dir = parent
	}

	return ""
}

func repoFromHomepage(homepage string) string {
	if repo, ok := strings.CutPrefix(homepage, "https://github.com/"); ok {
		return strings.TrimSuffix(repo, "/")
	}
	return ""
}

func Generate(config Config, discovered []DiscoveredPlugin) Marketplace {
	m := Marketplace{
		Name:  config.Name,
		Owner: config.Owner,
	}

	if config.Description != "" {
		m.Metadata = &Metadata{
			Description: config.Description,
		}
	}

	for _, dp := range discovered {
		meta := config.Plugins[dp.Name]

		serverType := dp.Type
		if serverType == "" {
			serverType = "stdio"
		}

		mcpServer := map[string]any{
			"type":    serverType,
			"command": dp.Command,
		}
		if len(dp.Args) > 0 {
			mcpServer["args"] = dp.Args
		}

		var source any
		repo := meta.Repo
		if repo == "" {
			repo = repoFromHomepage(meta.Homepage)
		}
		if repo != "" {
			source = GitHubSource{Source: "github", Repo: repo}
		} else {
			s := "./" + dp.Name
			if dp.StorePath != "" {
				s = dp.StorePath
			}
			source = s
		}

		strict := false
		plugin := Plugin{
			Name:       dp.Name,
			Source:     source,
			Strict:     &strict,
			McpServers: map[string]any{dp.Name: mcpServer},
		}

		applyMeta(&plugin, meta)
		m.Plugins = append(m.Plugins, plugin)
	}

	sort.Slice(m.Plugins, func(i, j int) bool {
		return m.Plugins[i].Name < m.Plugins[j].Name
	})

	return m
}

func applyMeta(p *Plugin, meta PluginMeta) {
	if meta.Description != "" {
		p.Description = meta.Description
	}

	if meta.Version != "" {
		p.Version = meta.Version
	}

	if meta.Homepage != "" {
		p.Homepage = meta.Homepage
	}
	if meta.Category != "" {
		p.Category = meta.Category
	}
	if len(meta.Tags) > 0 {
		p.Tags = meta.Tags
	}
}

func Write(m Marketplace, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling marketplace: %w", err)
	}

	data = append(data, '\n')
	return os.WriteFile(outputPath, data, 0o644)
}
