package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateCompletionsBash(t *testing.T) {
	app := NewApp("grit", "Git operations")

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		Params: []Param{
			{Name: "repo_path", Type: String, Description: "Path to repo", Required: true},
		},
	})
	app.AddCommand(&Command{
		Name:        "diff",
		Description: Description{Short: "Show changes"},
	})
	app.AddCommand(&Command{Name: "hidden", Hidden: true})

	dir := t.TempDir()
	if err := app.GenerateCompletions(dir); err != nil {
		t.Fatalf("GenerateCompletions: %v", err)
	}

	bashPath := filepath.Join(dir, "share", "bash-completion", "completions", "grit")
	data, err := os.ReadFile(bashPath)
	if err != nil {
		t.Fatalf("read bash completion: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "status") {
		t.Error("bash completion missing status command")
	}
	if !strings.Contains(content, "diff") {
		t.Error("bash completion missing diff command")
	}
	if strings.Contains(content, "hidden") {
		t.Error("bash completion should not contain hidden commands")
	}
	if !strings.Contains(content, "repo_path") {
		t.Error("bash completion missing repo_path flag")
	}
}

func TestGenerateCompletionsBashShortFlags(t *testing.T) {
	app := NewApp("grit", "Git operations")

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		Params: []Param{
			{Name: "repo_path", Type: String, Description: "Path to repo", Required: true},
			{Name: "verbose", Type: Bool, Description: "Verbose output", Short: 'v'},
		},
	})

	dir := t.TempDir()
	if err := app.GenerateCompletions(dir); err != nil {
		t.Fatalf("GenerateCompletions: %v", err)
	}

	bashPath := filepath.Join(dir, "share", "bash-completion", "completions", "grit")
	data, err := os.ReadFile(bashPath)
	if err != nil {
		t.Fatalf("read bash completion: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "-v") {
		t.Error("bash completion missing short flag -v")
	}
	if !strings.Contains(content, "--repo_path") {
		t.Error("bash completion missing long flag --repo_path")
	}
}

func TestGenerateCompletionsFishShortFlags(t *testing.T) {
	app := NewApp("grit", "Git operations")

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		Params: []Param{
			{Name: "verbose", Type: Bool, Description: "Verbose output", Short: 'v'},
		},
	})

	dir := t.TempDir()
	if err := app.GenerateCompletions(dir); err != nil {
		t.Fatalf("GenerateCompletions: %v", err)
	}

	fishPath := filepath.Join(dir, "share", "fish", "vendor_completions.d", "grit.fish")
	data, err := os.ReadFile(fishPath)
	if err != nil {
		t.Fatalf("read fish completion: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "-s v") {
		t.Error("fish completion missing short flag -s v for verbose")
	}
}

func TestGenerateCompletionsZsh(t *testing.T) {
	app := NewApp("grit", "Git operations")
	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
	})

	dir := t.TempDir()
	if err := app.GenerateCompletions(dir); err != nil {
		t.Fatalf("GenerateCompletions: %v", err)
	}

	zshPath := filepath.Join(dir, "share", "zsh", "site-functions", "_grit")
	data, err := os.ReadFile(zshPath)
	if err != nil {
		t.Fatalf("read zsh completion: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "#compdef grit") {
		t.Error("zsh completion missing #compdef header")
	}
	if !strings.Contains(content, "status") {
		t.Error("zsh completion missing status command")
	}
}

func TestGenerateCompletionsFish(t *testing.T) {
	app := NewApp("grit", "Git operations")
	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
	})

	dir := t.TempDir()
	if err := app.GenerateCompletions(dir); err != nil {
		t.Fatalf("GenerateCompletions: %v", err)
	}

	fishPath := filepath.Join(dir, "share", "fish", "vendor_completions.d", "grit.fish")
	data, err := os.ReadFile(fishPath)
	if err != nil {
		t.Fatalf("read fish completion: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "complete -c grit") {
		t.Error("fish completion missing complete -c header")
	}
	if !strings.Contains(content, "status") {
		t.Error("fish completion missing status command")
	}
}
