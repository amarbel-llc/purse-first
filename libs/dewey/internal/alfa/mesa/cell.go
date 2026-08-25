package mesa

import "strings"

// Span is a run of text at one severity with optional emphasis. It is the
// atom of styling: the renderer colors it by Sev and owns the palette.
type Span struct {
	Text      string
	Sev       Severity
	Bold      bool
	Italic    bool
	Underline bool
}

// Cell is one column's content in a row: a sequence of styled spans, so a
// single column can carry a primary value plus a trailing dim annotation.
type Cell struct {
	Spans []Span
}

// Row is a rendered line: one cell per declared column.
type Row struct {
	Cells []Cell
}

// Text is a cell of one neutral span.
func Text(s string) Cell {
	return Cell{Spans: []Span{{Text: s}}}
}

// Styled is a cell of one span at the given severity.
func Styled(sev Severity, s string) Cell {
	return Cell{Spans: []Span{{Text: s, Sev: sev}}}
}

// Spans is a cell built from explicit spans.
func Spans(spans ...Span) Cell {
	return Cell{Spans: spans}
}

type statusConfig struct {
	glyph  string
	badge  string
	marker string
}

// StatusOpt configures a [Status] cell.
type StatusOpt func(*statusConfig)

// WithGlyph overrides the status glyph (default "●").
func WithGlyph(g string) StatusOpt { return func(c *statusConfig) { c.glyph = g } }

// WithBadge appends a badge after the glyph (e.g. a repeated presence icon).
func WithBadge(b string) StatusOpt { return func(c *statusConfig) { c.badge = b } }

// WithMarker appends a dim marker after the label (e.g. "(current)").
func WithMarker(m string) StatusOpt { return func(c *statusConfig) { c.marker = m } }

// Status builds a composite status cell from ordinary spans: a glyph colored
// by sev, an optional badge, the label, and an optional dim marker. The
// result is just spans, so the wire keeps a single cell shape.
func Status(sev Severity, label string, opts ...StatusOpt) Cell {
	cfg := statusConfig{glyph: "●"}
	for _, o := range opts {
		o(&cfg)
	}
	var b spanBuilder
	b.push(cfg.glyph, sev)
	b.push(cfg.badge, Neutral)
	b.push(label, Neutral)
	b.push(cfg.marker, Muted)
	return Cell{Spans: b.spans}
}

// spanBuilder assembles spans separated by a single neutral space.
type spanBuilder struct {
	spans []Span
}

func (b *spanBuilder) push(text string, sev Severity) {
	if text == "" {
		return
	}
	if len(b.spans) > 0 {
		b.spans = append(b.spans, Span{Text: " "})
	}
	b.spans = append(b.spans, Span{Text: text, Sev: sev})
}

// plain returns the cell's text with no styling.
func (c Cell) plain() string {
	if len(c.Spans) == 1 {
		return c.Spans[0].Text
	}
	var sb strings.Builder
	for _, sp := range c.Spans {
		sb.WriteString(sp.Text)
	}
	return sb.String()
}

// isPlain reports whether the cell carries no styling, so it may be encoded
// as a bare JSON string on the wire.
func (c Cell) isPlain() bool {
	for _, sp := range c.Spans {
		if sp.Sev != Neutral || sp.Bold || sp.Italic || sp.Underline {
			return false
		}
	}
	return true
}
