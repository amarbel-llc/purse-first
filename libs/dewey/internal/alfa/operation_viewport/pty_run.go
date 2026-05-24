package operation_viewport

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/xpty"
)

// runWithViewport is the TTY path: allocate a PTY, attach the child to
// it, render the spinner + tail via bubbletea, dump the captured
// transcript to stderr on failure.
func runWithViewport(ctx context.Context, op Op) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pty, err := xpty.NewPty(ptyDefaultWidth, ptyDefaultHeight)
	if err != nil {
		return fmt.Errorf("allocating pty: %w", err)
	}
	defer pty.Close()

	if err := pty.Start(op.Cmd); err != nil {
		return fmt.Errorf("starting %s under pty: %w", op.Cmd.Path, err)
	}

	opts := []Option{
		WithTitle(op.Title),
		WithCancel(cancel),
	}
	if op.Lines > 0 {
		opts = append(opts, WithLines(op.Lines))
	}
	if op.Style != nil {
		opts = append(opts, WithStyle(*op.Style))
	}
	model := NewModel(opts...)
	program := tea.NewProgram(model)

	captured := newSyncBuffer()
	go scanPTY(pty, captured, func(line string) {
		program.Send(LogLine{Text: line})
	})

	cmdErrCh := make(chan error, 1)
	go func() {
		err := xpty.WaitProcess(ctx, op.Cmd)
		_ = pty.Close()
		cmdErrCh <- err
		program.Send(BatchDone{Err: err})
	}()

	// Seed the model with a single OperationStarted so its header
	// renders even before the first byte of output arrives.
	program.Send(OperationStarted{Name: op.Title, Index: 1, Total: 1})

	if _, err := program.Run(); err != nil {
		cancel()
		<-cmdErrCh
		return fmt.Errorf("running viewport ui: %w", err)
	}

	cmdErr := <-cmdErrCh
	if cmdErr != nil {
		_, _ = os.Stderr.Write(captured.Bytes())
		return wrapCmdErr(op, cmdErr)
	}
	return nil
}

// ptyDefaultWidth / ptyDefaultHeight are the dimensions handed to xpty
// for child processes. Wide enough for typical build-tool output
// without truncation; the viewport itself only renders the spinner row
// plus the tail.
const (
	ptyDefaultWidth  = 120
	ptyDefaultHeight = 40
)

// scanPTY reads the PTY master, tees bytes into captured, and emits
// each newline-terminated line via emit. Returns when the PTY hits EOF
// or an unrecoverable read error.
func scanPTY(r io.Reader, captured *syncBuffer, emit func(string)) {
	tee := io.TeeReader(r, captured)
	scanner := bufio.NewScanner(tee)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scanner.Scan() {
		emit(scanner.Text())
	}
}

// maxCapturedBytes bounds the failure-dump buffer. A long-running child
// can emit megabytes of output before failing; we keep the tail since
// that is what diagnoses the failure.
const maxCapturedBytes = 1 << 20 // 1 MiB

// syncBuffer is a concurrency-safe bytes.Buffer for tee'd capture. The
// scanner goroutine writes; the main goroutine reads on failure dump.
// When the buffer exceeds maxCapturedBytes the oldest half is dropped.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func newSyncBuffer() *syncBuffer { return &syncBuffer{} }

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.buf.Write(p)
	if s.buf.Len() > maxCapturedBytes {
		tail := s.buf.Bytes()[s.buf.Len()-maxCapturedBytes/2:]
		keep := make([]byte, len(tail))
		copy(keep, tail)
		s.buf.Reset()
		s.buf.Write(keep)
	}
	return n, err
}

func (s *syncBuffer) Bytes() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]byte, s.buf.Len())
	copy(cp, s.buf.Bytes())
	return cp
}
