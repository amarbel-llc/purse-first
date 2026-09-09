package mesa

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderPlain(t *testing.T) {
	tbl := New().
		Col("ID", Pin).
		Col("AGE", Pin).
		Row(Text("api"), Text("2m")).
		Row(Text("web"), Text("5m"))

	var buf bytes.Buffer
	if err := tbl.Render(&buf, ForcePlain()); err != nil {
		t.Fatalf("render: %v", err)
	}
	want := "ID\tAGE\napi\t2m\nweb\t5m\n"
	if buf.String() != want {
		t.Errorf("plain render = %q, want %q", buf.String(), want)
	}
}

func TestRenderPlainEmpty(t *testing.T) {
	tbl := New().Col("ID", Pin).Empty("no sessions")

	var buf bytes.Buffer
	if err := tbl.Render(&buf, ForcePlain()); err != nil {
		t.Fatalf("render: %v", err)
	}
	if buf.String() != "no sessions\n" {
		t.Errorf("empty render = %q, want %q", buf.String(), "no sessions\n")
	}
}

func TestRenderStyledEmptyHonorsPaletteOverride(t *testing.T) {
	// RFC 0003 §7.4 styles the empty text `muted`; §5 lets `palette`
	// override any severity. The override therefore has to reach the empty
	// text, as it does rows, the legend, and footer prose (purse-first#188).
	render := func(palette map[Severity]string) string {
		tbl := New().Col("ID", Pin).Empty("no sessions")
		if palette != nil {
			tbl = tbl.Palette(palette)
		}
		var buf bytes.Buffer
		if err := tbl.Render(&buf, ForceStyle()); err != nil {
			t.Fatalf("render: %v", err)
		}
		return buf.String()
	}

	plain := render(nil)
	overridden := render(map[Severity]string{Muted: "#ff0000"})

	if !strings.Contains(overridden, "no sessions") {
		t.Errorf("empty text lost under a palette override: %q", overridden)
	}
	if overridden == plain {
		t.Errorf("palette override for muted did not reach the empty text: %q", overridden)
	}
}

func TestRenderStyledEmptySanitizesControlChars(t *testing.T) {
	// The empty text now routes through styleSpan; make sure that kept the
	// sanitization the old direct-Foreground path did.
	tbl := New().Col("ID", Pin).Empty("no\x1b[31m sessions")

	var buf bytes.Buffer
	if err := tbl.Render(&buf, ForcePlain()); err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(buf.String(), "\x1b") {
		t.Errorf("empty text leaked an escape sequence: %q", buf.String())
	}
}

func TestRenderPlainFooter(t *testing.T) {
	tbl := New().
		Col("ID", Pin).
		Row(Text("api")).
		Footer(
			Text("self= this daemon's build"),
			Spans(Span{Text: "●", Sev: Warn}, Span{Text: " stale"}),
		)

	var buf bytes.Buffer
	if err := tbl.Render(&buf, ForcePlain()); err != nil {
		t.Fatalf("render: %v", err)
	}
	// Footer lines follow the rows verbatim, one per line, with no styling
	// and no TAB — the same treatment the empty-state text gets on a pipe.
	want := "ID\napi\nself= this daemon's build\n● stale\n"
	if buf.String() != want {
		t.Errorf("plain footer render = %q, want %q", buf.String(), want)
	}
}

func TestRenderPlainFooterBlankLineSeparatesParagraphs(t *testing.T) {
	tbl := New().
		Col("ID", Pin).
		Row(Text("api")).
		Footer(Text("first"), Spans(), Text("second"))

	var buf bytes.Buffer
	if err := tbl.Render(&buf, ForcePlain()); err != nil {
		t.Fatalf("render: %v", err)
	}
	want := "ID\napi\nfirst\n\nsecond\n"
	if buf.String() != want {
		t.Errorf("blank footer line = %q, want %q", buf.String(), want)
	}
}

func TestRenderPlainEmptySuppressesFooter(t *testing.T) {
	tbl := New().Col("ID", Pin).Empty("no sessions").Footer(Text("a key"))

	var buf bytes.Buffer
	if err := tbl.Render(&buf, ForcePlain()); err != nil {
		t.Fatalf("render: %v", err)
	}
	// RFC 0003 §7.4: an empty table renders the empty text and nothing
	// beyond it — there are no glyphs left for a key to explain.
	if buf.String() != "no sessions\n" {
		t.Errorf("empty+footer render = %q, want %q", buf.String(), "no sessions\n")
	}
}

