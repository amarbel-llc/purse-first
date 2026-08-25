package mesa

import (
	"bytes"
	"strings"
	"testing"
)

func sampleTable() *Table {
	return New().
		Col("ID", Flex).
		Col("STATUS", Pin).
		Col("AGE", Pin, WithAlign(Right)).
		Legend(Entry(OK, "●", "attached"), Entry(Error, "●", "stale")).
		Empty("no sessions").
		Palette(map[Severity]string{Special: "#c678dd"}).
		Row(Text("api"), Status(OK, "attached", WithMarker("(current)")), Text("2m")).
		Row(Text("web"), Status(Error, "stale"), Text("5m"))
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	orig := sampleTable()

	var buf bytes.Buffer
	if err := EncodeStream(&buf, orig); err != nil {
		t.Fatalf("encode: %v", err)
	}
	first := buf.String()

	decoded, err := DecodeStream(strings.NewReader(first))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	var buf2 bytes.Buffer
	if err := EncodeStream(&buf2, decoded); err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	if buf2.String() != first {
		t.Errorf("round-trip not stable:\nfirst:\n%s\nsecond:\n%s", first, buf2.String())
	}
}

func TestEncodePlainCellIsBareString(t *testing.T) {
	var buf bytes.Buffer
	tbl := New().Col("ID", Pin).Row(Text("api"))
	if err := EncodeStream(&buf, tbl); err != nil {
		t.Fatalf("encode: %v", err)
	}
	if !strings.Contains(buf.String(), `"cells":["api"]`) {
		t.Errorf("plain cell not encoded as bare string:\n%s", buf.String())
	}
}

func TestEncodeDisablesHTMLEscaping(t *testing.T) {
	var buf bytes.Buffer
	tbl := New().Col("Q", Pin).Row(Text("a < b & c"))
	if err := EncodeStream(&buf, tbl); err != nil {
		t.Fatalf("encode: %v", err)
	}
	if !strings.Contains(buf.String(), "a < b & c") {
		t.Errorf("expected verbatim '<' and '&', got:\n%s", buf.String())
	}
}

func TestDecodeStringCellShorthand(t *testing.T) {
	in := `{"columns":[{"name":"A","role":"pin"}]}
{"cells":["hi"]}
`
	tbl, err := DecodeStream(strings.NewReader(in))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(tbl.Rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(tbl.Rows))
	}
	got := tbl.Rows[0].Cells[0]
	if len(got.Spans) != 1 || got.Spans[0].Text != "hi" || got.Spans[0].Sev != Neutral {
		t.Errorf("shorthand cell = %+v", got)
	}
}

func TestDecodeUnknownSeverityDegradesToNeutral(t *testing.T) {
	in := `{"columns":[{"name":"A","role":"pin"}]}
{"cells":[{"spans":[{"text":"x","sev":"wat"}]}]}
`
	tbl, err := DecodeStream(strings.NewReader(in))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := tbl.Rows[0].Cells[0].Spans[0].Sev; got != Neutral {
		t.Errorf("unknown severity = %v, want Neutral", got)
	}
}

func TestDecodeProtocolErrors(t *testing.T) {
	cases := map[string]string{
		"first record lacks columns": `{"cells":["x"]}`,
		"empty columns":              `{"columns":[]}`,
		"invalid role":               `{"columns":[{"name":"A","role":"bogus"}]}`,
		"empty column name":          `{"columns":[{"name":"","role":"pin"}]}`,
		"unsupported version":        `{"v":2,"columns":[{"name":"A","role":"pin"}]}`,
		"cell count mismatch":        "{\"columns\":[{\"name\":\"A\",\"role\":\"pin\"}]}\n{\"cells\":[\"a\",\"b\"]}",
		"empty stream":               ``,
	}
	for name, in := range cases {
		if _, err := DecodeStream(strings.NewReader(in)); err == nil {
			t.Errorf("%s: DecodeStream = nil error, want protocol error", name)
		}
	}
}
