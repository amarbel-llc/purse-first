package operation_viewport

import (
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tonistiigi/vt100"
)

// syncWriter is a concurrency-safe io.Writer used to capture bubbletea
// render output mid-flight. Modeled on amarbel-llc/crap's harness.
type syncWriter struct {
	mu sync.Mutex
	b  strings.Builder
}

func (w *syncWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

func (w *syncWriter) Bytes() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	return []byte(w.b.String())
}

// renderScreen feeds raw bubbletea output through a 120x40 VT100
// emulator and returns the visible lines (trailing blanks trimmed) and
// the full Format grid for color assertions.
func renderScreen(raw []byte) ([]string, [][]vt100.Format) {
	term := vt100.NewVT100(40, 120)
	_, _ = term.Write(raw)

	var lines []string
	for _, row := range term.Content {
		lines = append(lines, strings.TrimRight(string(row), " "))
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines, term.Format
}

func findLineContaining(lines []string, substr string) int {
	for i, line := range lines {
		if strings.Contains(line, substr) {
			return i
		}
	}
	return -1
}

// runProgramWithMessages constructs a tea.Program against the given
// model, captures its output into a syncWriter, feeds the supplied
// messages one by one, then sends BatchDone and waits for exit. The
// captured raw output is returned for VT100 rendering.
func runProgramWithMessages(t *testing.T, m Model, msgs []tea.Msg, batchErr error) []byte {
	t.Helper()

	out := &syncWriter{}
	prog := tea.NewProgram(
		m,
		tea.WithOutput(out),
		tea.WithInput(io.NopCloser(strings.NewReader(""))),
		tea.WithoutSignalHandler(),
	)

	done := make(chan error, 1)
	go func() {
		_, err := prog.Run()
		done <- err
	}()

	// bubbletea's tea.Program serialises sent messages through a FIFO
	// channel processed after Init. Queue all updates before BatchDone;
	// the program will drain them in order and the final terminal
	// state is rendered before Run returns.
	for _, msg := range msgs {
		prog.Send(msg)
	}
	prog.Send(BatchDone{Err: batchErr})

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("tea.Program.Run: %v", err)
		}
	case <-time.After(2 * time.Second):
		prog.Kill()
		t.Fatal("tea.Program.Run did not exit within 2s")
	}

	return out.Bytes()
}

func TestVT_SuccessTerminalState(t *testing.T) {
	m := NewModel(WithTitle("loading tent"))
	raw := runProgramWithMessages(
		t, m,
		[]tea.Msg{
			OperationStarted{Name: "loading tent", Index: 1, Total: 1},
			LogLine{Text: "blob 1"},
			LogLine{Text: "blob 2"},
		},
		nil,
	)
	lines, _ := renderScreen(raw)
	if findLineContaining(lines, "✓ loading tent") < 0 {
		t.Errorf("expected success line containing '✓ loading tent', got lines:\n%s",
			strings.Join(lines, "\n"))
	}
}

func TestVT_FailureTerminalState(t *testing.T) {
	m := NewModel(WithTitle("loading tent"))
	raw := runProgramWithMessages(
		t, m,
		[]tea.Msg{
			OperationStarted{Name: "loading tent", Index: 1, Total: 1},
			LogLine{Text: "bad data"},
		},
		io.ErrUnexpectedEOF,
	)
	lines, _ := renderScreen(raw)
	if findLineContaining(lines, "✗ loading tent — failed") < 0 {
		t.Errorf("expected failure line, got lines:\n%s", strings.Join(lines, "\n"))
	}
}

// Color-rendering assertions belong on a real-PTY harness: when the
// program writes into a plain io.Writer, bubbletea's colorprofile
// detection strips SGR codes. The FDR-promotion criteria (a shipping
// caller of Run) will exercise the colored path end-to-end.
