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
