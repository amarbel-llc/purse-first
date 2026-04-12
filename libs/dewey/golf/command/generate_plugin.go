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

func (u *Utility) buildPluginManifest() pluginManifest {
	cmdName := u.Name
	if u.MCPBinary != "" {
		cmdName = u.MCPBinary
	}

	manifest := pluginManifest{
		Name:        u.Name,
		Description: u.PluginDescription,
		McpServers: map[string]pluginMcpServer{
			u.Name: {
				Type:    "stdio",
				Command: cmdName,
				Args:    u.MCPArgs,
			},
		},
		Skills: u.pluginSkills,
	}

	if u.PluginAuthor != "" {
		manifest.Author = &pluginAuthor{Name: u.PluginAuthor}
	}

	return manifest
}

// WritePluginJSON marshals the plugin manifest as indented JSON to w.
// No files are written; no side effects beyond the write.
func (u *Utility) WritePluginJSON(w io.Writer) error {
	manifest := u.buildPluginManifest()

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	_, err = w.Write(data)
	return err
}

// GeneratePlugin writes a plugin.json manifest to {dir}/{u.Name}/.claude-plugin/plugin.json.
func (u *Utility) GeneratePlugin(dir string) error {
	manifest := u.buildPluginManifest()

	pluginDir := filepath.Join(dir, u.Name, ".claude-plugin")
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
