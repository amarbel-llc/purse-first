# exec-parallel Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `tap-dancer exec-parallel` CLI command that runs shell commands in parallel (GNU parallel `:::` syntax) and emits TAP-14 test points.

**Architecture:** An `Executor` interface abstracts command execution (native goroutines now, GNU parallel forwarding later). A `ConvertExecParallel` function takes executor results and writes TAP-14 using the existing `Writer`. CLI registration follows the `go-test`/`cargo-test` pattern.

**Tech Stack:** Go standard library (`os/exec`, `sync`), existing `tap` package.

**Rollback:** N/A — purely additive new command.

---

### Task 1: Executor interface, ExecResult type, and GoroutineExecutor

**Files:**
- Create: `packages/tap-dancer/go/execparallel.go`
- Test: `packages/tap-dancer/go/execparallel_test.go`

**Step 1: Write the failing test**

In `packages/tap-dancer/go/execparallel_test.go`:

```go
package tap

import (
	"context"
	"testing"
)

func TestGoroutineExecutorAllSucceed(t *testing.T) {
	executor := &GoroutineExecutor{}
	results := executor.Run(context.Background(), "echo {}", []string{"hello", "world"})

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	for i, r := range results {
		if r.ExitCode != 0 {
			t.Errorf("result %d: expected exit code 0, got %d", i, r.ExitCode)
		}
	}

	if string(results[0].Stdout) != "hello\n" {
		t.Errorf("result 0 stdout: expected %q, got %q", "hello\n", string(results[0].Stdout))
	}
	if string(results[1].Stdout) != "world\n" {
		t.Errorf("result 1 stdout: expected %q, got %q", "world\n", string(results[1].Stdout))
	}
}

func TestGoroutineExecutorPreservesOrder(t *testing.T) {
	executor := &GoroutineExecutor{}
	results := executor.Run(context.Background(), "echo {}", []string{"a", "b", "c", "d", "e"})

	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}

	for i, expected := range []string{"a", "b", "c", "d", "e"} {
		if results[i].Arg != expected {
			t.Errorf("result %d: expected arg %q, got %q", i, expected, results[i].Arg)
		}
	}
}

func TestGoroutineExecutorFailingCommand(t *testing.T) {
	executor := &GoroutineExecutor{}
	results := executor.Run(context.Background(), "exit 1", []string{"x"})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", results[0].ExitCode)
	}
}

func TestGoroutineExecutorCapturesStderr(t *testing.T) {
	executor := &GoroutineExecutor{}
	results := executor.Run(context.Background(), "echo err >&2", []string{"x"})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if string(results[0].Stderr) != "err\n" {
		t.Errorf("stderr: expected %q, got %q", "err\n", string(results[0].Stderr))
	}
}

func TestGoroutineExecutorSubstitution(t *testing.T) {
	executor := &GoroutineExecutor{}
	results := executor.Run(context.Background(), "echo prefix-{}-suffix", []string{"mid"})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if string(results[0].Stdout) != "prefix-mid-suffix\n" {
		t.Errorf("stdout: expected %q, got %q", "prefix-mid-suffix\n", string(results[0].Stdout))
	}

	if results[0].Command != "echo prefix-mid-suffix" {
		t.Errorf("command: expected %q, got %q", "echo prefix-mid-suffix", results[0].Command)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test -run 'TestGoroutineExecutor' ./packages/tap-dancer/go/...`
Expected: Compilation error — `GoroutineExecutor` undefined.

**Step 3: Write minimal implementation**

In `packages/tap-dancer/go/execparallel.go`:

```go
package tap

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"sync"
	"syscall"
)

// ExecResult holds the outcome of a single parallel command execution.
type ExecResult struct {
	Arg      string
	Command  string
	ExitCode int
	Stdout   []byte
	Stderr   []byte
	Err      error
}

// Executor runs a template command against a list of arguments in parallel
// and returns results in argument order.
type Executor interface {
	Run(ctx context.Context, template string, args []string) []ExecResult
}

// GoroutineExecutor runs commands concurrently using goroutines.
type GoroutineExecutor struct{}

func expandTemplate(template, arg string) string {
	return strings.ReplaceAll(template, "{}", arg)
}

func (e *GoroutineExecutor) Run(ctx context.Context, template string, args []string) []ExecResult {
	results := make([]ExecResult, len(args))
	var wg sync.WaitGroup

	for i, arg := range args {
		wg.Add(1)
		go func(idx int, a string) {
			defer wg.Done()
			expanded := expandTemplate(template, a)
			results[idx] = runCommand(ctx, a, expanded)
		}(i, arg)
	}

	wg.Wait()
	return results
}

func runCommand(ctx context.Context, arg, expanded string) ExecResult {
	var stdout, stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, "sh", "-c", expanded)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			} else {
				exitCode = 1
			}
		} else {
			exitCode = 1
		}
	}

	return ExecResult{
		Arg:      arg,
		Command:  expanded,
		ExitCode: exitCode,
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		Err:      err,
	}
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test -run 'TestGoroutineExecutor' ./packages/tap-dancer/go/...`
Expected: PASS

