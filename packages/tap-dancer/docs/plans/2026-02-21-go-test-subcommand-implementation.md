# `tap-dancer go-test` Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `tap-dancer go-test` subcommand that runs `go test -json` and converts the output to TAP-14, including nested subtest support.

**Architecture:** Two components: (1) extend `tap.Writer` with a `Subtest()` method that returns a child writer emitting indented TAP-14, (2) add a `go-test` subcommand that spawns `go test -json`, buffers events per package, and emits TAP-14 using the writer. The subcommand uses `RunCLI` (CLI-only) since it needs to forward arbitrary args to `go test` and doesn't make sense as an MCP tool.

**Tech Stack:** Go 1.23+, `tap` package (tap-dancer/go), `command.App` from purse-first/go-mcp

---

### Task 1: Writer `Subtest()` — Indented Writer Wrapper

**Files:**
- Modify: `go/tap.go`
- Test: `go/tap_test.go`

**Step 1: Write the failing test for basic subtest output**

Add to `go/tap_test.go`:

```go
func TestSubtestEmitsIndentedBlock(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	sub := tw.Subtest("nested")
	sub.Ok("inner pass")
	sub.Plan()
	tw.Ok("nested")

	expected := "TAP version 14\n" +
		"    # Subtest: nested\n" +
		"    ok 1 - inner pass\n" +
		"    1..1\n" +
		"ok 1 - nested\n"

	if buf.String() != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, buf.String())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd ~/eng/repos/tap-dancer && nix develop --command bash -c "cd go && go test -run TestSubtestEmitsIndentedBlock -v ./..."`
Expected: Compilation error — `Subtest` method does not exist.

**Step 3: Implement `Subtest()` on `Writer`**

Add to `go/tap.go`:

```go
type indentWriter struct {
	w      io.Writer
	prefix string
}

func (iw *indentWriter) Write(p []byte) (int, error) {
	lines := strings.Split(string(p), "\n")
	total := 0
	for i, line := range lines {
		if i == len(lines)-1 && line == "" {
			// Trailing newline from fmt.Fprintln — write just the newline
			n, err := iw.w.Write([]byte("\n"))
			total += n
			if err != nil {
				return total, err
			}
			continue
		}
		out := iw.prefix + line
		if i < len(lines)-1 {
			out += "\n"
		}
		n, err := iw.w.Write([]byte(out))
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func (tw *Writer) Subtest(name string) *Writer {
	prefix := strings.Repeat("    ", tw.depth+1)
	fmt.Fprintf(tw.w, "%s# Subtest: %s\n", prefix, name)
	iw := &indentWriter{w: tw.w, prefix: prefix}
	return &Writer{w: iw, depth: tw.depth + 1}
}
```

Also modify the `Writer` struct to track depth:

```go
type Writer struct {
	w     io.Writer
	n     int
	depth int
}
```

`NewWriter` stays the same — `depth` zero-value is correct for the root.

**Step 4: Run test to verify it passes**

Run: `cd ~/eng/repos/tap-dancer && nix develop --command bash -c "cd go && go test -run TestSubtestEmitsIndentedBlock -v ./..."`
Expected: PASS

**Step 5: Commit**

```
feat(writer): add Subtest method for nested TAP-14 output
```

---

### Task 2: Writer Subtest — Additional Test Cases

**Files:**
- Test: `go/tap_test.go`

**Step 1: Write tests for nested subtests and diagnostics within subtests**

Add to `go/tap_test.go`:

