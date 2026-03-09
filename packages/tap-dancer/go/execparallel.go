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
