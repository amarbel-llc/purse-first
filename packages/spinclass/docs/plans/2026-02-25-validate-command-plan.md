# Validate Command Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `spinclass validate` command that validates the sweatfile hierarchy from PWD with TAP-14 output including subtests.

**Architecture:** New `internal/validate/` package with check functions returning `[]Issue`. TAP writer extended with `Subtest()` for indented sub-streams. Command wired in `main.go` like other simple commands.

**Tech Stack:** Go, Cobra, TOML (BurntSushi), TAP-14 (internal/tap)

---

### Task 1: Extend TAP writer with subtest and failure tracking

**Files:**
- Modify: `packages/spinclass/internal/tap/tap.go`
- Modify: `packages/spinclass/internal/tap/tap_test.go`

**Step 1: Write failing tests for Subtest and HasFailures**

Add to `packages/spinclass/internal/tap/tap_test.go`:

```go
func TestSubtestEmitsIndentedStream(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	sub := tw.Subtest("my subtest")
	sub.Ok("inner pass")
	sub.Plan()
	tw.EndSubtest("my subtest", sub)

	out := buf.String()
	if !strings.Contains(out, "    # Subtest: my subtest\n") {
		t.Errorf("expected subtest header, got: %q", out)
	}
	if !strings.Contains(out, "    1..1\n") {
		t.Errorf("expected indented plan, got: %q", out)
	}
	if !strings.Contains(out, "    ok 1 - inner pass\n") {
		t.Errorf("expected indented ok line, got: %q", out)
	}
	if !strings.Contains(out, "ok 1 - my subtest\n") {
		t.Errorf("expected parent ok line, got: %q", out)
	}
}

func TestSubtestWithFailureEmitsNotOkParent(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	sub := tw.Subtest("failing subtest")
	sub.Ok("pass")
	sub.NotOk("fail", map[string]string{"message": "bad"})
	sub.Plan()
	tw.EndSubtest("failing subtest", sub)

	out := buf.String()
	if !strings.Contains(out, "not ok 1 - failing subtest\n") {
		t.Errorf("expected parent not ok line, got: %q", out)
	}
}

func TestHasFailuresTracksNotOk(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.Ok("pass")
	if tw.HasFailures() {
		t.Error("expected no failures after Ok")
	}
	tw.NotOk("fail", nil)
	if !tw.HasFailures() {
		t.Error("expected failures after NotOk")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.spinclass/sweatfile --command go test ./packages/spinclass/internal/tap/ -v -run 'TestSubtest|TestHasFailures'`
Expected: FAIL — `Subtest`, `EndSubtest`, `HasFailures` don't exist

**Step 3: Implement Subtest, EndSubtest, HasFailures**

Modify `packages/spinclass/internal/tap/tap.go`:

Add `failed bool` field to `Writer` struct. Track failures in `NotOk`. Add:

```go
func (tw *Writer) HasFailures() bool {
	return tw.failed
}

// Subtest creates a child Writer that buffers its output. The child does NOT
// emit a TAP version header (subtests omit it per TAP-14 spec). Call
// EndSubtest on the parent when done.
func (tw *Writer) Subtest(name string) *Writer {
	return &Writer{w: &bytes.Buffer{}}
}

// EndSubtest writes the buffered subtest output (indented 4 spaces) under a
// "# Subtest:" comment, then emits the parent test point as ok/not ok based
// on whether the subtest had failures.
func (tw *Writer) EndSubtest(name string, sub *Writer) int {
	buf := sub.w.(*bytes.Buffer)

	fmt.Fprintf(tw.w, "    # Subtest: %s\n", name)
	for _, line := range strings.Split(buf.String(), "\n") {
		if line == "" {
			continue
		}
		fmt.Fprintf(tw.w, "    %s\n", line)
	}

	tw.n++
	if sub.HasFailures() {
		fmt.Fprintf(tw.w, "not ok %d - %s\n", tw.n, name)
		tw.failed = true
	} else {
		fmt.Fprintf(tw.w, "ok %d - %s\n", tw.n, name)
	}
	return tw.n
}
```

The `Subtest` writer must NOT emit `TAP version 14` — it's a raw writer. Add a constructor:

```go
// newRawWriter creates a Writer without emitting the TAP version header.
// Used for subtests.
func newRawWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}
```

Update `Subtest` to use `newRawWriter(&bytes.Buffer{})`.

Also update `NotOk` to set `tw.failed = true`.

**Step 4: Run tests to verify they pass**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.spinclass/sweatfile --command go test ./packages/spinclass/internal/tap/ -v`
Expected: All tests PASS

**Step 5: Commit**

```
feat(tap): add subtest and failure tracking support

