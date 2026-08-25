package mesa

import (
	"sort"

	"github.com/mattn/go-runewidth"
)

// defaultMinFlexWidth is the floor a Flex column shrinks to when it declares
// no MinWidth (RFC 0003 §7.2; the tuning lever in FDR 0015).
const defaultMinFlexWidth = 8

// displayWidth is the wcwidth-aware column width of already-sanitized text.
func displayWidth(s string) int { return runewidth.StringWidth(s) }

// naturalWidths returns each column's content width — the wider of its
// header and its widest cell — measured wcwidth-aware on sanitized text.
func (t *Table) naturalWidths() []int {
	w := make([]int, len(t.Columns))
	for c, col := range t.Columns {
		w[c] = displayWidth(sanitize(col.Name))
	}
	for _, row := range t.Rows {
		for c, cell := range row.Cells {
			if cw := displayWidth(sanitize(cell.plain())); cw > w[c] {
				w[c] = cw
			}
		}
	}
	return w
}

// targetWidths resolves the per-column render widths for a total available
// width (avail <= 0 means unconstrained, so columns keep their content
// width). When the natural layout overflows the budget, Flex columns shrink
// in ascending ShrinkPriority order down to their floor; Pin columns never
// shrink. Best-effort: if every Flex column is already at its floor and the
// content still overflows, the remaining deficit is left as-is.
func (t *Table) targetWidths(natural []int, avail int) []int {
	target := append([]int(nil), natural...)
	if avail <= 0 {
		return target
	}
	ncols := len(t.Columns)
	// Rounded border + Padding(0,1): 2 padding columns per cell plus
	// ncols+1 vertical border cells.
	overhead := 3*ncols + 1
	budget := avail - overhead
	if budget <= 0 {
		return target
	}
	deficit := sum(natural) - budget
	if deficit <= 0 {
		return target
	}
	for _, c := range t.flexByShrinkPriority() {
		if deficit <= 0 {
			break
		}
		floor := effectiveMinWidth(t.Columns[c], natural[c])
		room := target[c] - floor
		if room <= 0 {
			continue
		}
		take := room
		if take > deficit {
			take = deficit
		}
		target[c] -= take
		deficit -= take
	}
	return target
}

func effectiveMinWidth(col Column, natural int) int {
	m := col.MinWidth
	if m <= 0 {
		m = defaultMinFlexWidth
	}
	if m > natural {
		m = natural
	}
	return m
}

// flexByShrinkPriority returns the indices of Flex columns ordered by
// ascending ShrinkPriority (ties keep declared order).
func (t *Table) flexByShrinkPriority() []int {
	var idx []int
	for c, col := range t.Columns {
		if col.Role == Flex {
			idx = append(idx, c)
		}
	}
	sort.SliceStable(idx, func(a, b int) bool {
		return t.Columns[idx[a]].ShrinkPriority < t.Columns[idx[b]].ShrinkPriority
	})
	return idx
}

func sum(xs []int) int {
	total := 0
	for _, x := range xs {
		total += x
	}
	return total
}

// truncateSpansToWidth truncates a cell's spans to at most maxWidth display
// columns, appending a dim ellipsis when content is dropped. The styling of
// surviving spans (and of the partially-cut span) is preserved.
func truncateSpansToWidth(spans []Span, maxWidth int) []Span {
	if maxWidth <= 0 {
		return nil
	}
	total := 0
	for _, sp := range spans {
		total += displayWidth(sanitize(sp.Text))
	}
	if total <= maxWidth {
		return spans
	}
	budget := maxWidth - 1 // reserve one column for the ellipsis
	out := make([]Span, 0, len(spans)+1)
	used := 0
	for _, sp := range spans {
		clean := sanitize(sp.Text)
		w := displayWidth(clean)
		if used+w <= budget {
			out = append(out, sp)
			used += w
			continue
		}
		if remaining := budget - used; remaining > 0 {
			trimmed := sp
			trimmed.Text = runewidth.Truncate(clean, remaining, "")
			out = append(out, trimmed)
		}
		break
	}
	return append(out, Span{Text: "…", Sev: Muted})
}

// truncateHeader truncates a header label to width, ellipsizing on overflow.
func truncateHeader(name string, width int) string {
	clean := sanitize(name)
	if displayWidth(clean) <= width {
		return clean
	}
	return runewidth.Truncate(clean, width, "…")
}
