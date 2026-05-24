package operation_viewport

import (
	"bytes"
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// Model is the bubbletea model that paints the viewport. Callers
// implementing custom event sources construct it with [NewModel] and
// send the message types from messages.go to drive it.
type Model struct {
	title    string
	maxLines int
	style    Style

	spinner  spinner.Model
	progress progress.Model

	lines []string

	opName  string
	opIndex int
	opTotal int

	batchDone bool
	batchErr  error

	cancel context.CancelFunc
}

// NewModel constructs a [Model] applying the given options. The default
// tail size is 5 lines.
func NewModel(opts ...Option) Model {
	m := Model{
		maxLines: defaultMaxLines,
		style:    DefaultStyle(),
	}
	for _, opt := range opts {
		opt(&m)
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = m.style.Spinner
	m.spinner = sp

	m.progress = progress.New(progress.WithDefaultGradient())

	return m
}

// Init starts the spinner.
func (m Model) Init() tea.Cmd { return m.spinner.Tick }

// Update implements [tea.Model]. It dispatches on the viewport message
// types plus bubbletea's own KeyMsg / spinner.TickMsg.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			if m.cancel != nil {
				m.cancel()
			}
			return m, nil
		}
		return m, nil

	case OperationStarted:
		m.opName = msg.Name
		m.opIndex = msg.Index
		if msg.Total > 0 {
			m.opTotal = msg.Total
		}
		m.lines = m.lines[:0]
		return m, nil

	case LogLine:
		m.lines = append(m.lines, msg.Text)
		if len(m.lines) > m.maxLines {
			m.lines = m.lines[len(m.lines)-m.maxLines:]
		}
		return m, nil

	case OperationProgress, OperationDone:
		// Part of the v0 message protocol but the layout does not
		// render them. See FDR 0010 "Out of scope" notes.
		return m, nil

	case BatchDone:
		m.batchDone = true
		m.batchErr = msg.Err
		return m, tea.Quit

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

// View renders the spinner header plus the rolling tail, or the
// terminal state on [BatchDone].
func (m Model) View() string {
	if m.batchDone {
		label := m.headerLabel()
		if m.batchErr != nil {
			return m.style.Failure.Render("✗ "+label+" — failed") + "\n"
		}
		return m.style.Success.Render("✓ "+label) + "\n"
	}

	var b bytes.Buffer
	fmt.Fprintf(&b, "%s %s", m.spinner.View(), m.headerLabel())
	if m.opTotal > 1 {
		fmt.Fprintf(&b, "  %s %d/%d",
			m.progress.ViewAs(float64(m.opIndex)/float64(m.opTotal)),
			m.opIndex, m.opTotal,
		)
	}
	b.WriteByte('\n')
	for _, line := range m.lines {
		fmt.Fprintln(&b, m.style.Tail.Render("│ "+line))
	}
	return b.String()
}

// headerLabel returns the per-frame label rendered next to the spinner
// (or in the terminal success/failure line). Operation name wins when
// set; the construction-time title is the fallback for the pre-first-
// op moment and for single-op callers that never send an
// [OperationStarted].
func (m Model) headerLabel() string {
	if m.opName != "" {
		return m.opName
	}
	return m.title
}
