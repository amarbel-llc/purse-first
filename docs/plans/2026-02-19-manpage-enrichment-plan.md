# Manpage Enrichment Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enrich generated manpages with SYNOPSIS, EXAMPLES, and SEE ALSO sections, and add an `Example` struct to the command framework so authors can declare usage examples that flow into manpages and downstream skill generation.

**Architecture:** Add `Example` type and `Examples` fields to the existing `command` package. Modify `writeAppManpage` and `writeCommandManpage` to render the new sections. A helper `writeExamples` renders `[]Example` into roff `.nf`/`.fi` blocks. A helper `writeSeeAlso` renders cross-references. Template variable convention is defined but not resolved in this change.

**Tech Stack:** Go, roff/troff manpage format, existing `command` package in `libs/go-mcp/command/`

---

### Task 1: Add Example type and Examples fields

**Files:**
- Modify: `libs/go-mcp/command/command.go:64-81` (Command struct)
- Modify: `libs/go-mcp/command/app.go:6-15` (App struct)

**Step 1: Write the failing test**

Create a test that constructs a Command with Examples and verifies the field is accessible.

Add to `libs/go-mcp/command/generate_manpages_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `just test`

Expected: compilation error — `Example` type undefined, `Examples` field unknown.

**Step 3: Write minimal implementation**

In `libs/go-mcp/command/command.go`, add the `Example` type before the `Command` struct (after the `Param` struct, around line 59):

```go
// Example represents a single usage example for a command or app.
type Example struct {
	Description string // what this example demonstrates
	Command     string // shell invocation (may be multi-line)
	Output      string // optional expected output snippet
}
```

Add `Examples` field to the `Command` struct (after `MapsTools`):

```go
	Examples  []Example
```

In `libs/go-mcp/command/app.go`, add `Examples` field to the `App` struct (after `Params`):

```go
	Examples []Example // app-level workflow examples
```

**Step 4: Run test to verify it passes**

Run: `just test`

Expected: all tests pass including `TestCommandExamplesField`.

**Step 5: Commit**

```bash
git add libs/go-mcp/command/command.go libs/go-mcp/command/app.go libs/go-mcp/command/generate_manpages_test.go
git commit -m "feat(command): add Example type and Examples fields"
```

---

### Task 2: Add EXAMPLES section to per-command manpages

**Files:**
- Modify: `libs/go-mcp/command/generate_manpages.go:75-126` (writeCommandManpage)
- Modify: `libs/go-mcp/command/generate_manpages_test.go` (TestGenerateManpageCommand)

**Step 1: Write the failing test**

Update `TestGenerateManpageCommand` in `libs/go-mcp/command/generate_manpages_test.go` to add examples to the command and assert they appear in the output.

Add examples to the existing command setup (after Params):

```go
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
```

Add assertions after the existing ones:

```go
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
```

Also add a test that commands without examples produce no EXAMPLES section:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `just test`

Expected: FAIL — "missing EXAMPLES section" (the generator doesn't render examples yet).

**Step 3: Write minimal implementation**

In `libs/go-mcp/command/generate_manpages.go`, add a `writeExamples` helper and call it from `writeCommandManpage`.

Add helper function (after `writeCommandManpage`):

```go
func writeExamples(b *strings.Builder, examples []Example) {
	if len(examples) == 0 {
		return
	}
	fmt.Fprintf(b, ".SH EXAMPLES\n")
	for _, ex := range examples {
		fmt.Fprintf(b, ".TP\n")
		fmt.Fprintf(b, "%s\n", ex.Description)
		fmt.Fprintf(b, ".nf\n")
		for _, line := range strings.Split(ex.Command, "\n") {
			fmt.Fprintf(b, "$ %s\n", line)
		}
		if ex.Output != "" {
			fmt.Fprintf(b, "%s\n", ex.Output)
		}
		fmt.Fprintf(b, ".fi\n")
	}
}
```

In `writeCommandManpage`, insert a call to `writeExamples` after the ALIASES section (before the final `os.WriteFile`):

```go
	writeExamples(&b, cmd.Examples)
```

**Step 4: Run test to verify it passes**

Run: `just test`

Expected: all tests pass.

**Step 5: Commit**

```bash
git add libs/go-mcp/command/generate_manpages.go libs/go-mcp/command/generate_manpages_test.go
git commit -m "feat(manpage): add EXAMPLES section to per-command manpages"
```

---

### Task 3: Add SEE ALSO to per-command manpages

**Files:**
- Modify: `libs/go-mcp/command/generate_manpages.go` (writeCommandManpage)
- Modify: `libs/go-mcp/command/generate_manpages_test.go`

**Step 1: Write the failing test**

Add assertion to `TestGenerateManpageCommand`:

```go
	if !strings.Contains(content, ".SH SEE ALSO") {
		t.Error("missing SEE ALSO section")
	}
	if !strings.Contains(content, "grit(1)") {
		t.Error("missing back-reference to main app page")
	}
