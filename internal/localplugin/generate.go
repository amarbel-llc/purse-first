package localplugin

// TODO(terminology): rename package localplugin → localpackage
// when breaking change lands.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func DiscoverSkills(root string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(root, "skills", "*", "SKILL.md"))
	if err != nil {
		return nil, fmt.Errorf("globbing skills: %w", err)
	}

	var skills []string
	for _, path := range matches {
		name := filepath.Base(filepath.Dir(path))
		skills = append(skills, "./skills/"+name)
	}

	return skills, nil
}

func Generate(root, pluginPath string) error {
	data, err := os.ReadFile(pluginPath)
	if err != nil {
		return fmt.Errorf("reading plugin.json: %w", err)
	}

	var plugin map[string]any
	if err := json.Unmarshal(data, &plugin); err != nil {
		return fmt.Errorf("parsing plugin.json: %w", err)
	}

	skills, err := DiscoverSkills(root)
	if err != nil {
		return err
	}

	if len(skills) > 0 {
		plugin["skills"] = skills
	} else {
		delete(plugin, "skills")
	}

	out, err := json.MarshalIndent(plugin, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling plugin.json: %w", err)
	}

	out = append(out, '\n')
	return os.WriteFile(pluginPath, out, 0o644)
}
