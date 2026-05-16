package command

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
)

func TestGenerateManpageApp(t *testing.T) {
	app := NewUtility("grit", "Git operations MCP server")
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

	// Task 7: COMMANDS cross-reference
	if !strings.Contains(content, ".BR status (1)") {
		t.Error("COMMANDS should cross-reference subcommand manpage with (1)")
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
	app := NewUtility("grit", "Git operations")

	app.AddCommand(&Command{
		Name: "status",
		Description: Description{
			Short: "Show working tree status",
			Long:  "Show working tree status with machine-readable output.",
		},
		OldParams: []OldParam{
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
	app := NewUtility("grit", "Git operations")
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
	app := NewUtility("mytool", "A simple tool")
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

func TestGenerateManpageShortFlags(t *testing.T) {
	app := NewUtility("grit", "Git operations")

	app.AddCommand(&Command{
		Name: "status",
		Description: Description{
			Short: "Show working tree status",
			Long:  "Show working tree status with machine-readable output.",
		},
		OldParams: []OldParam{
			{Name: "repo_path", Type: String, Description: "Path to the git repository", Required: true},
			{Name: "verbose", Type: Bool, Description: "Show verbose output", Short: 'v'},
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

	// Short flag should appear in OPTIONS
	if !strings.Contains(content, "-v") {
		t.Error("OPTIONS should include short flag -v")
	}
	if !strings.Contains(content, "--verbose") {
		t.Error("OPTIONS should include long flag --verbose")
	}

	// Param without short flag should only show long form
	if !strings.Contains(content, "--repo_path") {
		t.Error("OPTIONS should include long flag --repo_path")
	}

	// SYNOPSIS should include short flag
	if !strings.Contains(content, "-v") {
		t.Error("SYNOPSIS should include short flag -v")
	}
}

func TestGenerateManpagePassthroughArgs(t *testing.T) {
	app := NewUtility("myapp", "My app")
	app.AddCommand(&Command{
		Name:            "exec-claude",
		Description:     Description{Short: "Execute claude with args"},
		PassthroughArgs: true,
		OldParams: []OldParam{
			{Name: "ignored", Type: String, Description: "This param should not appear"},
		},
	})

	dir := t.TempDir()
	if err := app.GenerateManpages(dir); err != nil {
		t.Fatalf("GenerateManpages: %v", err)
	}

	cmdPage, err := os.ReadFile(filepath.Join(dir, "share", "man", "man1", "myapp-exec-claude.1"))
	if err != nil {
		t.Fatalf("read manpage: %v", err)
	}

	content := string(cmdPage)
	if !strings.Contains(content, "args...") {
		t.Error("passthrough command SYNOPSIS should show [args...]")
	}
	if strings.Contains(content, "--ignored") {
		t.Error("passthrough command should not list individual flags in SYNOPSIS")
	}
	if strings.Contains(content, ".SH OPTIONS") {
		t.Error("passthrough command should not have OPTIONS section")
	}
}

func TestManpageSectionOrdering(t *testing.T) {
	app := NewUtility("demo", "Demo tool")
	app.Version = "1.0.0"
	app.Description.Long = "A demonstration tool."
	app.Examples = []Example{
		{Description: "Run a workflow", Command: "demo greet --name=world"},
	}

	app.AddCommand(&Command{
		Name:        "greet",
		Description: Description{Short: "Say hello", Long: "Greet someone by name."},
		OldParams:   []OldParam{{Name: "name", Type: String, Description: "Who to greet", Required: true}},
		Examples: []Example{
			{Description: "Basic greeting", Command: "demo greet --name=world", Output: "Hello, world!"},
		},
	})

	dir := t.TempDir()
	if err := app.GenerateManpages(dir); err != nil {
		t.Fatalf("GenerateManpages: %v", err)
	}

	// Verify app page section ordering
	appPage, _ := os.ReadFile(filepath.Join(dir, "share", "man", "man1", "demo.1"))
	appContent := string(appPage)

	sections := []string{".SH NAME", ".SH SYNOPSIS", ".SH DESCRIPTION", ".SH COMMANDS", ".SH EXAMPLES", ".SH SEE ALSO"}
	lastIdx := -1
	for _, section := range sections {
		idx := strings.Index(appContent, section)
		if idx == -1 {
			t.Errorf("app page missing section: %s", section)
			continue
		}
		if idx <= lastIdx {
			t.Errorf("section %s appears out of order", section)
		}
		lastIdx = idx
	}

	// Verify command page section ordering
	cmdPage, _ := os.ReadFile(filepath.Join(dir, "share", "man", "man1", "demo-greet.1"))
	cmdContent := string(cmdPage)

	cmdSections := []string{".SH NAME", ".SH SYNOPSIS", ".SH DESCRIPTION", ".SH OPTIONS", ".SH EXAMPLES", ".SH SEE ALSO"}
	lastIdx = -1
	for _, section := range cmdSections {
		idx := strings.Index(cmdContent, section)
		if idx == -1 {
			t.Errorf("command page missing section: %s", section)
			continue
		}
		if idx <= lastIdx {
			t.Errorf("section %s appears out of order in command page", section)
		}
		lastIdx = idx
	}
}

func TestGenerateManpageAppEnvVarsAndFiles(t *testing.T) {
	app := NewUtility("lux", "LSP multiplexer")
	app.Version = "0.1.0"
	app.EnvVars = []EnvVar{
		{
			Name:        "LUX_SOCKET",
			Description: "Path to the lux Unix domain socket. Overrides the default location.",
		},
		{
			Name:        "XDG_CONFIG_HOME",
			Description: "Base directory for lux configuration files.",
			Default:     "$HOME/.config",
		},
		{
			Name:        "EDITOR",
			Description: "Editor used by config-edit.",
			Default:     "vi",
		},
	}
	app.Files = []FilePath{
		{
			Path:        "$XDG_CONFIG_HOME/lux/config.toml",
			Description: "Per-user lux configuration file.",
		},
		{
			Path:        "$XDG_DATA_HOME/lux",
			Description: "Persistent state directory.",
		},
	}
	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show server status"},
	})

	dir := t.TempDir()
	if err := app.GenerateManpages(dir); err != nil {
		t.Fatalf("GenerateManpages: %v", err)
	}

	page, err := os.ReadFile(filepath.Join(dir, "share", "man", "man1", "lux.1"))
	if err != nil {
		t.Fatalf("read lux.1: %v", err)
	}
	content := string(page)

	if !strings.Contains(content, ".SH ENVIRONMENT") {
		t.Error("missing .SH ENVIRONMENT")
	}
	for _, name := range []string{"LUX_SOCKET", "XDG_CONFIG_HOME", "EDITOR"} {
		if !strings.Contains(content, ".B "+name) {
			t.Errorf("ENVIRONMENT missing .B %s", name)
		}
	}
	if !strings.Contains(content, "Default: $HOME/.config") {
		t.Error("ENVIRONMENT missing default for XDG_CONFIG_HOME")
	}
	if !strings.Contains(content, "Default: vi") {
		t.Error("ENVIRONMENT missing default for EDITOR")
	}

	if !strings.Contains(content, ".SH FILES") {
		t.Error("missing .SH FILES")
	}
	if !strings.Contains(content, ".I $XDG_CONFIG_HOME/lux/config.toml") {
		t.Error("FILES missing config.toml entry")
	}
	if !strings.Contains(content, ".I $XDG_DATA_HOME/lux") {
		t.Error("FILES missing data dir entry")
	}

	// ENVIRONMENT and FILES must appear after EXAMPLES (which is empty here,
	// so just check vs DESCRIPTION) and before SEE ALSO.
	envIdx := strings.Index(content, ".SH ENVIRONMENT")
	filesIdx := strings.Index(content, ".SH FILES")
	seeAlsoIdx := strings.Index(content, ".SH SEE ALSO")
	if !(envIdx < filesIdx && filesIdx < seeAlsoIdx) {
		t.Errorf("section order wrong: ENVIRONMENT=%d FILES=%d SEE_ALSO=%d", envIdx, filesIdx, seeAlsoIdx)
	}
}

func TestGenerateManpageCommandEnvVarsAndFiles(t *testing.T) {
	app := NewUtility("lux", "LSP multiplexer")
	app.AddCommand(&Command{
		Name:        "config-edit",
		Description: Description{Short: "Edit lux configuration"},
		EnvVars: []EnvVar{
			{Name: "EDITOR", Description: "Editor to launch.", Default: "vi"},
		},
		Files: []FilePath{
			{Path: "$XDG_CONFIG_HOME/lux/config.toml", Description: "File opened for editing."},
		},
	})

	dir := t.TempDir()
	if err := app.GenerateManpages(dir); err != nil {
		t.Fatalf("GenerateManpages: %v", err)
	}

	page, err := os.ReadFile(filepath.Join(dir, "share", "man", "man1", "lux-config-edit.1"))
	if err != nil {
		t.Fatalf("read lux-config-edit.1: %v", err)
	}
	content := string(page)

	if !strings.Contains(content, ".SH ENVIRONMENT") {
		t.Error("missing .SH ENVIRONMENT on command page")
	}
	if !strings.Contains(content, ".B EDITOR") {
		t.Error("ENVIRONMENT missing EDITOR on command page")
	}
	if !strings.Contains(content, ".SH FILES") {
		t.Error("missing .SH FILES on command page")
	}
}

func TestGenerateManpageNoEnvVarsOrFiles(t *testing.T) {
	app := NewUtility("plain", "no env/files")
	app.AddCommand(&Command{
		Name:        "run",
		Description: Description{Short: "Run it"},
	})

	dir := t.TempDir()
	if err := app.GenerateManpages(dir); err != nil {
		t.Fatalf("GenerateManpages: %v", err)
	}

	for _, name := range []string{"plain.1", "plain-run.1"} {
		page, err := os.ReadFile(filepath.Join(dir, "share", "man", "man1", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		content := string(page)
		if strings.Contains(content, ".SH ENVIRONMENT") {
			t.Errorf("%s: ENVIRONMENT should be absent when EnvVars is empty", name)
		}
		if strings.Contains(content, ".SH FILES") {
			t.Errorf("%s: FILES should be absent when Files is empty", name)
		}
	}
}

func TestGenerateManpageCommandSeeAlso(t *testing.T) {
	app := NewUtility("lux", "LSP multiplexer")
	app.AddCommand(&Command{
		Name:        "hover",
		Description: Description{Short: "Show hover info"},
		SeeAlso:     []string{"lux-definition", "lux-references"},
	})
	app.AddCommand(&Command{
		Name:        "definition",
		Description: Description{Short: "Go to definition"},
		SeeAlso:     []string{"lux-hover"},
	})

	dir := t.TempDir()
	if err := app.GenerateManpages(dir); err != nil {
		t.Fatalf("GenerateManpages: %v", err)
	}

	// hover page should list both cross-refs plus the parent back-ref
	hoverPage, err := os.ReadFile(filepath.Join(dir, "share", "man", "man1", "lux-hover.1"))
	if err != nil {
		t.Fatalf("read lux-hover.1: %v", err)
	}
	hoverContent := string(hoverPage)

	if !strings.Contains(hoverContent, ".BR lux-definition (1)") {
		t.Error("hover page missing cross-reference to lux-definition")
	}
	if !strings.Contains(hoverContent, ".BR lux-references (1)") {
		t.Error("hover page missing cross-reference to lux-references")
	}
	if !strings.Contains(hoverContent, ".BR lux (1)") {
		t.Error("hover page missing back-reference to parent app")
	}

	// definition page should reference hover plus parent
	defPage, err := os.ReadFile(filepath.Join(dir, "share", "man", "man1", "lux-definition.1"))
	if err != nil {
		t.Fatalf("read lux-definition.1: %v", err)
	}
	defContent := string(defPage)

	if !strings.Contains(defContent, ".BR lux-hover (1)") {
		t.Error("definition page missing cross-reference to lux-hover")
	}
	if !strings.Contains(defContent, ".BR lux (1)") {
		t.Error("definition page missing back-reference to parent app")
	}
}

func TestInstallExtraManpagesMapFS(t *testing.T) {
	app := NewUtility("moxy", "moxy proxy")
	mfs := fstest.MapFS{
		"cmd/moxy/moxyfile.5": &fstest.MapFile{
			Data: []byte(".Dd March 31, 2026\n.Dt MOXYFILE 5\n.Sh NAME\n"),
		},
		"cmd/maneater/maneater.1": &fstest.MapFile{
			Data: []byte(".Dd March 31, 2026\n.Dt MANEATER 1\n.Sh NAME\n"),
		},
	}
	app.ExtraManpages = []ManpageFile{
		{Source: mfs, Path: "cmd/moxy/moxyfile.5", Section: 5, Name: "moxyfile.5"},
		{Source: mfs, Path: "cmd/maneater/maneater.1", Section: 1, Name: "maneater.1"},
	}

	dir := t.TempDir()
	if err := app.GenerateAll(dir); err != nil {
		t.Fatalf("GenerateAll: %v", err)
	}

	for _, tc := range []struct {
		section int
		name    string
		want    string
	}{
		{5, "moxyfile.5", ".Dt MOXYFILE 5"},
		{1, "maneater.1", ".Dt MANEATER 1"},
	} {
		path := filepath.Join(dir, "share", "man", "man"+strconv.Itoa(tc.section), tc.name)
		got, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		if !strings.Contains(string(got), tc.want) {
			t.Errorf("%s: missing %q in copied bytes", path, tc.want)
		}
	}
}

func TestInstallExtraManpagesDirFS(t *testing.T) {
	// Stage source files on disk and read them via os.DirFS — exercises the
	// "path in source tree" code path that nix postInstall uses.
	srcDir := t.TempDir()
	manpageContent := ".TH FOO 1 \"\" \"foo 1.0\"\n.SH NAME\nfoo \\- demo\n"
	if err := os.MkdirAll(filepath.Join(srcDir, "doc"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "doc", "foo.1"), []byte(manpageContent), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	app := NewUtility("foo", "foo tool")
	app.ExtraManpages = []ManpageFile{
		{Source: os.DirFS(srcDir), Path: "doc/foo.1", Section: 1, Name: "foo.1"},
	}

	outDir := t.TempDir()
	if err := app.GenerateAll(outDir); err != nil {
		t.Fatalf("GenerateAll: %v", err)
	}

	// foo.1 will exist as both the GenerateManpages output and the ExtraManpages
	// copy at the same path. The ExtraManpages copy runs after GenerateManpages
	// so it overwrites — verify the final bytes are the hand-written content.
	got, err := os.ReadFile(filepath.Join(outDir, "share", "man", "man1", "foo.1"))
	if err != nil {
		t.Fatalf("read foo.1: %v", err)
	}
	if string(got) != manpageContent {
		t.Errorf("ExtraManpages did not overwrite generated page; got:\n%s", string(got))
	}
}

func TestInstallExtraManpagesValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		mf   ManpageFile
		want string
	}{
		{"nil source", ManpageFile{Path: "x", Section: 1, Name: "x.1"}, "Source is nil"},
		{"empty path", ManpageFile{Source: fstest.MapFS{}, Section: 1, Name: "x.1"}, "Path is empty"},
		{"zero section", ManpageFile{Source: fstest.MapFS{}, Path: "x", Name: "x.1"}, "Section must be > 0"},
		{"empty name", ManpageFile{Source: fstest.MapFS{}, Path: "x", Section: 1}, "Name is empty"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := NewUtility("t", "test")
			app.ExtraManpages = []ManpageFile{tc.mf}
			err := app.GenerateAll(t.TempDir())
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q did not contain %q", err.Error(), tc.want)
			}
		})
	}
}