Add Subtest/EndSubtest for TAP-14 nested test streams and HasFailures
for checking whether a writer recorded any failures.
```

---

### Task 2: Create validate package with Issue type and check functions

**Files:**
- Create: `packages/spinclass/internal/validate/validate.go`
- Create: `packages/spinclass/internal/validate/validate_test.go`

**Step 1: Write failing tests for check functions**

Create `packages/spinclass/internal/validate/validate_test.go`:

```go
package validate

import (
	"testing"

	"github.com/amarbel-llc/spinclass/internal/sweatfile"
)

func TestCheckClaudeAllowSyntaxValid(t *testing.T) {
	sf := sweatfile.Sweatfile{
		ClaudeAllow: []string{"Read", "Bash(git *)", "Write(/foo/*)"},
	}
	issues := CheckClaudeAllow(sf)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %v", issues)
	}
}

func TestCheckClaudeAllowSyntaxInvalid(t *testing.T) {
	sf := sweatfile.Sweatfile{
		ClaudeAllow: []string{"Bash(git *", "Read("},
	}
	issues := CheckClaudeAllow(sf)
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %v", issues)
	}
	for _, iss := range issues {
		if iss.Severity != SeverityError {
			t.Errorf("expected error severity, got %s", iss.Severity)
		}
	}
}

func TestCheckClaudeAllowUnknownTool(t *testing.T) {
	sf := sweatfile.Sweatfile{
		ClaudeAllow: []string{"FooBar", "Read"},
	}
	issues := CheckClaudeAllow(sf)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %v", issues)
	}
	if issues[0].Severity != SeverityWarning {
		t.Errorf("expected warning severity, got %s", issues[0].Severity)
	}
}

func TestCheckGitExcludesValid(t *testing.T) {
	sf := sweatfile.Sweatfile{
		GitExcludes: []string{".claude/", ".direnv/"},
	}
	issues := CheckGitExcludes(sf)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %v", issues)
	}
}

func TestCheckGitExcludesEmpty(t *testing.T) {
	sf := sweatfile.Sweatfile{
		GitExcludes: []string{".claude/", "", ".direnv/"},
	}
	issues := CheckGitExcludes(sf)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %v", issues)
	}
}

func TestCheckGitExcludesAbsolutePath(t *testing.T) {
	sf := sweatfile.Sweatfile{
		GitExcludes: []string{"/absolute/path"},
	}
	issues := CheckGitExcludes(sf)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %v", issues)
	}
}

func TestCheckMergedDuplicates(t *testing.T) {
	sf := sweatfile.Sweatfile{
		GitExcludes: []string{".claude/", ".direnv/", ".claude/"},
		ClaudeAllow: []string{"Read", "Read"},
	}
	issues := CheckMerged(sf)
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues (one per field), got %v", issues)
	}
	for _, iss := range issues {
		if iss.Severity != SeverityWarning {
			t.Errorf("expected warning severity, got %s", iss.Severity)
		}
	}
}

func TestCheckUnknownFields(t *testing.T) {
	data := []byte(`
git_excludes = [".claude/"]
unknown_field = "bad"
`)
	issues := CheckUnknownFields(data)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %v", issues)
	}
	if issues[0].Severity != SeverityError {
		t.Errorf("expected error severity, got %s", issues[0].Severity)
	}
}

