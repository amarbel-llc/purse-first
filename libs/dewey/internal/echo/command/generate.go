package command

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// GenerateAll writes all artifacts (plugin manifest, mappings, hooks, manpages,
// and shell completions) to standard paths under dir.
//
// Output layout:
//
//	{dir}/share/purse-first/{name}/.claude-plugin/plugin.json
//	{dir}/share/purse-first/{name}/mappings.json (if any commands have MapsTools)
//	{dir}/share/purse-first/{name}/hooks/hooks.json (if any commands have MapsTools)
//	{dir}/share/purse-first/{name}/hooks/pre-tool-use (if any commands have MapsTools)
//	{dir}/share/man/man1/{name}.1
//	{dir}/share/man/man1/{name}-{cmd}.1 (per visible command)
//	{dir}/share/bash-completion/completions/{name}
//	{dir}/share/zsh/site-functions/_{name}
//	{dir}/share/fish/vendor_completions.d/{name}.fish
func (u *Utility) GenerateAll(dir string) error {
	return u.GenerateAllWithSkills(dir, "")
}

// GenerateAllWithSkills writes all artifacts like GenerateAll, and when
// skillsDir is non-empty, discovers skills by globbing {skillsDir}/*/SKILL.md,
// copies the skill directories into the output, and includes them in plugin.json.
func (u *Utility) GenerateAllWithSkills(dir, skillsDir string) error {
	purseDir := filepath.Join(dir, "share", "purse-first")

	if skillsDir != "" {
		skills, err := discoverSkills(skillsDir)
		if err != nil {
			return fmt.Errorf("discovering skills: %w", err)
		}

		u.pluginSkills = skills

		// Copy skills into {dir}/share/purse-first/{name}/skills/
		dst := filepath.Join(purseDir, u.Name, "skills")
		if err := copyDir(skillsDir, dst); err != nil {
			return fmt.Errorf("copying skills: %w", err)
		}
	}

	if err := u.GeneratePlugin(purseDir); err != nil {
		return err
	}

	if err := u.GenerateMappings(purseDir); err != nil {
		return err
	}

	if err := u.GenerateHooks(purseDir); err != nil {
		return err
	}

	if err := u.GenerateManpages(dir); err != nil {
		return err
	}

	if err := u.InstallExtraManpages(dir); err != nil {
		return err
	}

	return u.GenerateCompletions(dir)
}

// InstallExtraManpages copies each ExtraManpages entry from its source fs.FS
// to {dir}/share/man/man{Section}/{Name}. The framework does not parse or
// modify the file contents — bytes are written verbatim.
func (u *Utility) InstallExtraManpages(dir string) error {
	for i, mf := range u.ExtraManpages {
		if mf.Source == nil {
			return fmt.Errorf("ExtraManpages[%d]: Source is nil", i)
		}
		if mf.Path == "" {
			return fmt.Errorf("ExtraManpages[%d]: Path is empty", i)
		}
		if mf.Section <= 0 {
			return fmt.Errorf("ExtraManpages[%d]: Section must be > 0", i)
		}
		if mf.Name == "" {
			return fmt.Errorf("ExtraManpages[%d]: Name is empty", i)
		}

		data, err := fs.ReadFile(mf.Source, mf.Path)
		if err != nil {
			return fmt.Errorf("ExtraManpages[%d]: reading %s: %w", i, mf.Path, err)
		}

		manDir := filepath.Join(dir, "share", "man", fmt.Sprintf("man%d", mf.Section))
		if err := os.MkdirAll(manDir, 0o755); err != nil {
			return fmt.Errorf("ExtraManpages[%d]: creating %s: %w", i, manDir, err)
		}

		dst := filepath.Join(manDir, mf.Name)
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return fmt.Errorf("ExtraManpages[%d]: writing %s: %w", i, dst, err)
		}
	}
	return nil
}

// HandleGeneratePlugin dispatches generate-plugin based on args:
//   - 0 args: write all artifacts to the current working directory
//   - 1 arg "-": write plugin.json as JSON to stdout (no files)
//   - 1 arg other: write all artifacts to the given directory
//   - >1 args: error
//
// The --skills-dir flag is parsed from args when present.
func (u *Utility) HandleGeneratePlugin(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("generate-plugin", flag.ContinueOnError)
	skillsDir := fs.String("skills-dir", "", "path to skills directory")

	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()

	switch len(remaining) {
	case 0:
		return u.GenerateAllWithSkills(".", *skillsDir)
	case 1:
		if remaining[0] == "-" {
			return u.WritePluginJSON(stdout)
		}
		return u.GenerateAllWithSkills(remaining[0], *skillsDir)
	default:
		return fmt.Errorf("generate-plugin: expected 0 or 1 arguments, got %d", len(remaining))
	}
}
