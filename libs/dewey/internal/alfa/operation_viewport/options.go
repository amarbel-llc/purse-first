package operation_viewport

import "context"

// defaultMaxLines is how many recent log lines stay visible under the
// spinner. Five mirrors tent_loader and fits comfortably above a shell
// prompt without pushing scrollback off screen.
const defaultMaxLines = 5

// Option configures a [Model] at construction.
type Option func(*Model)

// WithTitle sets the title rendered next to the spinner before any
// [OperationStarted] arrives, and used in [BatchDone] success/failure
// lines.
func WithTitle(title string) Option {
	return func(m *Model) { m.title = title }
}

// WithLines overrides the size of the rolling log tail.
func WithLines(n int) Option {
	return func(m *Model) {
		if n > 0 {
			m.maxLines = n
		}
	}
}

// WithTotal sets the expected operation count up front. The progress bar
// is hidden when Total <= 1.
func WithTotal(total int) Option {
	return func(m *Model) { m.opTotal = total }
}

// WithStyle replaces the default lipgloss styles.
func WithStyle(s Style) Option {
	return func(m *Model) { m.style = s }
}

// WithCancel wires Ctrl-C in the viewport to the given cancel func. The
// viewport waits for [BatchDone] before quitting so the child has a
// chance to render its final state.
func WithCancel(cancel context.CancelFunc) Option {
	return func(m *Model) { m.cancel = cancel }
}
