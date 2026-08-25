package mesa

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"

	"code.linenisgreat.com/purse-first/libs/dewey/internal/0/primordial"
)

var borderColor = lipgloss.AdaptiveColor{Light: "240", Dark: "245"}

type renderConfig struct {
	width      int
	forceStyle bool
	forcePlain bool
}

// RenderOpt configures a call to [Table.Render].
type RenderOpt func(*renderConfig)

// Width sets the target terminal width for styled rendering. Zero (the
// default) lets the renderer size to content.
func Width(n int) RenderOpt { return func(c *renderConfig) { c.width = n } }

// ForceStyle renders the styled table regardless of whether the writer is a
// terminal.
func ForceStyle() RenderOpt { return func(c *renderConfig) { c.forceStyle = true } }

// ForcePlain renders the plain TAB-separated form regardless of the writer.
func ForcePlain() RenderOpt { return func(c *renderConfig) { c.forcePlain = true } }

// Render writes the table to w. By default it renders styled when w is a
// terminal and plain otherwise (RFC 0003 §7.1); ForceStyle / ForcePlain
// override the detection.
func (t *Table) Render(w io.Writer, opts ...RenderOpt) error {
	if err := t.validate(); err != nil {
		return err
	}
	cfg := renderConfig{}
	for _, o := range opts {
		o(&cfg)
	}

	styled := cfg.forceStyle
	if !cfg.forceStyle && !cfg.forcePlain {
		if f, ok := w.(*os.File); ok {
			styled = primordial.IsTTY(f)
		}
	}
	if cfg.forcePlain {
		styled = false
	}

	if styled {
		return t.renderStyled(w, cfg)
	}
	return t.renderPlain(w)
}

// RenderStream decodes an NDJSON stream and renders it, the out-of-process
// path used by the `dewey table` CLI.
func RenderStream(r io.Reader, w io.Writer, opts ...RenderOpt) error {
	t, err := DecodeStream(r)
	if err != nil {
		return err
	}
	return t.Render(w, opts...)
}

func (t *Table) renderPlain(w io.Writer) error {
	bw := bufio.NewWriter(w)
	if len(t.Rows) == 0 {
		if t.EmptyText != "" {
			if _, err := fmt.Fprintln(bw, sanitize(t.EmptyText)); err != nil {
				return err
			}
		}
		return bw.Flush()
	}
	if err := writeTabRow(bw, t.columnNames()); err != nil {
		return err
	}
	for _, row := range t.Rows {
		cells := make([]string, len(row.Cells))
		for i, c := range row.Cells {
			cells[i] = sanitize(c.plain())
		}
		if err := writeTabRow(bw, cells); err != nil {
			return err
		}
	}
	return bw.Flush()
}

func writeTabRow(w io.Writer, cells []string) error {
	_, err := fmt.Fprintln(w, strings.Join(cells, "\t"))
	return err
}

func (t *Table) renderStyled(w io.Writer, cfg renderConfig) error {
	if len(t.Rows) == 0 {
		if t.EmptyText == "" {
			return nil
		}
		out := lipgloss.NewStyle().Foreground(mutedColor).Render(sanitize(t.EmptyText))
		_, err := fmt.Fprintln(w, out)
		return err
	}

	tbl := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(borderColor)).
		Headers(t.columnNames()...)

	for _, row := range t.Rows {
		cells := make([]string, len(row.Cells))
		for i, c := range row.Cells {
			cells[i] = t.renderCell(c)
		}
		tbl.Row(cells...)
	}

	columns := t.Columns
	tbl = tbl.StyleFunc(func(r, c int) lipgloss.Style {
		st := lipgloss.NewStyle().Padding(0, 1)
		if r == table.HeaderRow {
			return st.Bold(true)
		}
		if c >= 0 && c < len(columns) && columns[c].Align == Right {
			st = st.Align(lipgloss.Right)
		}
		return st
	})
	if cfg.width > 0 {
		tbl = tbl.Width(cfg.width)
	}

	if _, err := fmt.Fprintln(w, tbl.String()); err != nil {
		return err
	}
	if len(t.Legends) > 0 {
		return t.renderLegend(w)
	}
	return nil
}

func (t *Table) renderCell(c Cell) string {
	var sb strings.Builder
	for _, sp := range c.Spans {
		sb.WriteString(t.styleSpan(sp))
	}
	return sb.String()
}

func (t *Table) styleSpan(sp Span) string {
	st := lipgloss.NewStyle()
	if color := t.colorFor(sp.Sev); color != nil {
		st = st.Foreground(color)
	}
	if sp.Bold {
		st = st.Bold(true)
	}
	if sp.Italic {
		st = st.Italic(true)
	}
	if sp.Underline {
		st = st.Underline(true)
	}
	return st.Render(sanitize(sp.Text))
}

func (t *Table) colorFor(sev Severity) lipgloss.TerminalColor {
	if t.PaletteOverride != nil {
		if s, ok := t.PaletteOverride[sev]; ok {
			if c, ok := parseColor(s); ok {
				return c
			}
		}
	}
	return sev.defaultColor()
}

func (t *Table) renderLegend(w io.Writer) error {
	parts := make([]string, len(t.Legends))
	for i, e := range t.Legends {
		glyph := t.styleSpan(Span{Text: e.Glyph, Sev: e.Sev})
		parts[i] = glyph + " " + sanitize(e.Label)
	}
	_, err := fmt.Fprintln(w, strings.Join(parts, "  "))
	return err
}

// sanitize strips control characters so untrusted cell text cannot inject
// terminal escapes or break the grid (RFC 0003 Security Considerations).
func sanitize(s string) string {
	if !strings.ContainsFunc(s, isControl) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if isControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isControl(r rune) bool {
	return r < 0x20 || (r >= 0x7f && r <= 0x9f)
}