```

**Step 2: Run test to verify it fails**

Run: `just test`

Expected: FAIL — "missing SEE ALSO section".

**Step 3: Write minimal implementation**

In `writeCommandManpage`, after the `writeExamples` call, add:

```go
	fmt.Fprintf(&b, ".SH SEE ALSO\n")
	fmt.Fprintf(&b, ".BR %s (1)\n", a.Name)
```

**Step 4: Run test to verify it passes**

Run: `just test`

Expected: all tests pass.

**Step 5: Commit**

```bash
git add libs/go-mcp/command/generate_manpages.go libs/go-mcp/command/generate_manpages_test.go
git commit -m "feat(manpage): add SEE ALSO to per-command manpages"
```

---

### Task 4: Add SYNOPSIS to app manpage

**Files:**
- Modify: `libs/go-mcp/command/generate_manpages.go:36-73` (writeAppManpage)
- Modify: `libs/go-mcp/command/generate_manpages_test.go` (TestGenerateManpageApp)

**Step 1: Write the failing test**

Add assertions to `TestGenerateManpageApp`:

```go
	if !strings.Contains(content, ".SH SYNOPSIS") {
		t.Error("missing SYNOPSIS section")
	}
	if !strings.Contains(content, ".I command") {
		t.Error("missing command placeholder in SYNOPSIS")
	}
```

**Step 2: Run test to verify it fails**

Run: `just test`

Expected: FAIL — "missing SYNOPSIS section".

**Step 3: Write minimal implementation**

In `writeAppManpage`, insert SYNOPSIS after NAME (before DESCRIPTION):

```go
	// SYNOPSIS
	fmt.Fprintf(&b, ".SH SYNOPSIS\n")
	fmt.Fprintf(&b, ".B %s\n", a.Name)
	fmt.Fprintf(&b, ".I command\n")
	fmt.Fprintf(&b, ".RI [ options ]\n")
```

**Step 4: Run test to verify it passes**

Run: `just test`

Expected: all tests pass.

**Step 5: Commit**

```bash
git add libs/go-mcp/command/generate_manpages.go libs/go-mcp/command/generate_manpages_test.go
git commit -m "feat(manpage): add SYNOPSIS to app manpage"
```

---

### Task 5: Add EXAMPLES section to app manpage

**Files:**
- Modify: `libs/go-mcp/command/generate_manpages.go` (writeAppManpage)
- Modify: `libs/go-mcp/command/generate_manpages_test.go` (TestGenerateManpageApp)

**Step 1: Write the failing test**

Add app-level examples and assertions to `TestGenerateManpageApp`:

After `app.Description.Long = ...`:

```go
	app.Examples = []Example{
		{
			Description: "Stage and commit changes",
			Command:     "grit add --repo_path=. --paths='[\"main.go\"]'\ngrit commit --repo_path=. --message='initial'",
		},
	}
```

Add assertions:

```go
	if !strings.Contains(content, ".SH EXAMPLES") {
		t.Error("missing EXAMPLES section")
	}
	if !strings.Contains(content, "Stage and commit changes") {
		t.Error("missing app example description")
	}
	if !strings.Contains(content, "grit add") {
		t.Error("missing app example command")
	}
```

Also add a test for app without examples:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `just test`

Expected: FAIL — "missing EXAMPLES section".

**Step 3: Write minimal implementation**

In `writeAppManpage`, after the COMMANDS section, reuse the `writeExamples` helper:

```go
	writeExamples(&b, a.Examples)
```

**Step 4: Run test to verify it passes**

Run: `just test`

Expected: all tests pass.

**Step 5: Commit**

```bash
git add libs/go-mcp/command/generate_manpages.go libs/go-mcp/command/generate_manpages_test.go
git commit -m "feat(manpage): add EXAMPLES section to app manpage"
```

---

### Task 6: Add SEE ALSO to app manpage

**Files:**
- Modify: `libs/go-mcp/command/generate_manpages.go` (writeAppManpage)
- Modify: `libs/go-mcp/command/generate_manpages_test.go` (TestGenerateManpageApp)

**Step 1: Write the failing test**

Add assertions to `TestGenerateManpageApp`:

```go
	if !strings.Contains(content, ".SH SEE ALSO") {
		t.Error("missing SEE ALSO section")
	}
	if !strings.Contains(content, "grit-status(1)") {
		t.Error("missing cross-reference to subcommand page")
	}