```go
func TestNestedSubtestTwoLevelsDeep(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	outer := tw.Subtest("outer")
	inner := outer.Subtest("inner")
	inner.Ok("deep test")
	inner.Plan()
	outer.Ok("inner")
	outer.Plan()
	tw.Ok("outer")

	expected := "TAP version 14\n" +
		"    # Subtest: outer\n" +
		"        # Subtest: inner\n" +
		"        ok 1 - deep test\n" +
		"        1..1\n" +
		"    ok 1 - inner\n" +
		"    1..1\n" +
		"ok 1 - outer\n"

	if buf.String() != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, buf.String())
	}
}

func TestSubtestNotOkWithDiagnostics(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	sub := tw.Subtest("pkg")
	sub.NotOk("failing", map[string]string{
		"message": "broke",
	})
	sub.Plan()
	tw.NotOk("pkg", nil)

	out := buf.String()
	if !strings.Contains(out, "    not ok 1 - failing\n") {
		t.Errorf("expected indented not ok, got:\n%s", out)
	}
	if !strings.Contains(out, "      ---\n") {
		t.Errorf("expected indented YAML start, got:\n%s", out)
	}
	if !strings.Contains(out, "      message: broke\n") {
		t.Errorf("expected indented diagnostic, got:\n%s", out)
	}
	if !strings.Contains(out, "      ...\n") {
		t.Errorf("expected indented YAML end, got:\n%s", out)
	}
}

func TestSubtestBailOut(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	sub := tw.Subtest("broken-pkg")
	sub.BailOut("build failed")
	tw.NotOk("broken-pkg", nil)

	out := buf.String()
	if !strings.Contains(out, "    Bail out! build failed\n") {
		t.Errorf("expected indented bail out, got:\n%s", out)
	}
}

func TestSubtestHasIndependentCounter(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	sub1 := tw.Subtest("first")
	sub1.Ok("a")
	sub1.Ok("b")
	sub1.Plan()
	tw.Ok("first")

	sub2 := tw.Subtest("second")
	n := sub2.Ok("c")
	sub2.Plan()
	tw.Ok("second")

	if n != 1 {
		t.Errorf("expected sub2 counter to start at 1, got %d", n)
	}
}
```

**Step 2: Run all writer tests**

Run: `cd ~/eng/repos/tap-dancer && nix develop --command bash -c "cd go && go test -run TestSubtest -v ./..."`
Expected: All PASS

**Step 3: Commit**

```
test(writer): add subtest edge case tests
```

---

### Task 3: Validate Subtest Output With Reader

**Files:**
- Test: `go/tap_test.go`

**Step 1: Write a round-trip test — Writer subtests validated by Reader**

```go
func TestSubtestOutputValidatesWithReader(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)

	sub := tw.Subtest("mypackage")
	sub.Ok("TestOne")
	sub.NotOk("TestTwo", map[string]string{"message": "failed"})
	sub.Plan()
	tw.NotOk("mypackage", nil)
	tw.Plan()

	reader := NewReader(strings.NewReader(buf.String()))
	summary := reader.Summary()
	if !summary.Valid {
		diags := reader.Diagnostics()
		for _, d := range diags {
			t.Errorf("diagnostic: line %d: %s: %s", d.Line, d.Severity, d.Message)
		}
		t.Fatalf("writer output did not validate as TAP-14:\n%s", buf.String())
	}
}
```

**Step 2: Run the test**

Run: `cd ~/eng/repos/tap-dancer && nix develop --command bash -c "cd go && go test -run TestSubtestOutputValidatesWithReader -v ./..."`
Expected: PASS (if it fails, the `indentWriter` formatting needs adjustment — fix before proceeding)

**Step 3: Commit**

```
test(writer): add round-trip subtest validation with reader
```

---

### Task 4: Event Converter — Types and Core Logic

**Files:**
- Create: `go/gotest.go`
- Test: `go/gotest_test.go`

**Step 1: Write the failing test for single-package pass conversion**

Create `go/gotest_test.go`:

