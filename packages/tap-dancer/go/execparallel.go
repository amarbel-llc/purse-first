package tap

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
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
// and streams results in argument order.
type Executor interface {
	Run(ctx context.Context, template string, args []string) <-chan ExecResult
}

// GoroutineExecutor runs commands concurrently using goroutines.
type GoroutineExecutor struct{}

// expandTemplate replaces {} with arg. Arguments are interpolated as-is
// into the shell command, mirroring GNU parallel's ::: semantics.
func expandTemplate(template, arg string) string {
	return strings.ReplaceAll(template, "{}", arg)
}

func (e *GoroutineExecutor) Run(ctx context.Context, template string, args []string) <-chan ExecResult {
	ch := make(chan ExecResult, len(args))

	if len(args) == 0 {
		close(ch)
		return ch
	}

	results := make([]ExecResult, len(args))
	done := make([]chan struct{}, len(args))
	for i := range done {
		done[i] = make(chan struct{})
	}

	for i, arg := range args {
		go func(idx int, a string) {
			expanded := expandTemplate(template, a)
			results[idx] = runCommand(ctx, a, expanded)
			close(done[idx])
		}(i, arg)
	}

	go func() {
		defer close(ch)
		for i := range args {
			<-done[i]
			ch <- results[i]
		}
	}()

	return ch
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

// ConvertExecParallel writes TAP-14 output from parallel execution results.
// Returns 0 if all commands succeeded, 1 if any failed.
func ConvertExecParallel(results <-chan ExecResult, w io.Writer, verbose bool, color bool) int {
	tw := NewColorWriter(w, color)
	exitCode := 0

	for r := range results {
		if r.ExitCode == 0 {
			if verbose {
				tw.OkDiag(r.Command, execResultDiagnostics(r))
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

	if r.Err != nil && stdout == "" && stderr == "" {
		d.Extras["error"] = r.Err.Error()
	}

	return d
}

func execResultDiagnosticsMap(r ExecResult) map[string]string {
	d := execResultDiagnostics(r)
	m := make(map[string]string, len(d.Extras))
	for k, v := range d.Extras {
		m[k] = fmt.Sprintf("%v", v)
	}
	return m
}
