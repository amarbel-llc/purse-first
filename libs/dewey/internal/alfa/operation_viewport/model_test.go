package operation_viewport

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// drive applies a sequence of messages to a Model and returns the
// resulting Model. Cmds are discarded — the tail/progress/done state
// lives entirely on the model fields.
func drive(m Model, msgs ...tea.Msg) Model {
	var next tea.Model = m
	for _, msg := range msgs {
		next, _ = next.Update(msg)
	}
	return next.(Model)
}

func TestModel_TailRingTruncates(t *testing.T) {
	m := NewModel(WithTitle("t"), WithLines(3))
	m = drive(m,
		LogLine{Text: "a"},
		LogLine{Text: "b"},
		LogLine{Text: "c"},
		LogLine{Text: "d"},
	)
	got := m.lines
	want := []string{"b", "c", "d"}
	if len(got) != len(want) {
		t.Fatalf("tail len: got %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("tail[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestModel_OperationStartedResetsTail(t *testing.T) {
	m := NewModel()
	m = drive(m,
		LogLine{Text: "prev1"},
		LogLine{Text: "prev2"},
		OperationStarted{Name: "next", Index: 2, Total: 5},
	)
	if len(m.lines) != 0 {
		t.Errorf("expected tail reset after OperationStarted, got %v", m.lines)
	}
	if m.opName != "next" || m.opIndex != 2 || m.opTotal != 5 {
		t.Errorf("op fields: got name=%q index=%d total=%d", m.opName, m.opIndex, m.opTotal)
	}
}

func TestModel_OperationStartedTotalZeroPreservesPrior(t *testing.T) {
	m := NewModel(WithTotal(10))
	m = drive(m, OperationStarted{Name: "x", Index: 1})
	if m.opTotal != 10 {
		t.Errorf("Total=0 should not overwrite prior opTotal: got %d, want 10", m.opTotal)
	}
}

func TestModel_BatchDoneSuccessView(t *testing.T) {
	m := NewModel(WithTitle("loading"))
	m = drive(m, BatchDone{Err: nil})
	view := m.View()
	if !strings.Contains(view, "✓") || !strings.Contains(view, "loading") {
		t.Errorf("success view missing marker or title:\n%s", view)
	}
}

func TestModel_BatchDoneFailureView(t *testing.T) {
	m := NewModel(WithTitle("loading"))
	m = drive(m, BatchDone{Err: errors.New("boom")})
	view := m.View()
	if !strings.Contains(view, "✗") || !strings.Contains(view, "failed") {
		t.Errorf("failure view missing marker or 'failed':\n%s", view)
	}
}

func TestModel_HeaderLabelPrefersOpName(t *testing.T) {
	m := NewModel(WithTitle("batch"))
	m = drive(m, OperationStarted{Name: "current-op", Index: 1, Total: 3})
	if got := m.headerLabel(); got != "current-op" {
		t.Errorf("headerLabel: got %q, want current-op (op name wins over title)", got)
	}
}

func TestModel_HeaderLabelFallsBackToTitle(t *testing.T) {
	m := NewModel(WithTitle("batch"))
	if got := m.headerLabel(); got != "batch" {
		t.Errorf("headerLabel: got %q, want batch (no op started yet)", got)
	}
}

func TestModel_ProgressHiddenWhenTotalLEOne(t *testing.T) {
	m := NewModel(WithTitle("solo"), WithTotal(1))
	m = drive(m, OperationStarted{Name: "solo", Index: 1, Total: 1})
	view := m.View()
	if strings.Contains(view, "1/1") {
		t.Errorf("progress bar should be hidden when total<=1, got view:\n%s", view)
	}
}

func TestModel_ProgressShownWhenTotalGTOne(t *testing.T) {
	m := NewModel(WithTotal(4))
	m = drive(m, OperationStarted{Name: "op", Index: 2, Total: 4})
	view := m.View()
	if !strings.Contains(view, "2/4") {
		t.Errorf("progress bar should show 2/4, got view:\n%s", view)
	}
}

func TestModel_CtrlCInvokesCancel(t *testing.T) {
	called := 0
	m := NewModel(WithCancel(func() { called++ }))
	_ = drive(m, tea.KeyMsg{Type: tea.KeyCtrlC})
	if called != 1 {
		t.Errorf("cancel call count: got %d, want 1", called)
	}
}