```go
package tap

import (
	"bytes"
	"strings"
	"testing"
)

func TestConvertSinglePackageAllPass(t *testing.T) {
	jsonEvents := strings.Join([]string{
		`{"Action":"start","Package":"example.com/foo"}`,
		`{"Action":"run","Package":"example.com/foo","Test":"TestA"}`,
		`{"Action":"output","Package":"example.com/foo","Test":"TestA","Output":"=== RUN   TestA\n"}`,
		`{"Action":"pass","Package":"example.com/foo","Test":"TestA","Elapsed":0.001}`,
		`{"Action":"run","Package":"example.com/foo","Test":"TestB"}`,
		`{"Action":"output","Package":"example.com/foo","Test":"TestB","Output":"=== RUN   TestB\n"}`,
		`{"Action":"pass","Package":"example.com/foo","Test":"TestB","Elapsed":0.002}`,
		`{"Action":"output","Package":"example.com/foo","Output":"PASS\n"}`,
		`{"Action":"pass","Package":"example.com/foo","Elapsed":0.010}`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := ConvertGoTest(strings.NewReader(jsonEvents), &buf, false)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	out := buf.String()

	// Validate the output is valid TAP-14
	reader := NewReader(strings.NewReader(out))
	summary := reader.Summary()
	if !summary.Valid {
		for _, d := range reader.Diagnostics() {
			t.Errorf("diagnostic: line %d: %s: %s", d.Line, d.Severity, d.Message)
		}
		t.Fatalf("output is not valid TAP-14:\n%s", out)
	}

	// Should have package as subtest with 2 inner tests
	if !strings.Contains(out, "# Subtest: example.com/foo") {
		t.Errorf("expected package subtest, got:\n%s", out)
	}
	if !strings.Contains(out, "ok 1 - example.com/foo") {
		t.Errorf("expected parent ok for package, got:\n%s", out)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd ~/eng/repos/tap-dancer && nix develop --command bash -c "cd go && go test -run TestConvertSinglePackageAllPass -v ./..."`
Expected: Compilation error — `ConvertGoTest` does not exist.

**Step 3: Implement the converter**

Create `go/gotest.go`:

