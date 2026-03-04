package localplugin

// TODO(terminology): rename package localplugin → localpackage
// when breaking change lands.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	tap "github.com/amarbel-llc/purse-first/packages/tap-dancer/go"
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

// InstallLocalOptions configures the install-local command.
type InstallLocalOptions struct {
	Binary string // Go binary name under cmd/, triggers _generate
}

// InstallLocal sets up the local development environment: optionally generates
// plugin.json via _generate, discovers skills, and installs MCP servers.
func InstallLocal(w io.Writer, root string, opts InstallLocalOptions) error {
	tw := tap.NewWriter(w)

	pluginPath := filepath.Join(root, ".claude-plugin", "plugin.json")

	if opts.Binary != "" {
		tw.PlanAhead(3)

		generatedPath, err := runGenerate(root)
		if err != nil {
			tw.NotOk(fmt.Sprintf("generate plugin.json via _generate (%s)", opts.Binary), map[string]string{
				"error": err.Error(),
			})
			return err
		}
		tw.Ok(fmt.Sprintf("generate plugin.json via _generate (%s)", opts.Binary))
		pluginPath = generatedPath
	} else {
		tw.PlanAhead(2)
	}

	// Discover and update skills
	if err := Generate(root, pluginPath); err != nil {
		tw.NotOk("discover and update skills in plugin.json", map[string]string{
			"error": err.Error(),
		})
		return err
	}
	tw.Ok("discover and update skills in plugin.json")

	// Install MCP servers
	settingsPath := filepath.Join(root, ".claude", "settings.json")
	count, err := installMCPServers(pluginPath, settingsPath)
	if err != nil {
		tw.NotOk("install MCP servers to .claude/settings.json", map[string]string{
			"error": err.Error(),
		})
		return err
	}
	if count == 0 {
		tw.Skip("install MCP servers to .claude/settings.json", "no mcpServers declared")
	} else {
		tw.Ok(fmt.Sprintf("install MCP servers to .claude/settings.json (%d server%s)", count, plural(count)))
	}

	return nil
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
