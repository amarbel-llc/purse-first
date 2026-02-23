package command

import (
	"fmt"
	"path/filepath"
)

// GenerateAll writes all artifacts (plugin manifest, mappings, manpages,
// and shell completions) to standard paths under dir.
//
// Output layout:
//
//	{dir}/share/purse-first/{name}/plugin.json
//	{dir}/share/purse-first/{name}/mappings.json (if any commands have MapsTools)
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

	if err := a.GenerateManpages(dir); err != nil {
		return err
	}

	return a.GenerateCompletions(dir)
}