```go
package tap

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

type testEvent struct {
	Time    time.Time `json:"Time"`
	Action  string    `json:"Action"`
	Package string    `json:"Package"`
	Test    string    `json:"Test"`
	Elapsed float64   `json:"Elapsed"`
	Output  string    `json:"Output"`
}

type testResult struct {
	name    string
	action  string // pass, fail, skip
	elapsed float64
	output  strings.Builder
}

type packageResult struct {
	name    string
	tests   []*testResult
	testMap map[string]*testResult
	output  strings.Builder
	failed  bool
	elapsed float64
}

var fileLineRe = regexp.MustCompile(`(\w[\w_]*\.go):(\d+):`)

func parseFileLine(output string) (file string, line string) {
	m := fileLineRe.FindStringSubmatch(output)
	if m != nil {
		return m[1], m[2]
	}
	return "", ""
}

// ConvertGoTest reads go test -json events from r and writes TAP-14 to w.
// If verbose is true, passing tests include output diagnostics.
// Returns an exit code: 0 for all pass, 1 for any failure, 2 for build errors.
func ConvertGoTest(r io.Reader, w io.Writer, verbose bool) int {
	scanner := bufio.NewScanner(r)

	packages := make(map[string]*packageResult)
	var packageOrder []string

	tw := NewWriter(w)
	exitCode := 0

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var ev testEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			tw.Comment(fmt.Sprintf("unparseable: %s", line))
			continue
		}

		pkg := packages[ev.Package]
		if pkg == nil {
			pkg = &packageResult{
				name:    ev.Package,
				testMap: make(map[string]*testResult),
			}
			packages[ev.Package] = pkg
			packageOrder = append(packageOrder, ev.Package)
		}

		if ev.Test == "" {
			// Package-level event
			switch ev.Action {
			case "output":
				pkg.output.WriteString(ev.Output)
			case "pass":
				pkg.elapsed = ev.Elapsed
				emitPackage(tw, pkg, verbose)
			case "fail":
				pkg.failed = true
				pkg.elapsed = ev.Elapsed
				emitPackage(tw, pkg, verbose)
				if exitCode < 1 {
					exitCode = 1
				}
			}
			continue
		}

		// Test-level event
		tr := pkg.testMap[ev.Test]
		if tr == nil {
			tr = &testResult{name: ev.Test}
			pkg.testMap[ev.Test] = tr
			pkg.tests = append(pkg.tests, tr)
		}

		switch ev.Action {
		case "output":
			tr.output.WriteString(ev.Output)
		case "pass":
			tr.action = "pass"
			tr.elapsed = ev.Elapsed
		case "fail":
			tr.action = "fail"
			tr.elapsed = ev.Elapsed
		case "skip":
			tr.action = "skip"
			tr.elapsed = ev.Elapsed
		}
	}

	tw.Plan()
	return exitCode
}

func emitPackage(tw *Writer, pkg *packageResult, verbose bool) {
	sub := tw.Subtest(pkg.name)

	for _, tr := range pkg.tests {
		// Skip subtests — they'll be emitted by their parent
		if strings.Contains(tr.name, "/") {
			continue
		}
		emitTest(sub, pkg, tr, verbose)
	}

	sub.Plan()

	if pkg.failed {
		tw.NotOk(pkg.name, nil)
	} else {
		tw.Ok(pkg.name)
	}
}

func emitTest(tw *Writer, pkg *packageResult, tr *testResult, verbose bool) {
	// Check for child subtests
	prefix := tr.name + "/"
	var children []*testResult
	for _, child := range pkg.tests {
		if strings.HasPrefix(child.name, prefix) && !strings.Contains(child.name[len(prefix):], "/") {
			children = append(children, child)
		}
	}

	if len(children) > 0 {
		sub := tw.Subtest(tr.name)
		for _, child := range children {
			emitTest(sub, pkg, child, verbose)
		}
		sub.Plan()
		if tr.action == "fail" {
			tw.NotOk(tr.name, nil)
		} else {
			tw.Ok(tr.name)
		}
		return
	}

	// Leaf test
	name := tr.name
	// For display, use just the last segment
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = tr.name[idx+1:]
	}

	output := cleanTestOutput(tr.output.String())

	switch tr.action {
	case "pass":
		if verbose && output != "" {
			tw.Ok(name)
			// Note: Ok doesn't support diagnostics, so verbose pass
			// output is handled as a comment or we extend Ok.
			// For now, use NotOk-style diagnostics isn't appropriate.
			// Pass tests with verbose just get ok line.
			// TODO: Consider adding OkWithDiagnostics or using comments.
		} else {
			tw.Ok(name)
		}
	case "fail":
		diag := map[string]string{
			"elapsed": fmt.Sprintf("%.3f", tr.elapsed),
			"package": pkg.name,
		}
		if output != "" {
			diag["message"] = output
		}
		file, line := parseFileLine(output)
		if file != "" {
			diag["file"] = file
			diag["line"] = line
		}
		tw.NotOk(name, diag)
	case "skip":
		reason := extractSkipReason(output)
		tw.Skip(name, reason)
	default:
		tw.Ok(name)
	}
}

func cleanTestOutput(raw string) string {
	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		// Skip go test framework lines
		if strings.HasPrefix(trimmed, "=== RUN") ||
			strings.HasPrefix(trimmed, "=== PAUSE") ||
			strings.HasPrefix(trimmed, "=== CONT") ||
			strings.HasPrefix(trimmed, "--- PASS") ||
			strings.HasPrefix(trimmed, "--- FAIL") ||
			strings.HasPrefix(trimmed, "--- SKIP") ||
			trimmed == "PASS" || trimmed == "FAIL" ||
			trimmed == "" {
			continue
		}
		lines = append(lines, strings.TrimSpace(line))
	}
	return strings.Join(lines, "\n")
}

func extractSkipReason(output string) string {
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--- SKIP") {
			continue
		}
		// Skip reason is typically the line after --- SKIP
		// or the t.Skip() message in the output
		if trimmed != "" &&
			!strings.HasPrefix(trimmed, "=== RUN") &&
			!strings.HasPrefix(trimmed, "=== PAUSE") &&
			!strings.HasPrefix(trimmed, "=== CONT") {
			return trimmed
		}
	}
	return ""
}
```

**Step 4: Run test to verify it passes**

