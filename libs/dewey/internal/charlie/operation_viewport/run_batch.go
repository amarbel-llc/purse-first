package operation_viewport

import (
	"fmt"
	"os"

	"github.com/amarbel-llc/purse-first/libs/dewey/internal/0/interfaces"
	"github.com/amarbel-llc/purse-first/libs/dewey/internal/0/primordial"
	"github.com/amarbel-llc/purse-first/libs/dewey/internal/bravo/errors"
	tea "github.com/charmbracelet/bubbletea"
)

// Batch describes a many-op run. The Run callback is the work loop;
// it receives the inner [interfaces.ActiveContext] plus an [Emitter]
// for posting events to the viewport.
type Batch struct {
	// Title is shown next to the spinner before the first
	// OperationStarted, and in the terminal success or failure line.
	Title string
	// Total is the count of operations the loop will run, used to
	// drive the progress bar.
	Total int
	// Lines overrides the default tail size.
	Lines int
	// Style overrides the default palette.
	Style *Style
	// Run is the work loop. Its return value becomes the cancel cause
	// on the inner context (and therefore the return of RunBatch).
	// The ctx passed in is a child of the parent passed to RunBatch;
	// callers should respect ctx.Done() so Ctrl-C in the viewport
	// terminates the loop.
	Run func(ctx interfaces.ActiveContext, emit Emitter) error
}

// RunBatch drives a many-op work loop through the viewport. On a non-
// TTY stdout, the work loop still runs; log lines and operation
// transitions are written as plain text to stderr.
//
// On Ctrl-C the inner [errors.Context] is cancelled with an
// [errors.Signal]; the caller's Run loop sees the cancellation via
// ctx.Done() and is expected to terminate.
func RunBatch(parent interfaces.ActiveContext, b Batch) error {
	if b.Run == nil {
		return fmt.Errorf("operation_viewport: RunBatch requires Batch.Run")
	}
	if !primordial.IsTTY(os.Stdout) {
		return runBatchStreaming(parent, b)
	}
	return runBatchWithViewport(parent, b)
}

// runBatchStreaming is the non-TTY fallback. Log lines and operation
// transitions are written to stderr; the user still gets a transcript,
// just without the spinner/tail UI.
func runBatchStreaming(parent interfaces.ActiveContext, b Batch) error {
	ctx := errors.MakeContext(parent)
	return ctx.Run(func(c errors.Context) {
		if b.Title != "" {
			fmt.Fprintln(os.Stderr, b.Title)
		}
		if err := b.Run(c, streamingEmitter{}); err != nil {
			c.Cancel(err)
		}
	})
}

// runBatchWithViewport is the TTY path: spin up a bubbletea program,
// run the work loop in a goroutine, route the loop's return into
// BatchDone, and propagate Ctrl-C through c.Cancel from the goroutine
// that lives inside [errors.Context.Run]'s recover scope.
func runBatchWithViewport(parent interfaces.ActiveContext, b Batch) error {
	ctx := errors.MakeContext(parent)
	return ctx.Run(func(c errors.Context) {
		opts := []Option{WithTitle(b.Title)}
		if b.Total > 0 {
			opts = append(opts, WithTotal(b.Total))
		}
		if b.Lines > 0 {
			opts = append(opts, WithLines(b.Lines))
		}
		if b.Style != nil {
			opts = append(opts, WithStyle(*b.Style))
		}
		model := NewModel(opts...)
		program := tea.NewProgram(model)

		runErrCh := make(chan error, 1)
		go func() {
			err := b.Run(c, programEmitter{program: program})
			runErrCh <- err
			program.Send(BatchDone{Err: err})
		}()

		finalModel, teaErr := program.Run()
		m, _ := finalModel.(Model)

		runErr := <-runErrCh

		propagateCancel(c, m, runErr, teaErr)
	})
}

// propagateCancel translates the final state of a bubbletea run into a
// cancel call on the [errors.Context]. Ctrl-C wins over a work-loop
// error which wins over a bubbletea error; if all are nil, funcRun
// returns normally and the framework records ContextStateSucceeded.
//
// The call panics out of the enclosing ctx.Run (that is the framework's
// recover-flow contract); callers must invoke this from inside
// funcRun's goroutine.
func propagateCancel(c errors.Context, m Model, runErr, teaErr error) {
	switch {
	case m.Interrupted():
		c.Cancel(errors.Signal{Signal: os.Interrupt})
	case runErr != nil:
		c.Cancel(runErr)
	case teaErr != nil:
		c.Cancel(fmt.Errorf("running viewport ui: %w", teaErr))
	}
}

// Emitter is the caller-facing handle for posting events to the
// viewport from inside a [Batch.Run] callback.
type Emitter interface {
	OperationStarted(name string, index int)
	OperationProgress(current, total int)
	OperationDone(err error)
	LogLine(text string)
}

// programEmitter implements Emitter on top of a [tea.Program]. Used by
// the TTY path.
type programEmitter struct {
	program *tea.Program
}

func (e programEmitter) OperationStarted(name string, index int) {
	e.program.Send(OperationStarted{Name: name, Index: index})
}

func (e programEmitter) OperationProgress(current, total int) {
	e.program.Send(OperationProgress{Current: current, Total: total})
}

func (e programEmitter) OperationDone(err error) {
	e.program.Send(OperationDone{Err: err})
}

func (e programEmitter) LogLine(text string) {
	e.program.Send(LogLine{Text: text})
}

// streamingEmitter implements Emitter on the non-TTY fallback path. It
// writes each event to stderr as one line.
type streamingEmitter struct{}

func (streamingEmitter) OperationStarted(name string, index int) {
	if index > 0 {
		fmt.Fprintf(os.Stderr, "[%d] %s\n", index, name)
	} else {
		fmt.Fprintln(os.Stderr, name)
	}
}

func (streamingEmitter) OperationProgress(current, total int) {
	fmt.Fprintf(os.Stderr, "  progress: %d/%d\n", current, total)
}

func (streamingEmitter) OperationDone(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "  failed: %v\n", err)
	}
}

func (streamingEmitter) LogLine(text string) {
	fmt.Fprintln(os.Stderr, text)
}
