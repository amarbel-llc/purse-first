package operation_viewport

import (
	"context"
	"fmt"
	"os"

	"github.com/amarbel-llc/purse-first/libs/dewey/internal/0/primordial"
	tea "github.com/charmbracelet/bubbletea"
)

// Batch describes a many-op run. The Run callback is the work loop;
// it receives an [Emitter] for posting events to the viewport.
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
	// Run is the work loop. Its return value becomes [BatchDone.Err].
	Run func(emit Emitter) error
}

// RunBatch drives a many-op work loop through the viewport. On a non-
// TTY stdout, the work loop still runs; log lines and operation
// transitions are written as plain text to stderr.
func RunBatch(ctx context.Context, b Batch) error {
	if b.Run == nil {
		return fmt.Errorf("operation_viewport: RunBatch requires Batch.Run")
	}
	if !primordial.IsTTY(os.Stdout) {
		return runBatchStreaming(b)
	}
	return runBatchWithViewport(ctx, b)
}

// runBatchStreaming is the non-TTY fallback. Log lines and operation
// transitions are written to stderr; the user still gets a transcript,
// just without the spinner/tail UI.
func runBatchStreaming(b Batch) error {
	if b.Title != "" {
		fmt.Fprintln(os.Stderr, b.Title)
	}
	emit := streamingEmitter{}
	err := b.Run(emit)
	if err != nil {
		return err
	}
	return nil
}

// runBatchWithViewport is the TTY path: spin up a bubbletea program,
// run the work loop in a goroutine, route the loop's return into
// BatchDone.
func runBatchWithViewport(ctx context.Context, b Batch) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	opts := []Option{
		WithTitle(b.Title),
		WithCancel(cancel),
	}
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
		err := b.Run(programEmitter{program: program})
		runErrCh <- err
		program.Send(BatchDone{Err: err})
	}()

	if _, err := program.Run(); err != nil {
		cancel()
		<-runErrCh
		return fmt.Errorf("running viewport ui: %w", err)
	}
	return <-runErrCh
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
