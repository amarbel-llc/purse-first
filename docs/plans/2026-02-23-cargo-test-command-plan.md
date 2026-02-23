# cargo-test command implementation plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `cargo-test` subcommand to tap-dancer that runs `cargo test -- --format json` and converts the JSON event stream to TAP-14.

**Architecture:** New `ConvertCargoTest` function in `cargotest.go` mirrors the existing `ConvertGoTest` pattern: read JSON events from an `io.Reader`, accumulate per-suite results, emit TAP-14 via the existing `Writer`. The CLI handler in `main.go` spawns `cargo test` with `-- --format json`, pipes stdout through the converter.

**Tech Stack:** Go, libtest JSON format, tap-dancer Writer API

**Design doc:** `docs/plans/2026-02-23-cargo-test-command-design.md`

---

### Task 1: ConvertCargoTest — single suite, all pass

**Files:**
- Create: `packages/tap-dancer/go/cargotest_test.go`
- Create: `packages/tap-dancer/go/cargotest.go`

**Step 1: Write the failing test**

Create `packages/tap-dancer/go/cargotest_test.go`:

```go
package tap

import (
	"bytes"
	"strings"
	"testing"
)

func TestCargoConvertSingleSuiteAllPass(t *testing.T) {
	jsonEvents := strings.Join([]string{
		`{ "type": "suite", "event": "started", "test_count": 2 }`,
		`{ "type": "test", "event": "started", "name": "tests::test_a" }`,
		`{ "type": "test", "name": "tests::test_a", "event": "ok", "exec_time": 0.001, "stdout": "" }`,
		`{ "type": "test", "event": "started", "name": "tests::test_b" }`,
		`{ "type": "test", "name": "tests::test_b", "event": "ok", "exec_time": 0.002, "stdout": "" }`,
		`{ "type": "suite", "event": "ok", "passed": 2, "failed": 0, "ignored": 0, "measured": 0, "filtered_out": 0, "exec_time": 0.005 }`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := ConvertCargoTest(strings.NewReader(jsonEvents), &buf, false, false)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	out := buf.String()

	// Should produce valid TAP-14
	reader := NewReader(strings.NewReader(out))
	summary := reader.Summary()
	if !summary.Valid {
		for _, d := range reader.Diagnostics() {
			t.Errorf("diagnostic: line %d: %s: %s", d.Line, d.Severity, d.Message)
		}
		t.Fatalf("output is not valid TAP-14:\n%s", out)
	}

	// Should have a subtest for the suite
	if !strings.Contains(out, "# Subtest:") {
		t.Errorf("expected suite subtest:\n%s", out)
	}
	if !strings.Contains(out, "tests::test_a") {
		t.Errorf("expected test_a in output:\n%s", out)
	}
	if !strings.Contains(out, "tests::test_b") {
		t.Errorf("expected test_b in output:\n%s", out)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd packages/tap-dancer/go && go test -v -run TestCargoConvertSingleSuiteAllPass ./...`
Expected: FAIL — `ConvertCargoTest` not defined

**Step 3: Write minimal implementation**

Create `packages/tap-dancer/go/cargotest.go`:

