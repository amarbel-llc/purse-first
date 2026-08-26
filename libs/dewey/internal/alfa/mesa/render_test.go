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