func TestRenderStyledFooterFollowsLegend(t *testing.T) {
	tbl := New().
		Col("ID", Pin).
		Legend(Entry(OK, "●", "attached")).
		Footer(Text("self= this daemon's build")).
		Row(Text("api"))

	var buf bytes.Buffer
	if err := tbl.Render(&buf, ForceStyle()); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	legendAt := strings.Index(out, "attached")
	footerAt := strings.Index(out, "self=")
	if legendAt < 0 || footerAt < 0 {
		t.Fatalf("styled render missing legend or footer:\n%s", out)
	}
	if footerAt < legendAt {
		t.Errorf("footer rendered above the legend; want legend first:\n%s", out)
	}
	if gridAt := strings.Index(out, "╰"); gridAt < 0 || footerAt < gridAt {
		t.Errorf("footer rendered above the grid:\n%s", out)
	}
}

// renderFooterLine renders a one-span footer at sev and returns just the
// footer line, so a test can compare how two severities are drawn without
// pinning the renderer's exact SGR codes.
func renderFooterLine(t *testing.T, sev Severity) string {
	t.Helper()
	tbl := New().
		Col("ID", Pin).
		Footer(Spans(Span{Text: "marker", Sev: sev})).
		Row(Text("api"))

	var buf bytes.Buffer
	if err := tbl.Render(&buf, ForceStyle()); err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.Contains(line, "marker") {
			return line
		}
	}
	t.Fatalf("styled render has no footer line:\n%s", buf.String())
	return ""
}

func TestRenderStyledFooterDimsNeutralSpans(t *testing.T) {
	// The footer carries a single neutral span, so the only thing that can
	// put an escape on this line is the Neutral -> Muted substitution: drop
	// that rule and colorFor(Neutral) yields no color and no escape at all.
	neutral := renderFooterLine(t, Neutral)
	if !strings.Contains(neutral, "\x1b[") {
		t.Errorf("neutral footer span not dimmed (no ANSI): %q", neutral)
	}
	if got := renderFooterLine(t, Muted); got != neutral {
		t.Errorf("neutral footer span not drawn as Muted:\n neutral %q\n muted   %q", neutral, got)
	}
}

func TestRenderStyledFooterKeepsExplicitSeverity(t *testing.T) {
	// Same text, different severity: an explicitly styled span must not be
	// flattened into the muted default the neutral rule applies.
	neutral := renderFooterLine(t, Neutral)
	warn := renderFooterLine(t, Warn)
	if !strings.Contains(warn, "marker") {
		t.Errorf("footer lost its styled text: %q", warn)
	}
	if warn == neutral {
		t.Errorf("warn footer span rendered identically to neutral: %q", warn)
	}
}

func TestRenderStyledFooterIgnoresTableWidth(t *testing.T) {
	// RFC 0003 §6.1: footer text MUST NOT be wrapped, truncated, or
	// reflowed to the table width. The footer here is far wider than the
	// 30-column target, and the single pin column means nothing else can
	// ellipsize, so any "…" or line break would be the footer's.
	long := "self= this daemon's build · remote= the endpoint's build · no self= means stale"
	tbl := New().Col("ID", Pin).Footer(Text(long)).Row(Text("x"))

	var buf bytes.Buffer
	if err := tbl.Render(&buf, ForceStyle(), Width(30)); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "…") {
		t.Errorf("footer was ellipsized at the table width:\n%s", out)
	}
	// A single neutral span renders as one contiguous run between one
	// escape pair, so the whole text must sit on one line.
	var found bool
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, long) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("footer was wrapped or reflowed to the table width:\n%s", out)
	}
}

