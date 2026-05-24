package operation_viewport

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/amarbel-llc/purse-first/libs/dewey/internal/0/interfaces"
	"github.com/amarbel-llc/purse-first/libs/dewey/internal/0/primordial"
	"github.com/amarbel-llc/purse-first/libs/dewey/internal/bravo/errors"
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
//
// On Ctrl-C the underlying [errors.Context] is cancelled with an
// [errors.Signal], the child is killed, and the captured transcript is
// dumped to stderr.
func Run(parent interfaces.ActiveContext, op Op) error {
	if op.Cmd == nil {
		return fmt.Errorf("operation_viewport: Run requires Op.Cmd")
	}
	if primordial.IsTTY(os.Stdout) {
		return runWithViewport(parent, op)
	}
	return runStreaming(parent, op)
}

// runStreaming forwards the child's output to stderr unchanged. Used in
// CI, redirected stdout, and other non-interactive environments. The
// child is killed when parent is cancelled.
func runStreaming(parent interfaces.ActiveContext, op Op) error {
	ctx := errors.MakeContext(parent)
	return ctx.Run(func(c errors.Context) {
		if op.Title != "" {
			fmt.Fprintln(os.Stderr, op.Title)
		}
		op.Cmd.Stdout = os.Stderr
		op.Cmd.Stderr = os.Stderr

		if err := op.Cmd.Start(); err != nil {
			c.Cancel(fmt.Errorf("starting %s: %w", op.Cmd.Path, err))
		}

		watchAndKillOnCancel(c, op.Cmd)

		if err := op.Cmd.Wait(); err != nil {
			c.Cancel(wrapCmdErr(op, err))
		}
	})
}

// wrapCmdErr formats the failing command line shell-style ("podman load
// -i file.tar: exit status 1") so the error reads like what a user
// would have typed.
func wrapCmdErr(op Op, err error) error {
	cmdline := op.Cmd.Path
	if len(op.Cmd.Args) > 0 {
		cmdline = strings.Join(op.Cmd.Args, " ")
	}
	return fmt.Errorf("%s: %w", cmdline, err)
}

// watchAndKillOnCancel spawns a goroutine that kills the child process
// when the context is cancelled. The goroutine exits cleanly when the
// process exits naturally (its Process struct stays non-nil but the
// Kill becomes a no-op once the OS has reaped it).
//
// We do this because [xpty.WaitProcess] and [exec.Cmd.Wait] do not
// themselves respect a context.Context; they wrap [os.Process.Wait].
func watchAndKillOnCancel(c interfaces.ActiveContext, cmd *exec.Cmd) {
	go func() {
		<-c.Done()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()
}
