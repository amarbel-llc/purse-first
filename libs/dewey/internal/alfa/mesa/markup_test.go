package mesa

import "testing"

func TestParseMarkupNeutral(t *testing.T) {
	spans, err := parseMarkup("hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1: %+v", len(spans), spans)
	}
	if spans[0].Text != "hello world" || spans[0].Sev != Neutral {
		t.Errorf("span = %+v, want neutral %q", spans[0], "hello world")
	}
}

func TestParseMarkupStyled(t *testing.T) {
	spans, err := parseMarkup("<ok b>●</ok> attached <muted>(current)</muted>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []Span{
		{Text: "●", Sev: OK, Bold: true},
		{Text: " attached "},
		{Text: "(current)", Sev: Muted},
	}
	if len(spans) != len(want) {
		t.Fatalf("got %d spans, want %d: %+v", len(spans), len(want), spans)
	}
	for i := range want {
		if spans[i] != want[i] {
			t.Errorf("span[%d] = %+v, want %+v", i, spans[i], want[i])
		}
	}
}

func TestParseMarkupEscapes(t *testing.T) {
	spans, err := parseMarkup(`a \< b \\ c`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spans) != 1 || spans[0].Text != `a < b \ c` {
		t.Errorf("got %+v, want single span %q", spans, `a < b \ c`)
	}
}

func TestParseMarkupErrors(t *testing.T) {
	cases := map[string]string{
		"unknown severity":   "<bogus>x</bogus>",
		"unknown attr":       "<ok z>x</ok>",
		"nested":             "<ok><error>x</error></ok>",
		"close without open": "x</ok>",
		"unclosed":           "<ok>x",
		"unterminated tag":   "<ok x",
		"empty tag":          "<>x",
	}
	for name, in := range cases {
		if _, err := parseMarkup(in); err == nil {
			t.Errorf("%s: parseMarkup(%q) = nil error, want error", name, in)
		}
	}
}

func TestMarkupPanicsOnMalformed(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Errorf("Markup on malformed input did not panic")
		}
	}()
	_ = Markup("<nope>x</nope>")
}

func TestMarkupCompilesToCell(t *testing.T) {
	c := Markup("<error>stale</error>")
	if len(c.Spans) != 1 || c.Spans[0].Sev != Error || c.Spans[0].Text != "stale" {
		t.Errorf("Markup cell = %+v", c)
	}
}