**Step 5: Commit**

```
feat(tap-dancer): add Executor interface and GoroutineExecutor
```

---

### Task 2: ConvertExecParallel TAP emitter

**Files:**
- Modify: `packages/tap-dancer/go/execparallel.go`
- Test: `packages/tap-dancer/go/execparallel_test.go`

**Step 1: Write the failing test**

Append to `packages/tap-dancer/go/execparallel_test.go`:

```go
func TestConvertExecParallelAllPass(t *testing.T) {
	results := []ExecResult{
		{Arg: "a", Command: "echo a", ExitCode: 0, Stdout: []byte("a\n")},
		{Arg: "b", Command: "echo b", ExitCode: 0, Stdout: []byte("b\n")},
	}

	var buf bytes.Buffer
	exitCode := ConvertExecParallel(results, &buf, false, false)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	out := buf.String()
	reader := NewReader(strings.NewReader(out))
	summary := reader.Summary()
	if !summary.Valid {
		for _, d := range reader.Diagnostics() {
			t.Errorf("diagnostic: line %d: %s: %s", d.Line, d.Severity, d.Message)
		}
		t.Fatalf("output is not valid TAP-14:\n%s", out)
	}

	if summary.Passed != 2 {
		t.Errorf("expected 2 passed, got %d", summary.Passed)
	}
}

func TestConvertExecParallelWithFailure(t *testing.T) {
	results := []ExecResult{
		{Arg: "a", Command: "echo a", ExitCode: 0, Stdout: []byte("a\n")},
		{Arg: "b", Command: "false", ExitCode: 1, Stdout: []byte(""), Stderr: []byte("something broke\n")},
	}

	var buf bytes.Buffer
	exitCode := ConvertExecParallel(results, &buf, false, false)

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}

	out := buf.String()

	if !strings.Contains(out, "ok 1 - echo a") {
		t.Errorf("expected ok for first command, got:\n%s", out)
	}
	if !strings.Contains(out, "not ok 2 - false") {
		t.Errorf("expected not ok for second command, got:\n%s", out)
	}
	if !strings.Contains(out, "exit-code: 1") {
		t.Errorf("expected exit-code diagnostic, got:\n%s", out)
	}
	if !strings.Contains(out, "something broke") {
		t.Errorf("expected stderr in diagnostics, got:\n%s", out)
	}
}

func TestConvertExecParallelFailureNoDiagOnSuccess(t *testing.T) {
	results := []ExecResult{
		{Arg: "a", Command: "echo a", ExitCode: 0, Stdout: []byte("output\n")},
	}

	var buf bytes.Buffer
	ConvertExecParallel(results, &buf, false, false)

	out := buf.String()
	if strings.Contains(out, "---") {
		t.Errorf("verbose=false should not include diagnostics on success, got:\n%s", out)
	}
}

func TestConvertExecParallelVerboseIncludesDiagOnSuccess(t *testing.T) {
	results := []ExecResult{
		{Arg: "a", Command: "echo a", ExitCode: 0, Stdout: []byte("output\n")},
	}

	var buf bytes.Buffer
	ConvertExecParallel(results, &buf, true, false)

	out := buf.String()
	if !strings.Contains(out, "---") {
		t.Errorf("verbose=true should include diagnostics on success, got:\n%s", out)
	}
	if !strings.Contains(out, "output") {
		t.Errorf("verbose=true should include stdout, got:\n%s", out)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test -run 'TestConvertExecParallel' ./packages/tap-dancer/go/...`
Expected: Compilation error — `ConvertExecParallel` undefined.

**Step 3: Write minimal implementation**

Append to `packages/tap-dancer/go/execparallel.go`:

```go
// ConvertExecParallel writes TAP-14 output from parallel execution results.
// Returns 0 if all commands succeeded, 1 if any failed.
func ConvertExecParallel(results []ExecResult, w io.Writer, verbose bool, color bool) int {
	tw := NewColorWriter(w, color)
	exitCode := 0

	for _, r := range results {
		if r.ExitCode == 0 {
			if verbose {
				diags := execResultDiagnostics(r)
				tw.OkDiag(r.Command, diags)
			} else {
				tw.Ok(r.Command)
			}
		} else {
			exitCode = 1
			tw.NotOk(r.Command, execResultDiagnosticsMap(r))
		}
	}

	tw.Plan()
	return exitCode
}

func execResultDiagnostics(r ExecResult) *Diagnostics {
	d := &Diagnostics{
		Extras: make(map[string]any),
	}

	d.Extras["exit-code"] = r.ExitCode

	stdout := strings.TrimRight(string(r.Stdout), "\n")
	if stdout != "" {
		d.Extras["stdout"] = stdout
	}

	stderr := strings.TrimRight(string(r.Stderr), "\n")
	if stderr != "" {
		d.Extras["stderr"] = stderr
	}

	return d
}

func execResultDiagnosticsMap(r ExecResult) map[string]string {
	diags := map[string]string{
		"exit-code": fmt.Sprintf("%d", r.ExitCode),
	}

	stdout := strings.TrimRight(string(r.Stdout), "\n")
	if stdout != "" {
		diags["stdout"] = stdout
	}

	stderr := strings.TrimRight(string(r.Stderr), "\n")
	if stderr != "" {
		diags["stderr"] = stderr
	}

	return diags
}
```

