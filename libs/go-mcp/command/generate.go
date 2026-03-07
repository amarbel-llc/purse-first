package command

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
)

// GenerateAll writes all artifacts (plugin manifest, mappings, hooks, manpages,
// and shell completions) to standard paths under dir.
//
// Output layout:
//
//	{dir}/share/purse-first/{name}/plugin.json
//	{dir}/share/purse-first/{name}/mappings.json (if any commands have MapsTools)
//	{dir}/share/purse-first/{name}/hooks/hooks.json (if any commands have MapsTools)
//	{dir}/share/purse-first/{name}/hooks/pre-tool-use (if any commands have MapsTools)
//	{dir}/share/man/man1/{name}.1
//	{dir}/share/man/man1/{name}-{cmd}.1 (per visible command)
//	{dir}/share/bash-completion/completions/{name}
//	{dir}/share/zsh/site-functions/_{name}
//	{dir}/share/fish/vendor_completions.d/{name}.fish
func (a *App) GenerateAll(dir string) error {
	return a.GenerateAllWithSkills(dir, "")
}

// GenerateAllWithSkills writes all artifacts like GenerateAll, and when
// skillsDir is non-empty, discovers skills by globbing {skillsDir}/*/SKILL.md,
// copies the skill directories into the output, and includes them in plugin.json.
func (a *App) GenerateAllWithSkills(dir, skillsDir string) error {
	purseDir := filepath.Join(dir, "share", "purse-first")

	if skillsDir != "" {
		skills, err := discoverSkills(skillsDir)
		if err != nil {
			return fmt.Errorf("discovering skills: %w", err)
		}

		a.pluginSkills = skills

		// Copy skills into {dir}/share/purse-first/{name}/skills/
		dst := filepath.Join(purseDir, a.Name, "skills")
		if err := copyDir(skillsDir, dst); err != nil {
			return fmt.Errorf("copying skills: %w", err)
		}
	}

	if err := a.GeneratePlugin(purseDir); err != nil {
		return err
	}

	if err := a.GenerateMappings(purseDir); err != nil {
		return err
	}

	if err := a.GenerateHooks(purseDir); err != nil {
		return err
	}

	if err := a.GenerateManpages(dir); err != nil {
		return err
	}

	return a.GenerateCompletions(dir)
}

// HandleGeneratePlugin dispatches generate-plugin based on args:
//   - 0 args: write all artifacts to the current working directory
//   - 1 arg "-": write plugin.json as JSON to stdout (no files)
//   - 1 arg other: write all artifacts to the given directory
//   - >1 args: error
//
// The --skills-dir flag is parsed from args when present.
func (a *App) HandleGeneratePlugin(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("generate-plugin", flag.ContinueOnError)
	skillsDir := fs.String("skills-dir", "", "path to skills directory")

	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()

	switch len(remaining) {
	case 0:
		return a.GenerateAllWithSkills(".", *skillsDir)
	case 1:
		if remaining[0] == "-" {
			return a.WritePluginJSON(stdout)
		}
		return a.GenerateAllWithSkills(remaining[0], *skillsDir)
	default:
		return fmt.Errorf("generate-plugin: expected 0 or 1 arguments, got %d", len(remaining))
	}
}