Run: `cd ~/eng/repos/tap-dancer && nix develop --command bash -c "cd go && go test -run TestConvertSinglePackageAllPass -v ./..."`
Expected: PASS

**Step 5: Commit**

```
feat: add go test -json to TAP-14 converter
```

---

### Task 5: Converter — Failure and Skip Tests

**Files:**
- Test: `go/gotest_test.go`

**Step 1: Write tests for failures, skips, and mixed results**

Add to `go/gotest_test.go`:

```go
func TestConvertFailingTest(t *testing.T) {
	jsonEvents := strings.Join([]string{
		`{"Action":"run","Package":"example.com/foo","Test":"TestBad"}`,
		`{"Action":"output","Package":"example.com/foo","Test":"TestBad","Output":"=== RUN   TestBad\n"}`,
		`{"Action":"output","Package":"example.com/foo","Test":"TestBad","Output":"    foo_test.go:10: expected 1, got 2\n"}`,
		`{"Action":"fail","Package":"example.com/foo","Test":"TestBad","Elapsed":0.003}`,
		`{"Action":"output","Package":"example.com/foo","Output":"FAIL\n"}`,
		`{"Action":"fail","Package":"example.com/foo","Elapsed":0.010}`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := ConvertGoTest(strings.NewReader(jsonEvents), &buf, false)

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}

	out := buf.String()
	if !strings.Contains(out, "not ok") {
		t.Errorf("expected not ok in output:\n%s", out)
	}
	if !strings.Contains(out, "foo_test.go") {
		t.Errorf("expected file reference in diagnostics:\n%s", out)
	}

	reader := NewReader(strings.NewReader(out))
	if !reader.Summary().Valid {
		t.Errorf("output is not valid TAP-14:\n%s", out)
	}
}

func TestConvertSkippedTest(t *testing.T) {
	jsonEvents := strings.Join([]string{
		`{"Action":"run","Package":"example.com/foo","Test":"TestSkip"}`,
		`{"Action":"output","Package":"example.com/foo","Test":"TestSkip","Output":"=== RUN   TestSkip\n"}`,
		`{"Action":"output","Package":"example.com/foo","Test":"TestSkip","Output":"    foo_test.go:5: not applicable\n"}`,
		`{"Action":"output","Package":"example.com/foo","Test":"TestSkip","Output":"--- SKIP: TestSkip (0.00s)\n"}`,
		`{"Action":"skip","Package":"example.com/foo","Test":"TestSkip","Elapsed":0.0}`,
		`{"Action":"output","Package":"example.com/foo","Output":"PASS\n"}`,
		`{"Action":"pass","Package":"example.com/foo","Elapsed":0.005}`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := ConvertGoTest(strings.NewReader(jsonEvents), &buf, false)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	out := buf.String()
	if !strings.Contains(out, "# SKIP") {
		t.Errorf("expected SKIP directive:\n%s", out)
	}

	reader := NewReader(strings.NewReader(out))
	if !reader.Summary().Valid {
		t.Errorf("output is not valid TAP-14:\n%s", out)
	}
}
```

**Step 2: Run tests**

Run: `cd ~/eng/repos/tap-dancer && nix develop --command bash -c "cd go && go test -run 'TestConvert(Failing|Skipped)' -v ./..."`
Expected: PASS

**Step 3: Commit**

```
test: add converter tests for failures and skips
```

---

### Task 6: Converter — Subtests

**Files:**
- Test: `go/gotest_test.go`

**Step 1: Write test for Go subtests mapping to nested TAP-14**

