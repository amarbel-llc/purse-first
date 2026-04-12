package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateCompletionsBash(t *testing.T) {
	app := NewUtility("grit", "Git operations")

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		OldParams: []OldParam{
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
	app := NewUtility("grit", "Git operations")

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		OldParams: []OldParam{
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
	app := NewUtility("grit", "Git operations")

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		OldParams: []OldParam{
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
	app := NewUtility("grit", "Git operations")
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
	app := NewUtility("myapp", "My app")
	app.AddCommand(&Command{
		Name:        "deploy",
		Description: Description{Short: "Deploy app"},
		OldParams: []OldParam{
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
	app := NewUtility("myapp", "My app")
	app.AddCommand(&Command{
		Name:        "deploy",
		Description: Description{Short: "Deploy app"},
		OldParams: []OldParam{
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
	app := NewUtility("grit", "Git operations")
	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		OldParams: []OldParam{
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
	app := NewUtility("myapp", "My app")

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		OldParams: []OldParam{
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
	app := NewUtility("grit", "Git operations")
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

func TestGenerateCompletionsAliases(t *testing.T) {
	app := NewUtility("spinclass", "Worktree session manager")
	app.Aliases = []string{"sc"}

	app.AddCommand(&Command{
		Name:        "start",
		Description: Description{Short: "Start a session"},
		OldParams: []OldParam{
			{Name: "pr", Type: String, Description: "PR number", Completer: func() map[string]string { return nil }},
		},
	})

	dir := t.TempDir()
	if err := app.GenerateCompletions(dir); err != nil {
		t.Fatalf("GenerateCompletions: %v", err)
	}

	// Bash: alias file exists and contains complete -F _spinclass sc
	bashPath := filepath.Join(dir, "share", "bash-completion", "completions", "sc")
	data, err := os.ReadFile(bashPath)
	if err != nil {
		t.Fatalf("read bash alias completion: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "complete -F _spinclass sc") {
		t.Error("bash alias completion missing complete -F _spinclass sc")
	}

	// Zsh: alias file exists
	zshPath := filepath.Join(dir, "share", "zsh", "site-functions", "_sc")
	data, err = os.ReadFile(zshPath)
	if err != nil {
		t.Fatalf("read zsh alias completion: %v", err)
	}
	content = string(data)
	if !strings.Contains(content, "compdef _spinclass sc") {
		t.Error("zsh alias completion missing compdef _spinclass sc")
	}

	// Fish: separate alias file with complete -c sc lines
	fishPath := filepath.Join(dir, "share", "fish", "vendor_completions.d", "sc.fish")
	data, err = os.ReadFile(fishPath)
	if err != nil {
		t.Fatalf("read fish alias completion: %v", err)
	}
	content = string(data)
	if !strings.Contains(content, "complete -c sc") {
		t.Error("fish alias completion missing complete -c sc")
	}
	// Fish alias completions should call the original binary for __complete
	if !strings.Contains(content, "spinclass __complete") {
		t.Error("fish alias completion should call spinclass __complete, not sc __complete")
	}
}

func TestGenerateCompletionsMultipleAliases(t *testing.T) {
	app := NewUtility("spinclass", "Worktree session manager")
	app.Aliases = []string{"sc", "spin"}

	app.AddCommand(&Command{
		Name:        "start",
		Description: Description{Short: "Start a session"},
	})

	dir := t.TempDir()
	if err := app.GenerateCompletions(dir); err != nil {
		t.Fatalf("GenerateCompletions: %v", err)
	}

	for _, alias := range []string{"sc", "spin"} {
		bashPath := filepath.Join(dir, "share", "bash-completion", "completions", alias)
		if _, err := os.Stat(bashPath); err != nil {
			t.Errorf("bash alias file missing for %s", alias)
		}

		zshPath := filepath.Join(dir, "share", "zsh", "site-functions", "_"+alias)
		if _, err := os.Stat(zshPath); err != nil {
			t.Errorf("zsh alias file missing for %s", alias)
		}

		fishPath := filepath.Join(dir, "share", "fish", "vendor_completions.d", alias+".fish")
		if _, err := os.Stat(fishPath); err != nil {
			t.Errorf("fish alias file missing for %s", alias)
		}
	}

	// Primary bash file should contain complete lines for both aliases
	primaryBash := filepath.Join(dir, "share", "bash-completion", "completions", "spinclass")
	data, err := os.ReadFile(primaryBash)
	if err != nil {
		t.Fatalf("read primary bash: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "complete -F _spinclass sc") {
		t.Error("primary bash missing complete line for sc alias")
	}
	if !strings.Contains(content, "complete -F _spinclass spin") {
		t.Error("primary bash missing complete line for spin alias")
	}

	// Primary zsh file should contain compdef lines for both aliases
	primaryZsh := filepath.Join(dir, "share", "zsh", "site-functions", "_spinclass")
	data, err = os.ReadFile(primaryZsh)
	if err != nil {
		t.Fatalf("read primary zsh: %v", err)
	}
	content = string(data)
	if !strings.Contains(content, "compdef _spinclass sc") {
		t.Error("primary zsh missing compdef line for sc alias")
	}
	if !strings.Contains(content, "compdef _spinclass spin") {
		t.Error("primary zsh missing compdef line for spin alias")
	}
}

func TestGenerateCompletionsNoAliasesUnchanged(t *testing.T) {
	app := NewUtility("grit", "Git operations")
	// No aliases set — verify no extra files or lines

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
	})

	dir := t.TempDir()
	if err := app.GenerateCompletions(dir); err != nil {
		t.Fatalf("GenerateCompletions: %v", err)
	}

	// Only the primary files should exist
	bashDir := filepath.Join(dir, "share", "bash-completion", "completions")
	entries, _ := os.ReadDir(bashDir)
	if len(entries) != 1 || entries[0].Name() != "grit" {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("expected only 'grit' in bash dir, got %v", names)
	}

	zshDir := filepath.Join(dir, "share", "zsh", "site-functions")
	entries, _ = os.ReadDir(zshDir)
	if len(entries) != 1 || entries[0].Name() != "_grit" {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("expected only '_grit' in zsh dir, got %v", names)
	}

	fishDir := filepath.Join(dir, "share", "fish", "vendor_completions.d")
	entries, _ = os.ReadDir(fishDir)
	if len(entries) != 1 || entries[0].Name() != "grit.fish" {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("expected only 'grit.fish' in fish dir, got %v", names)
	}

	// Primary bash should not contain any extra complete lines
	data, _ := os.ReadFile(filepath.Join(bashDir, "grit"))
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	completeCount := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "complete -F") {
			completeCount++
		}
	}
	if completeCount != 1 {
		t.Errorf("expected 1 complete -F line, got %d", completeCount)
	}
}

func TestGenerateCompletionsAliasesHiddenCommands(t *testing.T) {
	app := NewUtility("myapp", "My app")
	app.Aliases = []string{"ma"}

	app.AddCommand(&Command{
		Name:        "visible",
		Description: Description{Short: "A visible command"},
	})
	app.AddCommand(&Command{Name: "hidden", Hidden: true})

	dir := t.TempDir()
	if err := app.GenerateCompletions(dir); err != nil {
		t.Fatalf("GenerateCompletions: %v", err)
	}

	// Fish alias should have visible but not hidden
	fishPath := filepath.Join(dir, "share", "fish", "vendor_completions.d", "ma.fish")
	data, err := os.ReadFile(fishPath)
	if err != nil {
		t.Fatalf("read fish alias: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "visible") {
		t.Error("fish alias missing visible command")
	}
	if strings.Contains(content, "hidden") {
		t.Error("fish alias should not contain hidden command")
	}

	// Bash alias should also exclude hidden
	bashPath := filepath.Join(dir, "share", "bash-completion", "completions", "ma")
	data, err = os.ReadFile(bashPath)
	if err != nil {
		t.Fatalf("read bash alias: %v", err)
	}
	content = string(data)
	if strings.Contains(content, "hidden") {
		t.Error("bash alias should not contain hidden command")
	}
}

func TestGenerateCompletionsAliasesPassthrough(t *testing.T) {
	app := NewUtility("myapp", "My app")
	app.Aliases = []string{"ma"}

	app.AddCommand(&Command{
		Name:        "run",
		Description: Description{Short: "Run something"},
		OldParams: []OldParam{
			{Name: "verbose", Type: Bool, Description: "Verbose output"},
		},
	})
	app.AddCommand(&Command{
		Name:            "exec",
		Description:     Description{Short: "Execute raw"},
		PassthroughArgs: true,
	})

	dir := t.TempDir()
	if err := app.GenerateCompletions(dir); err != nil {
		t.Fatalf("GenerateCompletions: %v", err)
	}

	// Fish alias: exec should appear as subcommand but have no flag completions
	fishPath := filepath.Join(dir, "share", "fish", "vendor_completions.d", "ma.fish")
	data, err := os.ReadFile(fishPath)
	if err != nil {
		t.Fatalf("read fish alias: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "'__fish_use_subcommand' -a exec") {
		t.Error("fish alias missing exec in subcommand list")
	}
	if strings.Contains(content, "__fish_seen_subcommand_from exec") {
		t.Error("fish alias should not have flag completions for passthrough command")
	}

	// Bash alias: exec should appear but have no case block for flags
	bashPath := filepath.Join(dir, "share", "bash-completion", "completions", "ma")
	data, err = os.ReadFile(bashPath)
	if err != nil {
		t.Fatalf("read bash alias: %v", err)
	}
	content = string(data)
	if !strings.Contains(content, "exec") {
		t.Error("bash alias missing exec in command list")
	}
}

func TestGenerateCompletionsAliasesFishCompleter(t *testing.T) {
	app := NewUtility("myapp", "My app")
	app.Aliases = []string{"ma"}

	app.AddCommand(&Command{
		Name:        "deploy",
		Description: Description{Short: "Deploy app"},
		OldParams: []OldParam{
			{Name: "target", Type: String, Description: "Deploy target"},
			{
				Name:        "env",
				Type:        String,
				Description: "Environment",
				Completer:   func() map[string]string { return nil },
			},
		},
	})

	dir := t.TempDir()
	if err := app.GenerateCompletions(dir); err != nil {
		t.Fatalf("GenerateCompletions: %v", err)
	}

	fishPath := filepath.Join(dir, "share", "fish", "vendor_completions.d", "ma.fish")
	data, err := os.ReadFile(fishPath)
	if err != nil {
		t.Fatalf("read fish alias: %v", err)
	}
	content := string(data)

	// Alias fish file should call the primary binary for __complete, not the alias
	if !strings.Contains(content, "myapp __complete --command deploy --param env") {
		t.Error("fish alias should call myapp __complete, not ma __complete")
	}
	// Should use complete -c ma (the alias name)
	if !strings.Contains(content, "complete -c ma") {
		t.Error("fish alias completions should use complete -c ma")
	}
	// target param (no Completer) should not have __complete
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, "-l target") && strings.Contains(line, "__complete") {
			t.Error("fish alias should not call __complete for param without Completer")
		}
	}
}

func TestGenerateCompletionsAliasesPrimaryFileContainsAliasLines(t *testing.T) {
	app := NewUtility("spinclass", "Worktree session manager")
	app.Aliases = []string{"sc"}

	app.AddCommand(&Command{
		Name:        "start",
		Description: Description{Short: "Start a session"},
	})

	dir := t.TempDir()
	if err := app.GenerateCompletions(dir); err != nil {
		t.Fatalf("GenerateCompletions: %v", err)
	}

	// Primary bash file should contain the alias complete line
	bashPath := filepath.Join(dir, "share", "bash-completion", "completions", "spinclass")
	data, err := os.ReadFile(bashPath)
	if err != nil {
		t.Fatalf("read primary bash: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "complete -F _spinclass spinclass") {
		t.Error("primary bash missing complete line for spinclass itself")
	}
	if !strings.Contains(content, "complete -F _spinclass sc") {
		t.Error("primary bash missing complete line for sc alias")
	}

	// Primary zsh file should contain the alias compdef line
	zshPath := filepath.Join(dir, "share", "zsh", "site-functions", "_spinclass")
	data, err = os.ReadFile(zshPath)
	if err != nil {
		t.Fatalf("read primary zsh: %v", err)
	}
	content = string(data)
	if !strings.Contains(content, "_spinclass\n") {
		t.Error("primary zsh missing _spinclass invocation")
	}
	if !strings.Contains(content, "compdef _spinclass sc") {
		t.Error("primary zsh missing compdef line for sc alias")
	}
}

func TestGenerateCompletionsPositionalBash(t *testing.T) {
	app := NewUtility("sc", "Worktree manager")
	app.AddCommand(&Command{
		Name:        "start-gh_pr",
		Description: Description{Short: "Start from PR"},
		OldParams: []OldParam{
			{
				Name: "pr", Type: String, Required: true, Description: "PR number",
				Completer: func() map[string]string { return nil },
			},
			{Name: "verbose", Type: Bool, Description: "Verbose output"},
		},
	})

	dir := t.TempDir()
	if err := app.GenerateCompletions(dir); err != nil {
		t.Fatalf("GenerateCompletions: %v", err)
	}

	bashPath := filepath.Join(dir, "share", "bash-completion", "completions", "sc")
	data, err := os.ReadFile(bashPath)
	if err != nil {
		t.Fatalf("read bash completion: %v", err)
	}

	content := string(data)
	// Flag-style completion should still work
	if !strings.Contains(content, "--pr)\n") {
		t.Error("bash completion missing --pr flag trigger for Completer")
	}
	// Positional completion: __complete should be reachable via positional index
	if !strings.Contains(content, "case \"${_pos}\" in") {
		t.Error("bash completion missing positional index dispatch")
	}
	if !strings.Contains(content, "sc __complete --command start-gh_pr --param pr") {
		t.Error("bash completion missing __complete call for positional pr param")
	}
}

func TestGenerateCompletionsPositionalFish(t *testing.T) {
	app := NewUtility("sc", "Worktree manager")
	app.AddCommand(&Command{
		Name:        "start-gh_pr",
		Description: Description{Short: "Start from PR"},
		OldParams: []OldParam{
			{
				Name: "pr", Type: String, Required: true, Description: "PR number",
				Completer: func() map[string]string { return nil },
			},
			{Name: "verbose", Type: Bool, Description: "Verbose output"},
		},
	})

	dir := t.TempDir()
	if err := app.GenerateCompletions(dir); err != nil {
		t.Fatalf("GenerateCompletions: %v", err)
	}

	fishPath := filepath.Join(dir, "share", "fish", "vendor_completions.d", "sc.fish")
	data, err := os.ReadFile(fishPath)
	if err != nil {
		t.Fatalf("read fish completion: %v", err)
	}

	content := string(data)
	// Flag-style completion should exist
	if !strings.Contains(content, "-l pr") {
		t.Error("fish completion missing --pr flag completion")
	}
	// Positional completion: should have a rule without -l that calls __complete
	// when the flag form hasn't been used
	hasPositionalRule := false
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, "__fish_contains_opt pr") &&
			strings.Contains(line, "sc __complete --command start-gh_pr --param pr") &&
			!strings.Contains(line, "-l pr") {
			hasPositionalRule = true
			break
		}
	}
	if !hasPositionalRule {
		t.Error("fish completion missing positional completion rule for pr param")
	}
}

func TestGenerateCompletionsPositionalMultipleParams(t *testing.T) {
	app := NewUtility("myapp", "My app")
	app.AddCommand(&Command{
		Name:        "deploy",
		Description: Description{Short: "Deploy"},
		OldParams: []OldParam{
			{Name: "target", Type: String, Description: "Deploy target"},
			{
				Name: "env", Type: String, Description: "Environment",
				Completer: func() map[string]string { return nil },
			},
			{
				Name: "region", Type: String, Description: "Region",
				Completer: func() map[string]string { return nil },
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
	// env is the 2nd non-Bool param (index 1), region is the 3rd (index 2)
	if !strings.Contains(content, "1)\n") {
		t.Error("bash completion missing positional index 1 for env")
	}
	if !strings.Contains(content, "myapp __complete --command deploy --param env") {
		t.Error("bash completion missing __complete for positional env param")
	}
	if !strings.Contains(content, "2)\n") {
		t.Error("bash completion missing positional index 2 for region")
	}
	if !strings.Contains(content, "myapp __complete --command deploy --param region") {
		t.Error("bash completion missing __complete for positional region param")
	}
}

func TestGenerateCompletionsPositionalBoolSkipped(t *testing.T) {
	app := NewUtility("myapp", "My app")
	app.AddCommand(&Command{
		Name:        "cmd",
		Description: Description{Short: "A command"},
		OldParams: []OldParam{
			{
				Name: "flag", Type: Bool, Description: "A bool flag",
				Completer: func() map[string]string { return nil },
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
	// Bool params should not get positional completion
	if strings.Contains(content, "case \"${_pos}\"") {
		t.Error("bash completion should not have positional dispatch for Bool-only Completer params")
	}

	fishPath := filepath.Join(dir, "share", "fish", "vendor_completions.d", "myapp.fish")
	data, err = os.ReadFile(fishPath)
	if err != nil {
		t.Fatalf("read fish completion: %v", err)
	}

	content = string(data)
	if strings.Contains(content, "__fish_contains_opt flag") {
		t.Error("fish completion should not have positional rule for Bool param")
	}
}

func TestGenerateCompletionsPositionalFishAlias(t *testing.T) {
	app := NewUtility("spinclass", "Worktree manager")
	app.Aliases = []string{"sc"}
	app.AddCommand(&Command{
		Name:        "start-gh_pr",
		Description: Description{Short: "Start from PR"},
		OldParams: []OldParam{
			{
				Name: "pr", Type: String, Required: true, Description: "PR number",
				Completer: func() map[string]string { return nil },
			},
		},
	})

	dir := t.TempDir()
	if err := app.GenerateCompletions(dir); err != nil {
		t.Fatalf("GenerateCompletions: %v", err)
	}

	fishPath := filepath.Join(dir, "share", "fish", "vendor_completions.d", "sc.fish")
	data, err := os.ReadFile(fishPath)
	if err != nil {
		t.Fatalf("read fish alias completion: %v", err)
	}

	content := string(data)
	// Alias should have positional completion rule using primary binary name
	hasPositionalRule := false
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, "complete -c sc") &&
			strings.Contains(line, "__fish_contains_opt pr") &&
			strings.Contains(line, "spinclass __complete --command start-gh_pr --param pr") {
			hasPositionalRule = true
			break
		}
	}
	if !hasPositionalRule {
		t.Error("fish alias completion missing positional rule that calls spinclass __complete")
	}
}
