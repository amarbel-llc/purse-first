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

func TestGenerateCompletionsCompleterBash(t *testing.T) {
	app := NewApp("myapp", "My app")
	app.AddCommand(&Command{
		Name:        "deploy",
		Description: Description{Short: "Deploy app"},
		Params: []Param{
			{Name: "target", Type: String, Description: "Deploy target"},
			{
				Name:        "env",
				Type:        String,
				Description: "Target environment",
				Completer:   func() map[string]string { return nil },
			},
		},
	})

	dir := t.TempDir()
	if err := app.GenerateCompletions(dir); err != nil {
		t.Fatalf("GenerateCompletions: %v", err)
	}

	bashPath := filepath.Join(dir, "share", "bash-completion", "completions", "myapp")
	data, err := os.ReadFile(bashPath)
	if err != nil {
		t.Fatalf("read bash completion: %v", err)
	}

	content := string(data)
	// Should have a case block for --env that calls __complete
	if !strings.Contains(content, "myapp __complete --command deploy --param env") {
		t.Error("bash completion should call __complete for param with Completer")
	}
	// Should still have flag completions in the default case
	if !strings.Contains(content, "--target") {
		t.Error("bash completion should still list flags")
	}
}

func TestGenerateCompletionsCompleterFish(t *testing.T) {
	app := NewApp("myapp", "My app")
	app.AddCommand(&Command{
		Name:        "deploy",
		Description: Description{Short: "Deploy app"},
		Params: []Param{
			{Name: "target", Type: String, Description: "Deploy target"},
			{
				Name:        "env",
				Type:        String,
				Description: "Target environment",
				Completer:   func() map[string]string { return nil },
			},
		},
	})

	dir := t.TempDir()
	if err := app.GenerateCompletions(dir); err != nil {
		t.Fatalf("GenerateCompletions: %v", err)
	}

	fishPath := filepath.Join(dir, "share", "fish", "vendor_completions.d", "myapp.fish")
	data, err := os.ReadFile(fishPath)
	if err != nil {
		t.Fatalf("read fish completion: %v", err)
	}

	content := string(data)
	// env param should have -ra with __complete callout
	if !strings.Contains(content, "myapp __complete --command deploy --param env") {
		t.Error("fish completion should call __complete for param with Completer")
	}
	// target param should NOT have -ra
	targetLine := ""
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, "-l target") {
			targetLine = line
			break
		}
	}
	if strings.Contains(targetLine, "__complete") {
		t.Error("fish completion should not call __complete for param without Completer")
	}
}

func TestGenerateCompletionsNoCompleterNoChange(t *testing.T) {
	// Verify that params without Completer produce the same output as before
	app := NewApp("grit", "Git operations")
	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		Params: []Param{
			{Name: "repo_path", Type: String, Description: "Path to repo"},
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
	if strings.Contains(content, "__complete") {
		t.Error("bash completion should not reference __complete when no params have Completer")
	}
	if !strings.Contains(content, "--repo_path") {
		t.Error("bash completion should still list flags normally")
	}
}

func TestGenerateCompletionsPassthroughNoFlags(t *testing.T) {
	app := NewApp("myapp", "My app")

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		Params: []Param{
			{Name: "verbose", Type: Bool, Description: "Verbose output"},
		},
	})
	app.AddCommand(&Command{
		Name:            "exec-claude",
		Description:     Description{Short: "Execute claude"},
		PassthroughArgs: true,
	})

	dir := t.TempDir()
	if err := app.GenerateCompletions(dir); err != nil {
		t.Fatalf("GenerateCompletions: %v", err)
	}

	// Bash: passthrough command should appear in subcommand list but not in flag case
	bashPath := filepath.Join(dir, "share", "bash-completion", "completions", "myapp")
	data, err := os.ReadFile(bashPath)
	if err != nil {
		t.Fatalf("read bash completion: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "exec-claude") {
		t.Error("bash completion missing exec-claude in subcommand list")
	}
	// The case block for flags should NOT have an exec-claude entry
	if strings.Contains(content, "exec-claude)\n") {
		t.Error("bash completion should not have flag completions for passthrough command")
	}

	// Fish: passthrough should appear as subcommand but have no per-flag completions
	fishPath := filepath.Join(dir, "share", "fish", "vendor_completions.d", "myapp.fish")
	data, err = os.ReadFile(fishPath)
	if err != nil {
		t.Fatalf("read fish completion: %v", err)
	}
	content = string(data)
	if !strings.Contains(content, "exec-claude") {
		t.Error("fish completion missing exec-claude in subcommand list")
	}
	if strings.Contains(content, "__fish_seen_subcommand_from exec-claude") {
		t.Error("fish completion should not have flag completions for passthrough command")
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
