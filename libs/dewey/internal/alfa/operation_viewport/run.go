package operation_viewport

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/amarbel-llc/purse-first/libs/dewey/internal/0/primordial"
)

// Op describes a single subprocess to run under the viewport.
type Op struct {
	// Title is shown next to the spinner and in the BatchDone success
	// or failure line.
	Title string
	// Cmd is the subprocess to run. Its Stdout, Stderr and Stdin are
	// replaced by the PTY when the viewport is TTY-attached, and by
	// os.Stderr in the non-TTY fallback.
	Cmd *exec.Cmd
	// Lines is the rolling-tail size. Zero falls back to the package
	// default.
	Lines int
	// Style overrides the default palette. Zero value means defaults.
	Style *Style
}

// Run executes op.Cmd under the viewport, returning the subprocess's
// exit error. On a non-TTY stdout, falls back to streaming the
// subprocess's combined output to stderr unchanged.
//
// Run signals OperationStarted (Index=1, Total=1) on the model itself,
// so callers do not need to feed any messages.
func Run(ctx context.Context, op Op) error {
	if op.Cmd == nil {
		return fmt.Errorf("operation_viewport: Run requires Op.Cmd")
	}
	if primordial.IsTTY(os.Stdout) {
		return runWithViewport(ctx, op)
	}
	return runStreaming(op)
}

// runStreaming forwards the child's output to stderr unchanged. Used in
// CI, redirected stdout, and other non-interactive environments.
func runStreaming(op Op) error {
	if op.Title != "" {
		fmt.Fprintln(os.Stderr, op.Title)
	}
	op.Cmd.Stdout = os.Stderr
	op.Cmd.Stderr = os.Stderr
	if err := op.Cmd.Run(); err != nil {
		return wrapCmdErr(op, err)
	}
	return nil
}

// wrapCmdErr is shared between the TTY and non-TTY paths so error text
// is identical.
func wrapCmdErr(op Op, err error) error {
	args := op.Cmd.Path
	if len(op.Cmd.Args) > 1 {
		args = fmt.Sprintf("%s %v", op.Cmd.Path, op.Cmd.Args[1:])
	}
	return fmt.Errorf("%s: %w", args, err)
}