```go
func TestConvertSubtests(t *testing.T) {
	jsonEvents := strings.Join([]string{
		`{"Action":"run","Package":"example.com/foo","Test":"TestParent"}`,
		`{"Action":"output","Package":"example.com/foo","Test":"TestParent","Output":"=== RUN   TestParent\n"}`,
		`{"Action":"run","Package":"example.com/foo","Test":"TestParent/child_a"}`,
		`{"Action":"output","Package":"example.com/foo","Test":"TestParent/child_a","Output":"=== RUN   TestParent/child_a\n"}`,
		`{"Action":"pass","Package":"example.com/foo","Test":"TestParent/child_a","Elapsed":0.001}`,
		`{"Action":"run","Package":"example.com/foo","Test":"TestParent/child_b"}`,
		`{"Action":"output","Package":"example.com/foo","Test":"TestParent/child_b","Output":"=== RUN   TestParent/child_b\n"}`,
		`{"Action":"pass","Package":"example.com/foo","Test":"TestParent/child_b","Elapsed":0.001}`,
		`{"Action":"pass","Package":"example.com/foo","Test":"TestParent","Elapsed":0.003}`,
		`{"Action":"output","Package":"example.com/foo","Output":"PASS\n"}`,
		`{"Action":"pass","Package":"example.com/foo","Elapsed":0.010}`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := ConvertGoTest(strings.NewReader(jsonEvents), &buf, false)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	out := buf.String()

	// Should have nested subtest for TestParent
	if !strings.Contains(out, "# Subtest: TestParent") {
		t.Errorf("expected TestParent subtest:\n%s", out)
	}
	if !strings.Contains(out, "child_a") {
		t.Errorf("expected child_a in output:\n%s", out)
	}

	reader := NewReader(strings.NewReader(out))
	if !reader.Summary().Valid {
		for _, d := range reader.Diagnostics() {
			t.Errorf("diagnostic: line %d: %s: %s", d.Line, d.Severity, d.Message)
		}
		t.Fatalf("output is not valid TAP-14:\n%s", out)
	}
}
```

**Step 2: Run test**

Run: `cd ~/eng/repos/tap-dancer && nix develop --command bash -c "cd go && go test -run TestConvertSubtests -v ./..."`
Expected: PASS

**Step 3: Commit**

```
test: add converter test for Go subtest nesting
```

---

### Task 7: Converter — Multiple Packages

**Files:**
- Test: `go/gotest_test.go`

**Step 1: Write test for multiple packages with interleaved events**

```go
func TestConvertMultiplePackages(t *testing.T) {
	jsonEvents := strings.Join([]string{
		`{"Action":"run","Package":"example.com/foo","Test":"TestFoo"}`,
		`{"Action":"run","Package":"example.com/bar","Test":"TestBar"}`,
		`{"Action":"output","Package":"example.com/foo","Test":"TestFoo","Output":"=== RUN   TestFoo\n"}`,
		`{"Action":"output","Package":"example.com/bar","Test":"TestBar","Output":"=== RUN   TestBar\n"}`,
		`{"Action":"pass","Package":"example.com/foo","Test":"TestFoo","Elapsed":0.001}`,
		`{"Action":"pass","Package":"example.com/bar","Test":"TestBar","Elapsed":0.002}`,
		`{"Action":"output","Package":"example.com/foo","Output":"PASS\n"}`,
		`{"Action":"pass","Package":"example.com/foo","Elapsed":0.005}`,
		`{"Action":"output","Package":"example.com/bar","Output":"PASS\n"}`,
		`{"Action":"pass","Package":"example.com/bar","Elapsed":0.006}`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := ConvertGoTest(strings.NewReader(jsonEvents), &buf, false)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	out := buf.String()
	if !strings.Contains(out, "# Subtest: example.com/foo") {
		t.Errorf("expected foo package subtest:\n%s", out)
	}
	if !strings.Contains(out, "# Subtest: example.com/bar") {
		t.Errorf("expected bar package subtest:\n%s", out)
	}
	if !strings.Contains(out, "1..2") {
		t.Errorf("expected plan 1..2:\n%s", out)
	}

	reader := NewReader(strings.NewReader(out))
	if !reader.Summary().Valid {
		t.Fatalf("output is not valid TAP-14:\n%s", out)
	}
}
```

**Step 2: Run test**

Run: `cd ~/eng/repos/tap-dancer && nix develop --command bash -c "cd go && go test -run TestConvertMultiplePackages -v ./..."`
Expected: PASS

