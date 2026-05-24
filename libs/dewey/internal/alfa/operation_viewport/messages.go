// Package operation_viewport renders a bubbletea spinner + rolling log
// tail above a child process, with an optional progress bar driven by a
// batch index. See FDR 0010 in this repo for the design rationale.
//
// Two entry points:
//
//   - [Run] runs a single [exec.Cmd] under the viewport, returning its
//     error. PTY-allocated; SGR colors survive.
//   - [RunBatch] drives many caller-supplied operations through the same
//     viewport, advancing a progress bar by [OperationStarted] events.
//
// Callers with a bespoke event source (e.g. a TAP parser) can construct
// the underlying [Model] directly and send messages on the
// [tea.Program] themselves.
//
// On a non-TTY stdout, [Run] streams the child's combined output to
// stderr unchanged.
package operation_viewport

// LogLine carries a single line of child output into the viewport.
type LogLine struct {
	Text string
}

// OperationStarted signals the start of an operation within a batch. For
// single-op callers, send once with Index=1 Total=1 — the progress bar
// hides itself when Total <= 1.
type OperationStarted struct {
	Name  string
	Index int
	Total int
}

// OperationProgress reports within-op progress. The v0 layout does not
// render a secondary bar for this; it is part of the message protocol so
// callers can begin emitting it without a breaking change later.
type OperationProgress struct {
	Current int
	Total   int
}

// OperationDone signals that the current operation finished. A non-nil
// Err on an individual operation does not by itself terminate the
// viewport — that is the caller's choice via the [Emitter] return value
// or [BatchDone].
type OperationDone struct {
	Err error
}

// BatchDone signals that the viewport should render its terminal state
// and exit. A non-nil Err causes the captured transcript to be dumped to
// stderr after the program returns.
type BatchDone struct {
	Err error
}