```go
package tap

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// cargoEvent represents a JSON event from `cargo test -- --format json`.
// Events have a "type" field ("suite" or "test") and an "event" field.
type cargoEvent struct {
	Type        string  `json:"type"`
	Event       string  `json:"event"`
	Name        string  `json:"name"`
	TestCount   int     `json:"test_count"`
	ExecTime    float64 `json:"exec_time"`
	Stdout      string  `json:"stdout"`
	Passed      int     `json:"passed"`
	Failed      int     `json:"failed"`
	Ignored     int     `json:"ignored"`
	Measured    int     `json:"measured"`
	FilteredOut int     `json:"filtered_out"`
}

type cargoTestResult struct {
	name    string
	event   string // ok, failed, ignored
	stdout  string
	elapsed float64
}

type cargoSuiteResult struct {
	name      string
	tests     []*cargoTestResult
	testCount int
	failed    bool
	elapsed   float64
}

// ConvertCargoTest reads cargo test --format json events from r and writes TAP-14 to w.
// If verbose is true, passing tests include output diagnostics.
// If skipEmpty is true, suites with no tests emit a SKIP directive instead of not ok.
// Returns an exit code: 0 for all pass, 1 for any failure.
func ConvertCargoTest(r io.Reader, w io.Writer, verbose bool, skipEmpty bool) int {
	scanner := bufio.NewScanner(r)
	tw := NewWriter(w)
	exitCode := 0

	var suites []*cargoSuiteResult
	var current *cargoSuiteResult

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var ev cargoEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			// Non-JSON line — check if it names a test binary
			name := parseCargoBinaryLine(line)
			if name != "" && current != nil {
				current.name = name
			} else if name != "" {
				// We'll set the name when suite starts
				// Store it for the next suite
				current = &cargoSuiteResult{name: name}
			} else {
				tw.Comment(fmt.Sprintf("unparseable: %s", line))
			}
			continue
		}

		switch ev.Type {
		case "suite":
			switch ev.Event {
			case "started":
				if current == nil {
					current = &cargoSuiteResult{}
				}
				current.testCount = ev.TestCount
				if current.name == "" {
					current.name = fmt.Sprintf("suite-%d", len(suites)+1)
				}
			case "ok", "failed":
				if current == nil {
					continue
				}
				current.elapsed = ev.ExecTime
				current.failed = ev.Event == "failed"
				suites = append(suites, current)

				emitCargoSuite(tw, current, verbose, skipEmpty)
				if current.failed && exitCode < 1 {
					exitCode = 1
				}
				if current.testCount == 0 && !skipEmpty && exitCode < 1 {
					exitCode = 1
				}
				current = nil
			}

		case "test":
			if current == nil {
				continue
			}
			switch ev.Event {
			case "started":
				// nothing to track yet
			case "ok", "failed", "ignored":
				current.tests = append(current.tests, &cargoTestResult{
					name:    ev.Name,
					event:   ev.Event,
					stdout:  ev.Stdout,
					elapsed: ev.ExecTime,
				})
			}
		}
	}

	tw.Plan()
	return exitCode
}

func parseCargoBinaryLine(line string) string {
	line = strings.TrimSpace(line)
	// Cargo outputs lines like:
	//   Running unittests src/lib.rs (target/debug/deps/my_crate-abc123)
	//   Running tests/integration.rs (target/debug/deps/integration-def456)
	if strings.HasPrefix(line, "Running ") {
		// Extract the binary description
		rest := strings.TrimPrefix(line, "Running ")
		if idx := strings.Index(rest, " ("); idx > 0 {
			return rest[:idx]
		}
		return rest
	}
	// Doc-tests line:
	//   Doc-tests my_crate
	if strings.HasPrefix(line, "Doc-tests ") {
		return line
	}
	return ""
}

func emitCargoSuite(tw *Writer, suite *cargoSuiteResult, verbose bool, skipEmpty bool) {
	if len(suite.tests) == 0 {
		if skipEmpty {
			tw.Skip(suite.name, "no tests")
		} else {
			tw.NotOk(suite.name, nil)
		}
		return
	}

	sub := tw.Subtest(suite.name)
	for _, tr := range suite.tests {
		emitCargoTest(sub, tr, verbose)
	}
	sub.Plan()

	if suite.failed {
		tw.NotOk(suite.name, nil)
	} else {
		tw.Ok(suite.name)
	}
}

func emitCargoTest(tw *Writer, tr *cargoTestResult, verbose bool) {
	switch tr.event {
	case "ok":
		tw.Ok(tr.name)
	case "failed":
		diag := map[string]string{
			"elapsed": fmt.Sprintf("%.3f", tr.elapsed),
		}
		stdout := strings.TrimSpace(tr.stdout)
		if stdout != "" {
			diag["message"] = stdout
			file, line := parseFileLine(stdout)
			if file != "" {
				diag["file"] = file
				diag["line"] = line
			}
		}
		tw.NotOk(tr.name, diag)
	case "ignored":
		tw.Skip(tr.name, "ignored")
	default:
		tw.Ok(tr.name)
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd packages/tap-dancer/go && go test -v -run TestCargoConvertSingleSuiteAllPass ./...`
Expected: PASS

