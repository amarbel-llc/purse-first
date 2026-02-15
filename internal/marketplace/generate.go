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

func DiscoverSkills(skillsDir string) ([]DiscoveredSkill, error) {
	matches, err := filepath.Glob(filepath.Join(skillsDir, "*", "SKILL.md"))
	if err != nil {
		return nil, fmt.Errorf("globbing skills: %w", err)
	}

	var skills []DiscoveredSkill
	for _, path := range matches {
		name := filepath.Base(filepath.Dir(path))
		relPath := filepath.Join("skills", name, "SKILL.md")
		skills = append(skills, DiscoveredSkill{Name: name, Path: relPath})
	}

	return skills, nil
}

func Generate(config Config, discovered []DiscoveredPlugin, skills []DiscoveredSkill) Marketplace {
	m := Marketplace{
		Name:  config.Name,
		Owner: config.Owner,
	}

	if config.Description != "" {
		m.Metadata = &Metadata{
			Description: config.Description,
		}
	}

	mcpServers := make(map[string]any, len(discovered))

	for _, dp := range discovered {
		serverType := dp.Type
		if serverType == "" {
			serverType = "stdio"
		}

		server := map[string]any{
			"type":    serverType,
			"command": dp.Command,
		}
		if len(dp.Args) > 0 {
			server["args"] = dp.Args
		}

		mcpServers[dp.Name] = server
	}

	if len(mcpServers) > 0 {
		var source any
		if config.Repo != "" {
			source = GitHubSource{Source: "github", Repo: config.Repo}
		} else {
			source = "."
		}

		strict := false
		plugin := Plugin{
			Name:        config.Name,
			Description: config.Description,
			Source:      source,
			Strict:      &strict,
			McpServers:  mcpServers,
		}

		if len(skills) > 0 {
			skillsMap := make(map[string]any, len(skills))
			for _, s := range skills {
				skillsMap[s.Name] = map[string]any{
					"path": s.Path,
				}
			}
			plugin.Skills = skillsMap
		}

		m.Plugins = append(m.Plugins, plugin)
	} else if len(skills) > 0 {
		skillsMap := make(map[string]any, len(skills))
		for _, s := range skills {
			skillsMap[s.Name] = map[string]any{
				"path": s.Path,
			}
		}

		var source any
		if config.Repo != "" {
			source = GitHubSource{Source: "github", Repo: config.Repo}
		} else {
			source = "."
		}

		strict := false
		plugin := Plugin{
			Name:        config.Name,
			Description: config.Description,
			Source:      source,
			Strict:      &strict,
			Skills:      skillsMap,
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
