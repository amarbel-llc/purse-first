package command

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

type pluginMcpServer struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

type pluginAuthor struct {
	Name string `json:"name"`
}

type pluginManifest struct {
	Name        string                     `json:"name"`
	Description string                     `json:"description,omitempty"`
	Author      *pluginAuthor              `json:"author,omitempty"`
	McpServers  map[string]pluginMcpServer `json:"mcpServers,omitempty"`
	Skills      []string                   `json:"skills,omitempty"`
}

// GeneratePlugin writes a plugin.json manifest to {dir}/{app.Name}/plugin.json.
func (a *App) GeneratePlugin(dir string) error {
	cmdName := a.Name
	if a.MCPBinary != "" {
		cmdName = a.MCPBinary
	}

	manifest := pluginManifest{
		Name:        a.Name,
		Description: a.PluginDescription,
		McpServers: map[string]pluginMcpServer{
			a.Name: {
				Type:    "stdio",
				Command: cmdName,
				Args:    a.MCPArgs,
			},
		},
		Skills: a.pluginSkills,
	}

	if a.PluginAuthor != "" {
		manifest.Author = &pluginAuthor{Name: a.PluginAuthor}
	}

	pluginDir := filepath.Join(dir, a.Name)
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0o644)
}

// discoverSkills globs {skillsDir}/*/SKILL.md and returns sorted "./skills/{name}" entries.
func discoverSkills(skillsDir string) ([]string, error) {
	pattern := filepath.Join(skillsDir, "*", "SKILL.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("globbing skills: %w", err)
	}

	var skills []string
	for _, match := range matches {
		// Extract the skill directory name from the match path
		skillDir := filepath.Dir(match)
		name := filepath.Base(skillDir)
		skills = append(skills, "./skills/"+name)
	}

	sort.Strings(skills)

	return skills, nil
}

// copyDir recursively copies a directory tree from src to dst.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("computing relative path: %w", err)
		}

		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		return copyFile(path, target)
	})
}

// copyFile copies a single file from src to dst, preserving permissions.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening %s: %w", src, err)
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", src, err)
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return fmt.Errorf("creating %s: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copying %s to %s: %w", src, dst, err)
	}

	return nil
}