**Step 5: Commit**

```
feat(tap-dancer): add ConvertCargoTest with single-suite passing test
```

---

### Task 2: Failing test, ignored test, and multiple suites

**Files:**
- Modify: `packages/tap-dancer/go/cargotest_test.go`

**Step 1: Write failing tests**

Add to `cargotest_test.go`:

```go
func TestCargoConvertFailingTest(t *testing.T) {
	jsonEvents := strings.Join([]string{
		`{ "type": "suite", "event": "started", "test_count": 1 }`,
		`{ "type": "test", "event": "started", "name": "tests::test_bad" }`,
		`{ "type": "test", "name": "tests::test_bad", "event": "failed", "exec_time": 0.003, "stdout": "thread 'tests::test_bad' panicked at src/lib.rs:10:5:\nassertion `left == right` failed\n  left: 1\n right: 2" }`,
		`{ "type": "suite", "event": "failed", "passed": 0, "failed": 1, "ignored": 0, "measured": 0, "filtered_out": 0, "exec_time": 0.010 }`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := ConvertCargoTest(strings.NewReader(jsonEvents), &buf, false, false)

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}

	out := buf.String()
	if !strings.Contains(out, "not ok") {
		t.Errorf("expected not ok in output:\n%s", out)
	}
	if !strings.Contains(out, "lib.rs") {
		t.Errorf("expected file reference in diagnostics:\n%s", out)
	}

	reader := NewReader(strings.NewReader(out))
	if !reader.Summary().Valid {
		t.Errorf("output is not valid TAP-14:\n%s", out)
	}
}

func TestCargoConvertIgnoredTest(t *testing.T) {
	jsonEvents := strings.Join([]string{
		`{ "type": "suite", "event": "started", "test_count": 2 }`,
		`{ "type": "test", "event": "started", "name": "tests::test_ok" }`,
		`{ "type": "test", "name": "tests::test_ok", "event": "ok", "exec_time": 0.001, "stdout": "" }`,
		`{ "type": "test", "event": "started", "name": "tests::test_ignored" }`,
		`{ "type": "test", "name": "tests::test_ignored", "event": "ignored" }`,
		`{ "type": "suite", "event": "ok", "passed": 1, "failed": 0, "ignored": 1, "measured": 0, "filtered_out": 0, "exec_time": 0.005 }`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := ConvertCargoTest(strings.NewReader(jsonEvents), &buf, false, false)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	out := buf.String()
	if !strings.Contains(out, "# SKIP") {
		t.Errorf("expected SKIP directive for ignored test:\n%s", out)
	}

	reader := NewReader(strings.NewReader(out))
	if !reader.Summary().Valid {
		t.Errorf("output is not valid TAP-14:\n%s", out)
	}
}

func TestCargoConvertMultipleSuites(t *testing.T) {
	jsonEvents := strings.Join([]string{
		// First suite — named by a "Running" line
		`Running unittests src/lib.rs (target/debug/deps/my_crate-abc123)`,
		`{ "type": "suite", "event": "started", "test_count": 1 }`,
		`{ "type": "test", "event": "started", "name": "tests::test_lib" }`,
		`{ "type": "test", "name": "tests::test_lib", "event": "ok", "exec_time": 0.001, "stdout": "" }`,
		`{ "type": "suite", "event": "ok", "passed": 1, "failed": 0, "ignored": 0, "measured": 0, "filtered_out": 0, "exec_time": 0.003 }`,
		// Second suite
		`Running tests/integration.rs (target/debug/deps/integration-def456)`,
		`{ "type": "suite", "event": "started", "test_count": 1 }`,
		`{ "type": "test", "event": "started", "name": "test_integration" }`,
		`{ "type": "test", "name": "test_integration", "event": "ok", "exec_time": 0.002, "stdout": "" }`,
		`{ "type": "suite", "event": "ok", "passed": 1, "failed": 0, "ignored": 0, "measured": 0, "filtered_out": 0, "exec_time": 0.004 }`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := ConvertCargoTest(strings.NewReader(jsonEvents), &buf, false, false)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	out := buf.String()
	if !strings.Contains(out, "# Subtest: unittests src/lib.rs") {
		t.Errorf("expected lib suite subtest:\n%s", out)
	}
	if !strings.Contains(out, "# Subtest: tests/integration.rs") {
		t.Errorf("expected integration suite subtest:\n%s", out)
	}
	if !strings.Contains(out, "1..2") {
		t.Errorf("expected plan 1..2:\n%s", out)
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

**Step 2: Run tests**

Run: `cd packages/tap-dancer/go && go test -v -run TestCargoCon ./...`
Expected: All PASS (implementation from Task 1 should handle these)

**Step 3: Fix any failures, then commit**

```
test(tap-dancer): add cargo-test failure, ignored, and multi-suite tests
```

---

### Task 3: Skip-empty tests

**Files:**
- Modify: `packages/tap-dancer/go/cargotest_test.go`

**Step 1: Write tests**

Add to `cargotest_test.go`:

```go
func TestCargoConvertEmptySuiteDefault(t *testing.T) {
	// Suite with test_count: 0 — without skip-empty, should produce not ok.
	jsonEvents := strings.Join([]string{
		`Running unittests src/lib.rs (target/debug/deps/my_crate-abc123)`,
		`{ "type": "suite", "event": "started", "test_count": 0 }`,
		`{ "type": "suite", "event": "ok", "passed": 0, "failed": 0, "ignored": 0, "measured": 0, "filtered_out": 0, "exec_time": 0.0 }`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := ConvertCargoTest(strings.NewReader(jsonEvents), &buf, false, false)

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}

	out := buf.String()
	if !strings.Contains(out, "not ok") {
		t.Errorf("expected not ok for empty suite:\n%s", out)
	}

	reader := NewReader(strings.NewReader(out))
	if !reader.Summary().Valid {
		for _, d := range reader.Diagnostics() {
			t.Errorf("diagnostic: line %d: %s: %s", d.Line, d.Severity, d.Message)
		}
		t.Fatalf("output is not valid TAP-14:\n%s", out)
	}
}

func TestCargoConvertEmptySuiteSkipEmpty(t *testing.T) {
	jsonEvents := strings.Join([]string{
		`Running unittests src/lib.rs (target/debug/deps/my_crate-abc123)`,
		`{ "type": "suite", "event": "started", "test_count": 0 }`,
		`{ "type": "suite", "event": "ok", "passed": 0, "failed": 0, "ignored": 0, "measured": 0, "filtered_out": 0, "exec_time": 0.0 }`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := ConvertCargoTest(strings.NewReader(jsonEvents), &buf, false, true)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	out := buf.String()
	if !strings.Contains(out, "# SKIP") {
		t.Errorf("expected SKIP directive:\n%s", out)
	}
	if strings.Contains(out, "not ok") {
		t.Errorf("expected no 'not ok' with skip-empty:\n%s", out)
	}

	reader := NewReader(strings.NewReader(out))
	if !reader.Summary().Valid {
		for _, d := range reader.Diagnostics() {
			t.Errorf("diagnostic: line %d: %s: %s", d.Line, d.Severity, d.Message)
		}
		t.Fatalf("output is not valid TAP-14:\n%s", out)
	}
}

func TestCargoConvertMixedEmptyAndRealSuites(t *testing.T) {
	jsonEvents := strings.Join([]string{
		`Running unittests src/lib.rs (target/debug/deps/my_crate-abc123)`,
		`{ "type": "suite", "event": "started", "test_count": 1 }`,
		`{ "type": "test", "event": "started", "name": "tests::test_real" }`,
		`{ "type": "test", "name": "tests::test_real", "event": "ok", "exec_time": 0.001, "stdout": "" }`,
		`{ "type": "suite", "event": "ok", "passed": 1, "failed": 0, "ignored": 0, "measured": 0, "filtered_out": 0, "exec_time": 0.003 }`,
		`Running unittests src/main.rs (target/debug/deps/my_crate-def456)`,
		`{ "type": "suite", "event": "started", "test_count": 0 }`,
		`{ "type": "suite", "event": "ok", "passed": 0, "failed": 0, "ignored": 0, "measured": 0, "filtered_out": 0, "exec_time": 0.0 }`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := ConvertCargoTest(strings.NewReader(jsonEvents), &buf, false, true)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	out := buf.String()
	if !strings.Contains(out, "# Subtest: unittests src/lib.rs") {
		t.Errorf("expected real suite subtest:\n%s", out)
	}
	if !strings.Contains(out, "# SKIP") {
		t.Errorf("expected SKIP for empty suite:\n%s", out)
	}
	if !strings.Contains(out, "1..2") {
		t.Errorf("expected plan 1..2:\n%s", out)
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

**Step 2: Run tests**

Run: `cd packages/tap-dancer/go && go test -v -run "TestCargoConvert(Empty|Mixed)" ./...`
Expected: All PASS

**Step 3: Commit**

```
test(tap-dancer): add cargo-test skip-empty and mixed suite tests
```

---

### Task 4: Register cargo-test CLI command

**Files:**
- Modify: `packages/tap-dancer/go/cmd/tap-dancer/main.go`

**Step 1: Add command registration and handler**

In `registerCommands()`, after the `go-test` command, add:

```go
app.AddCommand(&command.Command{
    Name:        "cargo-test",
    Description: command.Description{Short: "Run cargo test and convert output to TAP-14"},
    Params: []command.Param{
        {Name: "verbose", Type: command.Bool, Description: "Include output for passing tests", Required: false},
        {Name: "skip-empty", Type: command.Bool, Description: "Emit SKIP directive instead of not ok for suites with no tests", Required: false},
    },
    RunCLI: handleCargoTest,
})
```

Add `handleCargoTest` function (mirrors `handleGoTest`):

```go
func handleCargoTest(ctx context.Context, args json.RawMessage) error {
    var params struct {
        Verbose   bool `json:"verbose"`
        SkipEmpty bool `json:"skip-empty"`
    }
    if err := json.Unmarshal(args, &params); err != nil {
        return fmt.Errorf("invalid arguments: %w", err)
    }

    cargoArgs := []string{"test"}
    if params.Verbose {
        cargoArgs = append(cargoArgs, "-v")
    }

    // Collect extra args from CLI (after "cargo-test", excluding our flags)
    for i, arg := range os.Args {
        if arg == "cargo-test" {
            rest := os.Args[i+1:]
            for _, a := range rest {
                if a == "-v" || a == "--verbose" ||
                    a == "-skip-empty" || a == "--skip-empty" {
                    continue
                }
                cargoArgs = append(cargoArgs, a)
            }
            break
        }
    }

    // Append --format json after the -- separator
    cargoArgs = append(cargoArgs, "--", "--format", "json")

    cmd := exec.CommandContext(ctx, "cargo", cargoArgs...)
    cmd.Stderr = os.Stderr

    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return fmt.Errorf("creating stdout pipe: %w", err)
    }

    if err := cmd.Start(); err != nil {
        tw := tap.NewWriter(os.Stdout)
        tw.BailOut(fmt.Sprintf("failed to start cargo test: %v", err))
        return err
    }

    exitCode := tap.ConvertCargoTest(stdout, os.Stdout, params.Verbose, params.SkipEmpty)

    cmd.Wait()

    if exitCode != 0 {
        os.Exit(exitCode)
    }

    return nil
}
```

Update `flag.Usage` to include `cargo-test`.

**Step 2: Run all tests**

Run: `cd packages/tap-dancer/go && go test -v ./...`
Expected: All PASS

**Step 3: Commit**

```
feat(tap-dancer): register cargo-test CLI command
```

---

### Task 5: Build verification

**Step 1: Run full build and test**

Run: `just build test`
Expected: All pass

**Step 2: Commit if any fixups needed, then final commit**

```
feat(tap-dancer): add cargo-test command for cargo test → TAP-14 conversion
```
