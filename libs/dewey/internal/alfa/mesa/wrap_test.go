package mesa

import (
	"bytes"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string { return ansiPattern.ReplaceAllString(s, "") }

// belowGrid renders tbl styled with the given options and returns the
// ANSI-stripped lines beneath the grid's bottom border: the legend and footer.
func belowGrid(t *testing.T, tbl *Table, opts ...RenderOpt) []string {
	t.Helper()
	var buf bytes.Buffer
	if err := tbl.Render(&buf, append([]RenderOpt{ForceStyle()}, opts...)...); err != nil {
		t.Fatalf("render: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(stripANSI(buf.String()), "\n"), "\n")
	for i, line := range lines {
		if strings.Contains(line, "╰") {
			return lines[i+1:]
		}
	}
	t.Fatalf("styled render has no grid bottom border:\n%s", buf.String())
	return nil
}

func legendTable() *Table {
	return New().
		Col("ID", Pin).
		Row(Text("x")).
		Legend(
			Entry(OK, "●", "active"),
			Entry(Warn, "●", "running-detached"),
			Entry(Muted, "●", "inactive"),
		)
}

// The legend above is 8 + 2 + 18 + 2 + 10 = 40 columns on one line.

func TestRenderLegendWideStaysOnOneLine(t *testing.T) {
	got := belowGrid(t, legendTable(), Width(80))
	want := []string{"● active  ● running-detached  ● inactive"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("wide legend = %q, want %q", got, want)
	}
}

func TestRenderLegendExactWidthStaysOnOneLine(t *testing.T) {
	got := belowGrid(t, legendTable(), Width(40))
	want := []string{"● active  ● running-detached  ● inactive"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("exact-width legend = %q, want %q", got, want)
	}
}

func TestRenderLegendOneColumnNarrowBreaksAtEntryBoundary(t *testing.T) {
	got := belowGrid(t, legendTable(), Width(39))
	want := []string{"● active  ● running-detached", "● inactive"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("legend one column too narrow = %q, want %q", got, want)
	}
}

func TestRenderLegendNarrowOneEntryPerLine(t *testing.T) {
	got := belowGrid(t, legendTable(), Width(20))
	want := []string{"● active", "● running-detached", "● inactive"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("narrow legend = %q, want %q", got, want)
	}
}

func TestRenderLegendEntryWiderThanWidthWrapsWithinEntry(t *testing.T) {
	got := belowGrid(t, legendTable(), Width(10))
	want := []string{"● active", "●", "running-de", "tached", "● inactive"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("legend with an oversized entry = %q, want %q", got, want)
	}
}

func TestRenderLegendUnknownWidthKeepsOneLine(t *testing.T) {
	// No Width() and a non-terminal writer: the width is unknown (zero), so
	// the legend is never broken, however long.
	got := belowGrid(t, legendTable())
	want := []string{"● active  ● running-detached  ● inactive"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("unknown-width legend = %q, want %q", got, want)
	}
}

func TestRenderFittingLegendAndFooterAreByteIdenticalToUnknownWidth(t *testing.T) {
	// Wrapping must not perturb output that already fits: same bytes, escape
	// sequences included, as when the width is unknown.
	build := func() *Table {
		return legendTable().Footer(
			Text("self= this daemon's build"),
			Spans(Span{Text: "●", Sev: Warn}, Span{Text: " stale"}),
		)
	}
	var unknown, wide bytes.Buffer
	if err := build().Render(&unknown, ForceStyle()); err != nil {
		t.Fatalf("render: %v", err)
	}
	if err := build().Render(&wide, ForceStyle(), Width(120)); err != nil {
		t.Fatalf("render: %v", err)
	}
	if unknown.String() != wide.String() {
		t.Errorf("fitting render differs from unknown-width render:\n unknown %q\n wide    %q",
			unknown.String(), wide.String())
	}
}

func TestRenderFooterWrapsAtWordBoundaries(t *testing.T) {
	tbl := New().Col("ID", Pin).Row(Text("x")).Footer(Text("alpha bravo charlie delta"))
	got := belowGrid(t, tbl, Width(12))
	want := []string{"alpha bravo", "charlie", "delta"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("wrapped footer = %q, want %q", got, want)
	}
}

func TestRenderFooterWrapKeepsStyledSpans(t *testing.T) {
	line := Spans(Span{Text: "●", Sev: Warn}, Span{Text: " a line without self= is stale"})
	build := func() *Table { return New().Col("ID", Pin).Row(Text("x")).Footer(line) }

	var unwrapped, wrapped bytes.Buffer
	if err := build().Render(&unwrapped, ForceStyle()); err != nil {
		t.Fatalf("render: %v", err)
	}
	if err := build().Render(&wrapped, ForceStyle(), Width(16)); err != nil {
		t.Fatalf("render: %v", err)
	}

	plain := belowGrid(t, build(), Width(16))
	wantPlain := []string{"● a line without", "self= is stale"}
	if !reflect.DeepEqual(plain, wantPlain) {
		t.Fatalf("wrapped footer text = %q, want %q", plain, wantPlain)
	}

	// The warn-colored glyph keeps the escape it has when nothing wraps.
	glyphPrefix := func(out string) string { return out[:strings.Index(out, "●")] }
	if glyphPrefix(wrapped.String()) != glyphPrefix(unwrapped.String()) {
		t.Errorf("wrapped glyph lost its styling:\n wrapped   %q\n unwrapped %q",
			wrapped.String(), unwrapped.String())
	}
	// The neutral continuation is drawn Muted, so the second line is styled
	// too rather than falling back to bare text.
	rawLines := strings.Split(strings.TrimSuffix(wrapped.String(), "\n"), "\n")
	last := rawLines[len(rawLines)-1]
	if !strings.Contains(last, "\x1b[") || stripANSI(last) != "self= is stale" {
		t.Errorf("continuation line lost its muted styling: %q", last)
	}
}

func TestRenderFooterBlankLineSurvivesWrapping(t *testing.T) {
	tbl := New().Col("ID", Pin).Row(Text("x")).Footer(Text("first"), Spans(), Text("second"))
	got := belowGrid(t, tbl, Width(8))
	want := []string{"first", "", "second"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("footer with a blank separator = %q, want %q", got, want)
	}
}

func TestRenderFooterWrapSanitizesControlChars(t *testing.T) {
	tbl := New().Col("ID", Pin).Row(Text("x")).Footer(Text("aaa\x1b[31m bbb ccc"))
	got := belowGrid(t, tbl, Width(8))
	// sanitize drops only the ESC byte, leaving "[31m" as visible text. Had
	// the ESC leaked, stripANSI (used by belowGrid) would have eaten the whole
	// sequence and "aaa[31m" could not appear.
	want := []string{"aaa[31m", "bbb ccc"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("wrapped footer = %q, want %q", got, want)
	}
}

func TestRenderPlainIgnoresWidthForLegendAndFooter(t *testing.T) {
	// RFC 0003 §6.1/§7.3: the legend is suppressed on a pipe and footer lines
	// pass through verbatim, whatever the width.
	long := "alpha bravo charlie delta echo foxtrot golf hotel"
	build := func() *Table { return legendTable().Footer(Text(long)) }

	var narrow, unset bytes.Buffer
	if err := build().Render(&narrow, ForcePlain(), Width(10)); err != nil {
		t.Fatalf("render: %v", err)
	}
	if err := build().Render(&unset, ForcePlain()); err != nil {
		t.Fatalf("render: %v", err)
	}
	if narrow.String() != unset.String() {
		t.Errorf("plain output changed with width:\n narrow %q\n unset  %q", narrow.String(), unset.String())
	}
	if want := "ID\nx\n" + long + "\n"; narrow.String() != want {
		t.Errorf("plain output = %q, want %q", narrow.String(), want)
	}
}

func TestWrapSpansSplitsWordWiderThanWidth(t *testing.T) {
	lines := wrapSpans([]Span{{Text: "abcdefgh"}}, 3)
	var got []string
	for _, l := range lines {
		got = append(got, spansText(l))
	}
	want := []string{"abc", "def", "gh"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("hard split = %q, want %q", got, want)
	}
}

func TestWrapSpansMakesProgressOnWideRuneInNarrowWidth(t *testing.T) {
	// A double-width rune cannot fit in one column; it must still be placed
	// (on its own line) rather than looping forever.
	lines := wrapSpans([]Span{{Text: "漢字"}}, 1)
	var got []string
	for _, l := range lines {
		got = append(got, spansText(l))
	}
	want := []string{"漢", "字"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("wide runes at width 1 = %q, want %q", got, want)
	}
}

func TestWrapSpansKeepsLeadingIndentOnFirstLineOnly(t *testing.T) {
	lines := wrapSpans([]Span{{Text: "  alpha beta gamma"}}, 10)
	var got []string
	for _, l := range lines {
		got = append(got, spansText(l))
	}
	want := []string{"  alpha", "beta gamma"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("indented wrap = %q, want %q", got, want)
	}
}

func TestWrapSpansWordSpanningTwoSpansStaysTogether(t *testing.T) {
	// "ab" + "cd" with no space between them is one word, so it moves to the
	// next line as a unit instead of being split between its spans.
	lines := wrapSpans([]Span{{Text: "xx ab"}, {Text: "cd", Sev: Warn}}, 5)
	var got []string
	for _, l := range lines {
		got = append(got, spansText(l))
	}
	want := []string{"xx", "abcd"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("cross-span word = %q, want %q", got, want)
	}
	if last := lines[1]; len(last) != 2 || last[1].Sev != Warn {
		t.Errorf("cross-span word lost its second span's styling: %+v", last)
	}
}

func TestWrapSpansUnknownWidthReturnsInputUntouched(t *testing.T) {
	in := []Span{{Text: "alpha bravo"}, {Text: " charlie", Sev: Warn}}
	lines := wrapSpans(in, 0)
	if len(lines) != 1 || !reflect.DeepEqual(lines[0], in) {
		t.Errorf("width 0 = %+v, want the input as one line", lines)
	}
}

func spansText(spans []Span) string {
	var sb strings.Builder
	for _, sp := range spans {
		sb.WriteString(sp.Text)
	}
	return sb.String()
}