func TestRenderFooterSanitizesControlChars(t *testing.T) {
	tbl := New().Col("A", Pin).Row(Text("a")).Footer(Text("k\x1b[31mey"))

	var buf bytes.Buffer
	if err := tbl.Render(&buf, ForcePlain()); err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(buf.String(), "\x1b") {
		t.Errorf("footer leaked an escape sequence: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "k[31mey") {
		t.Errorf("footer text mangled beyond the stripped ESC: %q", buf.String())
	}
}

func TestRenderPlainSanitizesControlChars(t *testing.T) {
	tbl := New().Col("A", Pin).Row(Text("a\tb\x1bc"))

	var buf bytes.Buffer
	if err := tbl.Render(&buf, ForcePlain()); err != nil {
		t.Fatalf("render: %v", err)
	}
	// The embedded TAB and ESC must be stripped so they cannot break the
	// grid or inject an escape sequence.
	if buf.String() != "A\nabc\n" {
		t.Errorf("sanitized render = %q, want %q", buf.String(), "A\nabc\n")
	}
}

func TestRenderStyledHasBorderAndContent(t *testing.T) {
	tbl := New().
		Col("ID", Pin).
		Col("AGE", Pin).
		Row(Text("api"), Text("2m"))

	var buf bytes.Buffer
	if err := tbl.Render(&buf, ForceStyle()); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	// Rounded border corner and content are present regardless of the
	// terminal color profile.
	for _, want := range []string{"╭", "ID", "AGE", "api", "2m"} {
		if !strings.Contains(out, want) {
			t.Errorf("styled render missing %q:\n%s", want, out)
		}
	}
}

func TestRenderStyledForcedEmitsANSI(t *testing.T) {
	tbl := New().
		Col("ID", Pin).
		Col("STATUS", Pin).
		Row(Text("api"), Status(OK, "attached"))

	var buf bytes.Buffer
	if err := tbl.Render(&buf, ForceStyle()); err != nil {
		t.Fatalf("render: %v", err)
	}
	// Forcing style over a non-terminal buffer must still emit ANSI (the
	// bold header alone guarantees an escape), otherwise `--force-style`
	// through a pipe would silently produce plain text.
	if !strings.Contains(buf.String(), "\x1b[") {
		t.Errorf("forced-style output has no ANSI escape:\n%q", buf.String())
	}
}

func TestRenderStyledWrapVsTruncate(t *testing.T) {
	long := "alpha bravo charlie delta echo foxtrot golf hotel india juliet"

	wrapT := New().Col("ID", Pin).Col("DESC", Flex, Wrap()).Row(Text("x"), Text(long))
	var wbuf bytes.Buffer
	if err := wrapT.Render(&wbuf, ForceStyle(), Width(30)); err != nil {
		t.Fatalf("wrap render: %v", err)
	}
	wout := wbuf.String()
	if !strings.Contains(wout, "juliet") {
		t.Errorf("wrap dropped the tail (should keep full text):\n%s", wout)
	}
	if strings.Contains(wout, "…") {
		t.Errorf("wrap should not ellipsize:\n%s", wout)
	}

	truncT := New().Col("ID", Pin).Col("DESC", Flex).Row(Text("x"), Text(long))
	var tbuf bytes.Buffer
	if err := truncT.Render(&tbuf, ForceStyle(), Width(30)); err != nil {
		t.Fatalf("truncate render: %v", err)
	}
	tout := tbuf.String()
	if !strings.Contains(tout, "…") {
		t.Errorf("truncate should ellipsize:\n%s", tout)
	}
	if strings.Count(wout, "\n") <= strings.Count(tout, "\n") {
		t.Errorf("wrap (%d lines) should be taller than truncate (%d lines)",
			strings.Count(wout, "\n"), strings.Count(tout, "\n"))
	}
}

func TestRenderRejectsNoColumns(t *testing.T) {
	tbl := New()
	if err := tbl.Render(&bytes.Buffer{}, ForcePlain()); err == nil {
		t.Errorf("render with no columns = nil error, want error")
	}
}

func TestRenderRejectsRowMismatch(t *testing.T) {
	tbl := New().Col("A", Pin).Row(Text("a"), Text("b"))
	if err := tbl.Render(&bytes.Buffer{}, ForcePlain()); err == nil {
		t.Errorf("render with mismatched row = nil error, want error")
	}
}

func TestRenderStreamRoundTrip(t *testing.T) {
	src := sampleTable()
	var wire bytes.Buffer
	if err := EncodeStream(&wire, src); err != nil {
		t.Fatalf("encode: %v", err)
	}

	var out bytes.Buffer
	if err := RenderStream(&wire, &out, ForcePlain()); err != nil {
		t.Fatalf("render stream: %v", err)
	}
	got := out.String()
	for _, want := range []string{"ID\tSTATUS\tAGE", "api", "attached", "(current)", "web", "stale"} {
		if !strings.Contains(got, want) {
			t.Errorf("stream render missing %q:\n%s", want, got)
		}
	}
}

func TestSanitize(t *testing.T) {
	if got := sanitize("a\x00\x1b\x7fb\u009f"); got != "ab" {
		t.Errorf("sanitize = %q, want %q", got, "ab")
	}
	if got := sanitize("plain"); got != "plain" {
		t.Errorf("sanitize(plain) = %q, want plain", got)
	}
	// U+009F is a proper C1 control code point (2-byte UTF-8) and is stripped.
	if got := sanitize("x\u009fy"); got != "xy" {
		t.Errorf("sanitize(C1) = %q, want %q", got, "xy")
	}
}
