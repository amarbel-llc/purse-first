package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateManpageApp(t *testing.T) {
	app := NewApp("grit", "Git operations MCP server")
	app.Version = "0.1.0"
	app.Description.Long = "An MCP server exposing git operations."
	app.Examples = []Example{
		{
			Description: "Stage and commit changes",
			Command:     "grit add --repo_path=. --paths='[\"main.go\"]'\ngrit commit --repo_path=. --message='initial'",
		},
	}

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show working tree status"},
	})
	app.AddCommand(&Command{
		Name:   "generate-all",
		Hidden: true,
	})

	dir := t.TempDir()
	if err := app.GenerateManpages(dir); err != nil {
		t.Fatalf("GenerateManpages: %v", err)
	}

	appPage, err := os.ReadFile(filepath.Join(dir, "share", "man", "man1", "grit.1"))
	if err != nil {
		t.Fatalf("read grit.1: %v", err)
	}

	content := string(appPage)
	if !strings.Contains(content, ".TH GRIT 1") {
		t.Error("missing .TH header")
	}
	if !strings.Contains(content, "Git operations MCP server") {
		t.Error("missing short description in NAME")
	}
	if !strings.Contains(content, "An MCP server exposing git operations.") {
		t.Error("missing long description in DESCRIPTION")
	}
	if !strings.Contains(content, "status") {
		t.Error("missing status in COMMANDS")
	}
	if strings.Contains(content, "generate-all") {
		t.Error("hidden command should not appear in manpage")
	}

	// Task 4: SYNOPSIS
	if !strings.Contains(content, ".SH SYNOPSIS") {
		t.Error("missing SYNOPSIS section")
	}
	if !strings.Contains(content, ".I command") {
		t.Error("missing command placeholder in SYNOPSIS")
	}

	// Task 5: EXAMPLES
	if !strings.Contains(content, ".SH EXAMPLES") {
		t.Error("missing EXAMPLES section")
	}
	if !strings.Contains(content, "Stage and commit changes") {
		t.Error("missing app example description")
	}
	if !strings.Contains(content, "grit add") {
		t.Error("missing app example command")
	}

	// Task 6: SEE ALSO
	if !strings.Contains(content, ".SH SEE ALSO") {
		t.Error("missing SEE ALSO section")
	}
	if !strings.Contains(content, "grit-status (1)") {
		t.Error("missing cross-reference to subcommand page")
	}
}

func TestCommandExamplesField(t *testing.T) {
	cmd := &Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		Examples: []Example{
			{
				Description: "Check status of current directory",
				Command:     "grit status --repo_path=.",
			},
			{
				Description: "Check with output",
				Command:     "grit status --repo_path=/tmp/repo",
				Output:      `{"branch": "main", "clean": true}`,
			},
		},
	}

	if len(cmd.Examples) != 2 {
		t.Fatalf("expected 2 examples, got %d", len(cmd.Examples))
	}
	if cmd.Examples[0].Description != "Check status of current directory" {
		t.Error("wrong example description")
	}
	if cmd.Examples[1].Output == "" {
		t.Error("expected non-empty output on second example")
	}
}

func TestGenerateManpageCommand(t *testing.T) {
	app := NewApp("grit", "Git operations")

	app.AddCommand(&Command{
		Name: "status",
		Description: Description{
			Short: "Show working tree status",
			Long:  "Show working tree status with machine-readable output.",
		},
		Params: []Param{
			{Name: "repo_path", Type: String, Description: "Path to the git repository", Required: true},
			{Name: "verbose", Type: Bool, Description: "Show verbose output", Default: false},
		},
		Examples: []Example{
			{
				Description: "Check status of current directory",
				Command:     "grit status --repo_path=.",
			},
			{
				Description: "Check with JSON output",
				Command:     "grit status --repo_path=/tmp/repo",
				Output:      `{"branch": "main"}`,
			},
		},
	})

	dir := t.TempDir()
	if err := app.GenerateManpages(dir); err != nil {
		t.Fatalf("GenerateManpages: %v", err)
	}

	cmdPage, err := os.ReadFile(filepath.Join(dir, "share", "man", "man1", "grit-status.1"))
	if err != nil {
		t.Fatalf("read grit-status.1: %v", err)
	}

	content := string(cmdPage)
	if !strings.Contains(content, ".TH GRIT-STATUS 1") {
		t.Error("missing .TH header")
	}
	if !strings.Contains(content, "repo_path") {
		t.Error("missing repo_path in OPTIONS")
	}
	if !strings.Contains(content, "(required)") {
		t.Error("missing required marker")
	}
	if !strings.Contains(content, "Path to the git repository") {
		t.Error("missing param description")
	}

	// EXAMPLES assertions (Task 2)
	if !strings.Contains(content, ".SH EXAMPLES") {
		t.Error("missing EXAMPLES section")
	}
	if !strings.Contains(content, "Check status of current directory") {
		t.Error("missing example description")
	}
	if !strings.Contains(content, "grit status --repo_path=.") {
		t.Error("missing example command")
	}
	if !strings.Contains(content, `{"branch": "main"}`) {
		t.Error("missing example output")
	}
	if !strings.Contains(content, ".nf") {
		t.Error("missing .nf (no-fill) block")
	}
	if !strings.Contains(content, ".fi") {
		t.Error("missing .fi (end no-fill) block")
	}

	// SEE ALSO assertions (Task 3)
	if !strings.Contains(content, ".SH SEE ALSO") {
		t.Error("missing SEE ALSO section")
	}
	if !strings.Contains(content, "grit (1)") {
		t.Error("missing back-reference to main app page")
	}
}

func TestGenerateManpageCommandNoExamples(t *testing.T) {
	app := NewApp("grit", "Git operations")
	app.AddCommand(&Command{
		Name:        "log",
		Description: Description{Short: "Show commit history"},
	})

	dir := t.TempDir()
	if err := app.GenerateManpages(dir); err != nil {
		t.Fatalf("GenerateManpages: %v", err)
	}

	cmdPage, err := os.ReadFile(filepath.Join(dir, "share", "man", "man1", "grit-log.1"))
	if err != nil {
		t.Fatalf("read grit-log.1: %v", err)
	}

	if strings.Contains(string(cmdPage), ".SH EXAMPLES") {
		t.Error("EXAMPLES section should not appear when no examples defined")
	}
}

func TestGenerateManpageAppNoExamples(t *testing.T) {
	app := NewApp("mytool", "A simple tool")
	app.Version = "0.1.0"
	app.AddCommand(&Command{
		Name:        "run",
		Description: Description{Short: "Run the tool"},
	})

	dir := t.TempDir()
	if err := app.GenerateManpages(dir); err != nil {
		t.Fatalf("GenerateManpages: %v", err)
	}

	appPage, err := os.ReadFile(filepath.Join(dir, "share", "man", "man1", "mytool.1"))
	if err != nil {
		t.Fatalf("read mytool.1: %v", err)
	}

	if strings.Contains(string(appPage), ".SH EXAMPLES") {
		t.Error("EXAMPLES section should not appear when no examples defined")
	}
}
