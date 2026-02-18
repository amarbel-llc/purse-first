package command

import "path/filepath"

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
	purseDir := filepath.Join(dir, "share", "purse-first")

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
