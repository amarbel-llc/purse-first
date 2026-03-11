package tap

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func collect(ch <-chan ExecResult) []ExecResult {
	var results []ExecResult
	for r := range ch {
		results = append(results, r)
	}
	return results
}

func chanFromSlice(results []ExecResult) <-chan ExecResult {
	ch := make(chan ExecResult, len(results))
	for _, r := range results {
		ch <- r
	}
	close(ch)
	return ch
}

func TestGoroutineExecutorAllSucceed(t *testing.T) {
	executor := &GoroutineExecutor{}
	results := collect(executor.Run(context.Background(), "echo {}", []string{"hello", "world"}))

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
	results := collect(executor.Run(context.Background(), "echo {}", []string{"a", "b", "c", "d", "e"}))

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
	results := collect(executor.Run(context.Background(), "exit 1", []string{"x"}))

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", results[0].ExitCode)
	}
}

func TestGoroutineExecutorCapturesStderr(t *testing.T) {
	executor := &GoroutineExecutor{}
	results := collect(executor.Run(context.Background(), "echo err >&2", []string{"x"}))

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if string(results[0].Stderr) != "err\n" {
		t.Errorf("stderr: expected %q, got %q", "err\n", string(results[0].Stderr))
	}
}

func TestGoroutineExecutorSubstitution(t *testing.T) {
	executor := &GoroutineExecutor{}
	results := collect(executor.Run(context.Background(), "echo prefix-{}-suffix", []string{"mid"}))

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

func TestConvertExecParallelAllPass(t *testing.T) {
	results := chanFromSlice([]ExecResult{
		{Arg: "a", Command: "echo a", ExitCode: 0, Stdout: []byte("a\n")},
		{Arg: "b", Command: "echo b", ExitCode: 0, Stdout: []byte("b\n")},
	})

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
	results := chanFromSlice([]ExecResult{
		{Arg: "a", Command: "echo a", ExitCode: 0, Stdout: []byte("a\n")},
		{Arg: "b", Command: "false", ExitCode: 1, Stdout: []byte(""), Stderr: []byte("something broke\n")},
	})

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

func TestConvertExecParallelNoDiagOnSuccess(t *testing.T) {
	results := chanFromSlice([]ExecResult{
		{Arg: "a", Command: "echo a", ExitCode: 0, Stdout: []byte("output\n")},
	})

	var buf bytes.Buffer
	ConvertExecParallel(results, &buf, false, false)

	out := buf.String()
	if strings.Contains(out, "---") {
		t.Errorf("verbose=false should not include diagnostics on success, got:\n%s", out)
	}
}

func TestConvertExecParallelVerboseIncludesDiagOnSuccess(t *testing.T) {
	results := chanFromSlice([]ExecResult{
		{Arg: "a", Command: "echo a", ExitCode: 0, Stdout: []byte("output\n")},
	})

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

func TestGoroutineExecutorContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	executor := &GoroutineExecutor{}
	results := collect(executor.Run(ctx, "sleep 10", []string{"a"}))

	if results[0].Err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestGoroutineExecutorEmptyArgs(t *testing.T) {
	executor := &GoroutineExecutor{}
	results := collect(executor.Run(context.Background(), "echo {}", []string{}))

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}
