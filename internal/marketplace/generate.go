package marketplace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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

		pluginDir := filepath.Dir(path)
		pluginName := filepath.Base(pluginDir)
		storePath := resolveStorePath(path)

		skills := discoverPluginSkills(pluginDir)

		plugins = append(plugins, DiscoveredPlugin{
			Name:       pluginName,
			McpServers: p.McpServers,
			Skills:     skills,
			StorePath:  storePath,
		})
	}

	return plugins, nil
}

func discoverPluginSkills(pluginDir string) []DiscoveredSkill {
	matches, err := filepath.Glob(filepath.Join(pluginDir, "skills", "*", "SKILL.md"))
	if err != nil {
		return nil
	}

	var skills []DiscoveredSkill
	for _, path := range matches {
		skillName := filepath.Base(filepath.Dir(path))
		relPath := "./" + filepath.Join("skills", skillName)
		skills = append(skills, DiscoveredSkill{Name: skillName, Path: relPath})
	}

	return skills
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
		mcpServers := make(map[string]any)
		var skills []string

		for name, srv := range dp.McpServers {
			serverType := srv.Type
			if serverType == "" {
				serverType = "stdio"
			}

			server := map[string]any{
				"type":    serverType,
				"command": srv.Command,
			}
			if len(srv.Args) > 0 {
				server["args"] = srv.Args
			}

			mcpServers[name] = server
		}

		for _, s := range dp.Skills {
			skills = append(skills, s.Path)
		}

		// The self-plugin (marketplace's own repo) should not specify
		// component specs in the marketplace entry — the installed
		// plugin.json is the sole authority for its components.
		isSelf := dp.Name == config.Name

		if len(mcpServers) == 0 && len(skills) == 0 && !isSelf {
			continue
		}

		meta := config.Plugins[dp.Name]

		var source any
		switch {
		case meta.Repo != "":
			source = GitHubSource{Source: "github", Repo: meta.Repo}
		case config.Repo != "":
			source = GitHubSource{Source: "github", Repo: config.Repo}
		default:
			source = "."
		}

		description := meta.Description
		if description == "" {
			description = config.Description
		}

		strict := false
		plugin := Plugin{
			Name:        dp.Name,
			Description: description,
			Version:     meta.Version,
			Source:      source,
			Category:    meta.Category,
			Homepage:    meta.Homepage,
			Tags:        meta.Tags,
			Strict:      &strict,
		}

		if len(mcpServers) > 0 {
			plugin.McpServers = mcpServers
		}
		if len(skills) > 0 && !isSelf {
			plugin.Skills = skills
		}

		m.Plugins = append(m.Plugins, plugin)
	}

	return m
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