Add `"fmt"` and `"io"` to the imports in `execparallel.go` if not already present.

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test -run 'TestConvertExecParallel' ./packages/tap-dancer/go/...`
Expected: PASS

**Step 5: Commit**

```
feat(tap-dancer): add ConvertExecParallel TAP emitter
```

---

### Task 3: CLI command registration and arg parsing

**Files:**
- Modify: `packages/tap-dancer/go/cmd/tap-dancer/main.go`

**Step 1: Write the failing test**

This is CLI integration — we test it manually and via the existing tests in Task 2. No new test file needed.

Verify current tests still pass: `nix develop --command go test ./packages/tap-dancer/go/...`

**Step 2: Register the command in `registerCommands()`**

Add to `packages/tap-dancer/go/cmd/tap-dancer/main.go` in `registerCommands()`, after the `reformat` command:

```go
app.AddCommand(&command.Command{
	Name:        "exec-parallel",
	Description: command.Description{Short: "Run commands in parallel and emit TAP-14 test points"},
	Params: []command.Param{
		{Name: "verbose", Type: command.Bool, Description: "Include stdout/stderr diagnostics on successful test points", Required: false},
	},
	RunCLI: handleExecParallel,
})
```

**Step 3: Add `handleExecParallel` function**

Add to `main.go`:

```go
func handleExecParallel(ctx context.Context, args json.RawMessage) error {
	var params struct {
		Verbose bool `json:"verbose"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}

	// Parse CLI args: everything after "exec-parallel", excluding our flags,
	// split on ":::" into template and args.
	var cliArgs []string
	for i, arg := range os.Args {
		if arg == "exec-parallel" {
			rest := os.Args[i+1:]
			for _, a := range rest {
				if a == "-v" || a == "--verbose" {
					continue
				}
				cliArgs = append(cliArgs, a)
			}
			break
		}
	}

	// Find ::: separator
	sepIdx := -1
	for i, a := range cliArgs {
		if a == ":::" {
			sepIdx = i
			break
		}
	}

	if sepIdx < 0 {
		return fmt.Errorf("missing ::: separator\nusage: tap-dancer exec-parallel [--verbose] <template> ::: <arg1> <arg2> ...")
	}

	if sepIdx == 0 {
		return fmt.Errorf("missing command template before :::\nusage: tap-dancer exec-parallel [--verbose] <template> ::: <arg1> <arg2> ...")
	}

	template := strings.Join(cliArgs[:sepIdx], " ")
	execArgs := cliArgs[sepIdx+1:]

	if len(execArgs) == 0 {
		return fmt.Errorf("no arguments after :::\nusage: tap-dancer exec-parallel [--verbose] <template> ::: <arg1> <arg2> ...")
	}

	color := stdoutIsTerminal()
	executor := &tap.GoroutineExecutor{}
	results := executor.Run(ctx, template, execArgs)
	exitCode := tap.ConvertExecParallel(results, os.Stdout, params.Verbose, color)

	if exitCode != 0 {
		os.Exit(exitCode)
	}

	return nil
}
```

**Step 4: Update `flag.Usage`**

Add to the usage text in `main()`:

```go
fmt.Fprintf(os.Stderr, "  exec-parallel       Run commands in parallel and emit TAP-14\n")
```

**Step 5: Manual verification**

Run: `nix develop --command go run ./packages/tap-dancer/go/cmd/tap-dancer exec-parallel 'echo {}' ::: hello world`
Expected:
```
TAP version 14
ok 1 - echo hello
ok 2 - echo world
1..2
```

Run: `nix develop --command go run ./packages/tap-dancer/go/cmd/tap-dancer exec-parallel 'exit 1' ::: a b`
Expected: TAP output with `not ok` test points and YAML diagnostics, exit code 1.

**Step 6: Run all tap-dancer tests**

Run: `nix develop --command go test ./packages/tap-dancer/go/...`
Expected: PASS

**Step 7: Commit**

```
feat(tap-dancer): register exec-parallel CLI command
```

---

### Task 4: Format and lint

**Step 1: Format**

Run: `nix develop --command gofumpt -w packages/tap-dancer/go/execparallel.go packages/tap-dancer/go/execparallel_test.go packages/tap-dancer/go/cmd/tap-dancer/main.go`

**Step 2: Lint**

Run: `nix develop --command go vet ./packages/tap-dancer/go/...`
Expected: No issues.

**Step 3: Full test suite**

Run: `nix develop --command go test ./packages/tap-dancer/go/...`
Expected: PASS

**Step 4: Commit if formatting changed**

```
chore(tap-dancer): format exec-parallel
```