```

**Step 2: Run test to verify it fails**

Run: `just test`

Expected: FAIL — "missing SEE ALSO section".

**Step 3: Write minimal implementation**

In `writeAppManpage`, after `writeExamples`, add SEE ALSO using the sorted `cmds` slice already computed:

```go
	if len(cmds) > 0 {
		fmt.Fprintf(&b, ".SH SEE ALSO\n")
		var refs []string
		for _, nc := range cmds {
			refs = append(refs, fmt.Sprintf(".BR %s-%s (1)", a.Name, nc.name))
		}
		fmt.Fprintf(&b, "%s\n", strings.Join(refs, ",\n"))
	}
```

**Step 4: Run test to verify it passes**

Run: `just test`

Expected: all tests pass.

**Step 5: Commit**

```bash
git add libs/go-mcp/command/generate_manpages.go libs/go-mcp/command/generate_manpages_test.go
git commit -m "feat(manpage): add SEE ALSO to app manpage"
```

---

### Task 7: Update COMMANDS section to cross-reference subcommand pages

**Files:**
- Modify: `libs/go-mcp/command/generate_manpages.go` (writeAppManpage)
- Modify: `libs/go-mcp/command/generate_manpages_test.go` (TestGenerateManpageApp)

**Step 1: Write the failing test**

Add assertion to `TestGenerateManpageApp` that COMMANDS entries use `.BR` with `(1)`:

```go
	if !strings.Contains(content, ".BR status (1)") {
		t.Error("COMMANDS should cross-reference subcommand manpage with (1)")
	}
```

**Step 2: Run test to verify it fails**

Run: `just test`

Expected: FAIL — current format uses `.B status` without `(1)`.

**Step 3: Write minimal implementation**

In `writeAppManpage`, change the COMMANDS rendering from:

```go
			fmt.Fprintf(&b, ".B %s\n", nc.name)
```

to:

```go
			fmt.Fprintf(&b, ".BR %s (1)\n", nc.name)
```

**Step 4: Run test to verify it passes**

Run: `just test`

Expected: all tests pass.

**Step 5: Commit**

```bash
git add libs/go-mcp/command/generate_manpages.go libs/go-mcp/command/generate_manpages_test.go
git commit -m "feat(manpage): cross-reference subcommand pages in COMMANDS section"
```

---

### Task 8: Verify full manpage output with mandoc

**Files:**
- No code changes — validation only.

**Step 1: Write a quick integration check**

Add a test that generates a complete app with examples and renders the output to verify no roff syntax errors. This test uses the full builder path and validates structural ordering.

Add to `libs/go-mcp/command/generate_manpages_test.go`:

```go
func TestManpageSectionOrdering(t *testing.T) {
	app := NewApp("demo", "Demo tool")
	app.Version = "1.0.0"
	app.Description.Long = "A demonstration tool."
	app.Examples = []Example{
		{Description: "Run a workflow", Command: "demo greet --name=world"},
	}

	app.AddCommand(&Command{
		Name:        "greet",
		Description: Description{Short: "Say hello", Long: "Greet someone by name."},
		Params:      []Param{{Name: "name", Type: String, Description: "Who to greet", Required: true}},
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
```

**Step 2: Run all tests**

Run: `just test`

Expected: all tests pass, confirming section ordering is correct.

**Step 3: Commit**

```bash
git add libs/go-mcp/command/generate_manpages_test.go
git commit -m "test(manpage): add section ordering validation test"
```

---

### Task 9: Update skill documentation and API reference

**Files:**
- Modify: `skills/go-cli-framework/references/api-reference.md`
- Modify: `skills/go-cli-framework/examples/command-app.go`

**Step 1: Update API reference**

Add `Example` type documentation to the API reference after the `Param` type section. Include the struct definition and field descriptions.

Add `Examples` field to both the `Command` and `App` struct documentation sections.

**Step 2: Update example app**

Add `Examples` to the example commands in `skills/go-cli-framework/examples/command-app.go` to show authors how to declare examples.

Add to the `stat` command:

```go
		Examples: []command.Example{
			{
				Description: "Get metadata for a Go source file",
				Command:     "fileinfo stat --path=main.go",
				Output:      `{"name": "main.go", "size": 1234, "mode": "-rw-r--r--"}`,
			},
		},
```

Add app-level examples:

```go
	app.Examples = []command.Example{
		{
			Description: "Inspect a file and list its directory",
			Command:     "fileinfo stat --path=src/main.go\nfileinfo ls --path=src/",
		},
	}
```

**Step 3: Run tests to make sure nothing broke**

Run: `just test`

Expected: all tests pass.

**Step 4: Commit**

```bash
git add skills/go-cli-framework/references/api-reference.md skills/go-cli-framework/examples/command-app.go
git commit -m "docs(skills): document Example type and update example app"
```
