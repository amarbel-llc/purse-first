package packagetoml

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// pluginMcpServer is the JSON representation of an MCP server in plugin.json.
type pluginMcpServer struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

// pluginAuthor is the JSON representation of an author in plugin.json.
type pluginAuthor struct {
	Name string `json:"name"`
}

// pluginHookCommand is a single hook command entry in plugin.json.
type pluginHookCommand struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

// pluginHookMatcher groups hooks by their matcher pattern in plugin.json.
type pluginHookMatcher struct {
	Matcher string              `json:"matcher"`
	Hooks   []pluginHookCommand `json:"hooks"`
}

// pluginManifest is the full plugin.json structure.
type pluginManifest struct {
	Name        string                        `json:"name"`
	Description string                        `json:"description,omitempty"`
	Author      *pluginAuthor                 `json:"author,omitempty"`
	McpServers  map[string]pluginMcpServer    `json:"mcpServers,omitempty"`
	Skills      []string                      `json:"skills,omitempty"`
	Hooks       map[string][]pluginHookMatcher `json:"hooks,omitempty"`
}

// GeneratePluginJSON generates a plugin.json from a parsed Package, discovers
// skills from skillsDir, copies them to the output, and writes plugin.json to
// {outputDir}/share/purse-first/{pkg.Name}/.claude-plugin/plugin.json.
func GeneratePluginJSON(pkg *Package, outputDir, skillsDir string) error {
	manifest := pluginManifest{
		Name:        pkg.Name,
		Description: pkg.Description,
	}

	if pkg.Author.Name != "" {
		manifest.Author = &pluginAuthor{Name: pkg.Author.Name}
	}

	if len(pkg.MCP) > 0 {
		manifest.McpServers = make(map[string]pluginMcpServer, len(pkg.MCP))
		for name, srv := range pkg.MCP {
			manifest.McpServers[name] = pluginMcpServer{
				Type:    "stdio",
				Command: srv.Command,
				Args:    srv.Args,
			}
		}
	}

	if len(pkg.Hooks) > 0 {
		manifest.Hooks = make(map[string][]pluginHookMatcher, len(pkg.Hooks))
		for event, hooks := range pkg.Hooks {
			var matchers []pluginHookMatcher
			for _, h := range hooks {
				matchers = append(matchers, pluginHookMatcher{
					Matcher: h.Matcher,
					Hooks: []pluginHookCommand{
						{
							Type:    "command",
							Command: h.Command,
							Timeout: h.Timeout,
						},
					},
				})
			}
			manifest.Hooks[event] = matchers
		}
	}

	packageDir := filepath.Join(outputDir, "share", "purse-first", pkg.Name)

	// Discover and copy skills
	if skillsDir != "" {
		skills, err := discoverSkills(skillsDir)
		if err != nil {
			return err
		}
		manifest.Skills = skills

		if len(skills) > 0 {
			dstSkillsDir := filepath.Join(packageDir, "skills")
			if err := copyDir(skillsDir, dstSkillsDir); err != nil {
				return fmt.Errorf("copying skills: %w", err)
			}
		}
	}

	claudePluginDir := filepath.Join(packageDir, ".claude-plugin")
	if err := os.MkdirAll(claudePluginDir, 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling plugin.json: %w", err)
	}
	data = append(data, '\n')

	pluginPath := filepath.Join(claudePluginDir, "plugin.json")
	return os.WriteFile(pluginPath, data, 0o644)
}

// discoverSkills globs {skillsDir}/*/SKILL.md and returns sorted
// "./skills/{name}" entries.
func discoverSkills(skillsDir string) ([]string, error) {
	pattern := filepath.Join(skillsDir, "*", "SKILL.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("globbing skills: %w", err)
	}

	var skills []string
	for _, match := range matches {
		name := filepath.Base(filepath.Dir(match))
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