**Step 3: Commit**

```
test: add converter test for multiple interleaved packages
```

---

### Task 8: CLI Subcommand — `go-test`

**Files:**
- Modify: `go/cmd/tap-dancer/main.go`

**Step 1: Register the `go-test` command and implement the handler**

Add to `registerCommands()` in `main.go`:

```go
app.AddCommand(&command.Command{
	Name:        "go-test",
	Description: command.Description{Short: "Run go test and convert output to TAP-14"},
	Params: []command.Param{
		{Name: "verbose", Short: 'v', Type: command.Bool, Description: "Pass -v to go test and include output for passing tests", Required: false},
	},
	RunCLI: handleGoTest,
})
```

Implement `handleGoTest`:

```go
func handleGoTest(ctx context.Context, args json.RawMessage) error {
	var params struct {
		Verbose bool `json:"verbose"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}

	// Build go test command args: everything after "go-test" in os.Args
	goTestArgs := []string{"test", "-json"}
	if params.Verbose {
		goTestArgs = append(goTestArgs, "-v")
	}

	// Find remaining args from os.Args after "go-test"
	for i, arg := range os.Args {
		if arg == "go-test" {
			// Skip flags we handle (-v/--verbose) and collect the rest
			rest := os.Args[i+1:]
			for _, a := range rest {
				if a == "-v" || a == "--verbose" {
					continue
				}
				goTestArgs = append(goTestArgs, a)
			}
			break
		}
	}

	cmd := exec.CommandContext(ctx, "go", goTestArgs...)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		// Bail out if go test can't start
		tw := tap.NewWriter(os.Stdout)
		tw.BailOut(fmt.Sprintf("failed to start go test: %v", err))
		return err
	}

	exitCode := tap.ConvertGoTest(stdout, os.Stdout, params.Verbose)

	// Wait for command to finish (ignore error — we use our own exit code)
	cmd.Wait()

	if exitCode != 0 {
		os.Exit(exitCode)
	}

	return nil
}
```

Add `"os/exec"` to the imports.

Also update `flag.Usage` to include the `go-test` command:

```go
fmt.Fprintf(os.Stderr, "  go-test [args...]    Run go test and convert output to TAP-14\n")
```

**Step 2: Run existing tests to make sure nothing breaks**

Run: `cd ~/eng/repos/tap-dancer && nix develop --command bash -c "cd go && go test ./..."`
Expected: All PASS

**Step 3: Smoke test the CLI**

Run: `cd ~/eng/repos/tap-dancer && nix develop --command bash -c "cd go && go run ./cmd/tap-dancer go-test ./..."`
Expected: TAP-14 output with package subtests

**Step 4: Commit**

```
feat: add go-test subcommand
```

---

### Task 9: Justfile Recipes

**Files:**
- Modify: `justfile`

**Step 1: Add test and smoke-test recipes**

Add to `justfile`:

```just
test-go-test:
    nix develop --command bash -c "go run ./go/cmd/tap-dancer go-test ./go/... | go run ./go/cmd/tap-dancer validate"
```

**Step 2: Run the recipe**

Run: `cd ~/eng/repos/tap-dancer && just test-go-test`
Expected: Valid TAP-14 output confirmed by the validator

**Step 3: Commit**

```
chore: add test-go-test justfile recipe
```

---

### Task 10: Format, Tidy, and Final Verification

**Files:**
- All modified Go files

**Step 1: Format**

Run: `cd ~/eng/repos/tap-dancer && just fmt-go`

**Step 2: Tidy deps**

Run: `cd ~/eng/repos/tap-dancer && just deps`

**Step 3: Run full test suite**

Run: `cd ~/eng/repos/tap-dancer && just test`
Expected: All tests pass

**Step 4: Run nix build**

Run: `cd ~/eng/repos/tap-dancer && just build`
Expected: Build succeeds

**Step 5: Commit any formatting/dep changes**

```
chore: format and tidy
```