func TestCheckUnknownFieldsClean(t *testing.T) {
	data := []byte(`
git_excludes = [".claude/"]
claude_allow = ["Read"]
`)
	issues := CheckUnknownFields(data)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %v", issues)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.spinclass/sweatfile --command go test ./packages/spinclass/internal/validate/ -v`
Expected: FAIL — package doesn't exist

**Step 3: Implement validate package**

Create `packages/spinclass/internal/validate/validate.go`:

```go
package validate

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/amarbel-llc/spinclass/internal/sweatfile"
)

const (
	SeverityError   = "error"
	SeverityWarning = "warning"
)

type Issue struct {
	Message  string
	Severity string
	Field    string
	Value    string
}

var KnownTools = []string{
	"Bash", "Read", "Write", "Edit", "Glob", "Grep",
	"WebFetch", "WebSearch", "NotebookEdit", "Task", "Skill", "LSP",
}

func isKnownTool(name string) bool {
	for _, t := range KnownTools {
		if t == name {
			return true
		}
	}
	return false
}

// parseRuleSyntax checks if a rule has valid ToolName or ToolName(pattern) syntax.
// Returns the tool name and any syntax error.
func parseRuleSyntax(rule string) (string, error) {
	if rule == "" {
		return "", fmt.Errorf("empty rule")
	}

	parenIdx := strings.Index(rule, "(")
	if parenIdx < 0 {
		// Plain tool name like "Read"
		return rule, nil
	}

	if !strings.HasSuffix(rule, ")") {
		return "", fmt.Errorf("unmatched parenthesis in rule %q", rule)
	}

	toolName := rule[:parenIdx]
	if toolName == "" {
		return "", fmt.Errorf("empty tool name in rule %q", rule)
	}

	return toolName, nil
}

func CheckClaudeAllow(sf sweatfile.Sweatfile) []Issue {
	var issues []Issue
	for _, rule := range sf.ClaudeAllow {
		toolName, err := parseRuleSyntax(rule)
		if err != nil {
			issues = append(issues, Issue{
				Message:  err.Error(),
				Severity: SeverityError,
				Field:    "claude_allow",
				Value:    rule,
			})
			continue
		}
		if !isKnownTool(toolName) {
			issues = append(issues, Issue{
				Message:  fmt.Sprintf("unknown tool name %q", toolName),
				Severity: SeverityWarning,
				Field:    "claude_allow",
				Value:    rule,
			})
		}
	}
	return issues
}

func CheckGitExcludes(sf sweatfile.Sweatfile) []Issue {
	var issues []Issue
	for _, exc := range sf.GitExcludes {
		if exc == "" {
			issues = append(issues, Issue{
				Message:  "empty exclude pattern",
				Severity: SeverityError,
				Field:    "git_excludes",
			})
		} else if filepath.IsAbs(exc) {
			issues = append(issues, Issue{
				Message:  fmt.Sprintf("absolute path %q in git_excludes", exc),
				Severity: SeverityError,
				Field:    "git_excludes",
				Value:    exc,
			})
		}
	}
	return issues
}

func CheckMerged(sf sweatfile.Sweatfile) []Issue {
	var issues []Issue

	if dups := findDuplicates(sf.GitExcludes); len(dups) > 0 {
		issues = append(issues, Issue{
			Message:  fmt.Sprintf("duplicate git_excludes: %s", strings.Join(dups, ", ")),
			Severity: SeverityWarning,
			Field:    "git_excludes",
		})
	}

	if dups := findDuplicates(sf.ClaudeAllow); len(dups) > 0 {
		issues = append(issues, Issue{
			Message:  fmt.Sprintf("duplicate claude_allow: %s", strings.Join(dups, ", ")),
			Severity: SeverityWarning,
			Field:    "claude_allow",
		})
	}

	return issues
}

func findDuplicates(items []string) []string {
	seen := make(map[string]bool)
	var dups []string
	for _, item := range items {
		if seen[item] {
			dups = append(dups, item)
		}
		seen[item] = true
	}
	return dups
}

func CheckUnknownFields(data []byte) []Issue {
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil // parse errors handled elsewhere
	}

	known := map[string]bool{
		"git_excludes": true,
		"claude_allow": true,
	}

	var issues []Issue
	for key := range raw {
		if !known[key] {
			issues = append(issues, Issue{
				Message:  fmt.Sprintf("unknown field %q", key),
				Severity: SeverityError,
				Field:    key,
			})
		}
	}
	return issues
}
```

**Step 4: Run tests to verify they pass**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.spinclass/sweatfile --command go test ./packages/spinclass/internal/validate/ -v`
Expected: All tests PASS

**Step 5: Commit**

```
feat(validate): add check functions for sweatfile validation

Issue type with severity levels, checks for claude_allow syntax and
known tools, git_excludes validation, merged duplicate detection, and
unknown field detection.
```

---

### Task 3: Implement Run function with TAP-14 output

**Files:**
- Modify: `packages/spinclass/internal/validate/validate.go`
- Create: `packages/spinclass/internal/validate/run_test.go`

**Step 1: Write failing tests for Run**

Create `packages/spinclass/internal/validate/run_test.go`:

```go
package validate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSweatfile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creating directory for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func TestRunValidHierarchy(t *testing.T) {
	home := t.TempDir()
	repoDir := filepath.Join(home, "eng", "myrepo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	globalPath := filepath.Join(home, ".config", "spinclass", "sweatfile")
	writeSweatfile(t, globalPath, `claude_allow = ["Read", "Bash(git *)"]`)

	repoSweatfile := filepath.Join(repoDir, "sweatfile")
	writeSweatfile(t, repoSweatfile, `git_excludes = [".direnv/"]`)

	var buf bytes.Buffer
	exitCode := Run(&buf, home, repoDir)

	out := buf.String()
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d\noutput:\n%s", exitCode, out)
	}
	if !strings.HasPrefix(out, "TAP version 14\n") {
		t.Errorf("expected TAP version header, got: %q", out)
	}
	if !strings.Contains(out, "# Subtest:") {
		t.Errorf("expected subtests, got: %q", out)
	}
}

func TestRunInvalidSyntax(t *testing.T) {
	home := t.TempDir()
	repoDir := filepath.Join(home, "eng", "myrepo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	repoSweatfile := filepath.Join(repoDir, "sweatfile")
	writeSweatfile(t, repoSweatfile, `claude_allow = ["Bash(git *"]`)

	var buf bytes.Buffer
	exitCode := Run(&buf, home, repoDir)

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(buf.String(), "not ok") {
		t.Errorf("expected not ok in output, got: %q", buf.String())
	}
}

func TestRunNoSweatfiles(t *testing.T) {
	home := t.TempDir()
	repoDir := filepath.Join(home, "eng", "myrepo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	exitCode := Run(&buf, home, repoDir)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(buf.String(), "# SKIP") {
		t.Errorf("expected SKIP directives, got: %q", buf.String())
	}
}

func TestRunInvalidTOML(t *testing.T) {
	home := t.TempDir()
	repoDir := filepath.Join(home, "eng", "myrepo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	repoSweatfile := filepath.Join(repoDir, "sweatfile")
	writeSweatfile(t, repoSweatfile, `this is not valid toml [[[`)

	var buf bytes.Buffer
	exitCode := Run(&buf, home, repoDir)

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(buf.String(), "not ok") {
		t.Errorf("expected not ok in output, got: %q", buf.String())
	}
}

func TestRunDuplicatesInMerged(t *testing.T) {
	home := t.TempDir()
	repoDir := filepath.Join(home, "eng", "myrepo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	globalPath := filepath.Join(home, ".config", "spinclass", "sweatfile")
	writeSweatfile(t, globalPath, `claude_allow = ["Read"]`)

	repoSweatfile := filepath.Join(repoDir, "sweatfile")
	writeSweatfile(t, repoSweatfile, `claude_allow = ["Read"]`)

	var buf bytes.Buffer
	exitCode := Run(&buf, home, repoDir)

	// Duplicates are warnings, not errors — exit code 0
	if exitCode != 0 {
		t.Errorf("expected exit code 0 (warnings only), got %d\noutput:\n%s", exitCode, buf.String())
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.spinclass/sweatfile --command go test ./packages/spinclass/internal/validate/ -v -run 'TestRun'`
Expected: FAIL — `Run` doesn't exist

**Step 3: Implement Run function**

Add to `packages/spinclass/internal/validate/validate.go`:

```go
func Run(w io.Writer, home, repoDir string) int {
	tw := tap.NewWriter(w)

	// Load hierarchy to discover all sweatfile locations
	result, err := sweatfile.LoadHierarchy(home, repoDir)
	if err != nil {
		tw.NotOk("load hierarchy", map[string]string{
			"severity": SeverityError,
			"message":  err.Error(),
		})
		tw.Plan()
		return 1
	}

	// Validate each file in the hierarchy
	for _, src := range result.Sources {
		if !src.Found {
			tw.Skip(src.Path, "not found")
			continue
		}

		sub := tw.Subtest(src.Path)

		// Check 1: valid TOML (re-read raw data for unknown field check)
		data, readErr := os.ReadFile(src.Path)
		if readErr != nil {
			sub.NotOk("readable", map[string]string{
				"severity": SeverityError,
				"message":  readErr.Error(),
			})
			sub.Plan()
			tw.EndSubtest(src.Path, sub)
			continue
		}

		_, parseErr := sweatfile.Parse(data)
		if parseErr != nil {
			sub.NotOk("valid TOML", map[string]string{
				"severity": SeverityError,
				"message":  parseErr.Error(),
			})
			sub.Plan()
			tw.EndSubtest(src.Path, sub)
			continue
		}
		sub.Ok("valid TOML")

		// Check 2: unknown fields
		if issues := CheckUnknownFields(data); len(issues) > 0 {
			for _, iss := range issues {
				sub.NotOk("no unknown fields", map[string]string{
					"severity": iss.Severity,
					"message":  iss.Message,
					"field":    iss.Field,
				})
			}
		} else {
			sub.Ok("no unknown fields")
		}

		// Check 3: claude_allow
		if len(src.File.ClaudeAllow) > 0 {
			if issues := CheckClaudeAllow(src.File); len(issues) > 0 {
				for _, iss := range issues {
					diag := map[string]string{
						"severity": iss.Severity,
						"message":  iss.Message,
					}
					if iss.Value != "" {
						diag["rule"] = iss.Value
					}
					sub.NotOk("claude_allow valid", diag)
				}
			} else {
				sub.Ok("claude_allow valid")
			}
		}

		// Check 4: git_excludes
		if len(src.File.GitExcludes) > 0 {
			if issues := CheckGitExcludes(src.File); len(issues) > 0 {
				for _, iss := range issues {
					diag := map[string]string{
						"severity": iss.Severity,
						"message":  iss.Message,
					}
					if iss.Value != "" {
						diag["value"] = iss.Value
					}
					sub.NotOk("git_excludes valid", diag)
				}
			} else {
				sub.Ok("git_excludes valid")
			}
		}

		sub.Plan()
		tw.EndSubtest(src.Path, sub)
	}

	// Merged result checks
	sub := tw.Subtest("merged result")
	if issues := CheckMerged(result.Merged); len(issues) > 0 {
		for _, iss := range issues {
			sub.NotOk(iss.Field+" unique", map[string]string{
				"severity": iss.Severity,
				"message":  iss.Message,
			})
		}
	} else {
		sub.Ok("no duplicate entries")
	}
	sub.Plan()
	tw.EndSubtest("merged result", sub)

	// Apply dry-run checks
	applySub := tw.Subtest("apply (dry-run)")
	allExcludes := append(result.Merged.GitExcludes, sweatfile.HardcodedExcludes...)
	if issues := CheckGitExcludes(sweatfile.Sweatfile{GitExcludes: allExcludes}); len(issues) > 0 {
		for _, iss := range issues {
			applySub.NotOk("git excludes structure valid", map[string]string{
				"severity": iss.Severity,
				"message":  iss.Message,
			})
		}
	} else {
		applySub.Ok("git excludes structure valid")
	}

	if issues := CheckClaudeAllow(result.Merged); len(issues) > 0 {
		hasErrors := false
		for _, iss := range issues {
			if iss.Severity == SeverityError {
				hasErrors = true
				applySub.NotOk("claude settings structure valid", map[string]string{
					"severity": iss.Severity,
					"message":  iss.Message,
				})
			}
		}
		if !hasErrors {
			applySub.Ok("claude settings structure valid")
		}
	} else {
		applySub.Ok("claude settings structure valid")
	}
	applySub.Plan()
	tw.EndSubtest("apply (dry-run)", applySub)

	tw.Plan()

	if tw.HasFailures() {
		return 1
	}
	return 0
}
```

Add `"io"` and `"os"` to imports. Add import for `"github.com/amarbel-llc/spinclass/internal/tap"`.

**Step 4: Run tests to verify they pass**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.spinclass/sweatfile --command go test ./packages/spinclass/internal/validate/ -v`
Expected: All tests PASS

**Step 5: Commit**

```
feat(validate): implement Run with TAP-14 subtest output

Orchestrates hierarchy loading, per-file checks, merged result checks,
and dry-run apply checks with TAP-14 output including subtests.
```

---

### Task 4: Wire validate command in main.go

**Files:**
- Modify: `packages/spinclass/cmd/spinclass/main.go`

**Step 1: Add the validate command**

Add import: `"github.com/amarbel-llc/spinclass/internal/validate"`

Add command:

```go
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the sweatfile hierarchy",
	Long:  `Walk the sweatfile hierarchy from PWD and validate each file for structural and semantic correctness. Outputs TAP-14 with subtests.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		exitCode := validate.Run(os.Stdout, home, cwd)
		if exitCode != 0 {
			os.Exit(exitCode)
		}
		return nil
	},
}
```

Add to `init()`: `rootCmd.AddCommand(validateCmd)`

**Step 2: Build and smoke test**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.spinclass/sweatfile --command go build -o build/spinclass ./packages/spinclass/cmd/spinclass/`
Expected: Build succeeds

Run: `./build/spinclass validate`
Expected: TAP-14 output showing hierarchy validation results

**Step 3: Run all tests**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.spinclass/sweatfile --command go test ./packages/spinclass/... -v`
Expected: All tests PASS

**Step 4: Commit**

```
feat(spinclass): add validate command for sweatfile hierarchy

Wires the validate package as `spinclass validate`. Walks the
hierarchy from PWD, validates each file, and outputs TAP-14 with
subtests.
```

---

### Task 5: Manual integration test

**Step 1: Run against real hierarchy**

Run: `./build/spinclass validate`
Expected: TAP-14 output matching the design doc format

**Step 2: Verify with tap-dancer validate**

Run: `./build/spinclass validate | nix develop /Users/sfriedenberg/eng/repos/purse-first/.spinclass/sweatfile --command tap-dancer validate`
Expected: Valid TAP-14
