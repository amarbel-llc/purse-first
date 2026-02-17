package command

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateAll(t *testing.T) {
	app := NewApp("grit", "Git operations")
	app.Version = "0.1.0"

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		Params: []Param{
			{Name: "repo_path", Type: String, Description: "Path to repo", Required: true},
		},
		MapsBash: []BashMapping{
			{Prefixes: []string{"git status"}, UseWhen: "checking status"},
		},
	})

	dir := t.TempDir()
	if err := app.GenerateAll(dir); err != nil {
		t.Fatalf("GenerateAll: %v", err)
	}

	expected := []string{
		filepath.Join("share", "purse-first", "grit", "plugin.json"),
		filepath.Join("share", "purse-first", "grit", "mappings.json"),
		filepath.Join("share", "man", "man1", "grit.1"),
		filepath.Join("share", "man", "man1", "grit-status.1"),
		filepath.Join("share", "bash-completion", "completions", "grit"),
		filepath.Join("share", "zsh", "site-functions", "_grit"),
		filepath.Join("share", "fish", "vendor_completions.d", "grit.fish"),
	}

	for _, rel := range expected {
		path := filepath.Join(dir, rel)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file missing: %s", rel)
		}
	}
}
