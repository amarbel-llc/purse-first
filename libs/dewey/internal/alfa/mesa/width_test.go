package mesa

import (
	"bytes"
	"strings"
	"testing"
)

func TestNaturalWidths(t *testing.T) {
	tbl := New().
		Col("ID", Pin).
		Col("DESC", Flex).
		Row(Text("api"), Text("hello world")).
		Row(Text("web"), Text("x"))

	got := tbl.naturalWidths()
	want := []int{3, 11} // max("ID",3,3)=3 ; max("DESC",11,1)=11
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("naturalWidths = %v, want %v", got, want)
	}
}

func TestTargetWidthsUnconstrained(t *testing.T) {
	tbl := New().Col("A", Pin).Col("B", Flex)
	natural := []int{5, 20}
	got := tbl.targetWidths(natural, 0)
	if got[0] != 5 || got[1] != 20 {
		t.Errorf("unconstrained targetWidths = %v, want [5 20]", got)
	}
}

func TestTargetWidthsShrinksLowestPriorityFirst(t *testing.T) {
	tbl := New().
		Col("pin", Pin).
		Col("a", Flex, Shrink(0)).
		Col("b", Flex, Shrink(1))
	natural := []int{5, 20, 20}          // sum 45; overhead 3*3+1=10
	got := tbl.targetWidths(natural, 40) // budget 30, deficit 15

	if got[0] != 5 {
		t.Errorf("pin column shrank: got %d, want 5", got[0])
	}
	if got[1] >= got[2] {
		t.Errorf("lower-priority flex should shrink first: got a=%d b=%d", got[1], got[2])
	}
	// a shrinks to its floor (8) first, then b absorbs the rest.
	if got[1] != 8 || got[2] != 17 {
		t.Errorf("targetWidths = %v, want [5 8 17]", got)
	}
}

func TestTargetWidthsRespectsMinWidth(t *testing.T) {
	tbl := New().
		Col("pin", Pin).
		Col("a", Flex, Shrink(0), MinWidth(12)).
		Col("b", Flex, Shrink(1))
	natural := []int{5, 20, 20}
	got := tbl.targetWidths(natural, 40)
	if got[1] < 12 {
		t.Errorf("flex a shrank below its MinWidth 12: got %d", got[1])
	}
}

func TestEffectiveMinWidth(t *testing.T) {
	cases := []struct {
		col     Column
		natural int
		want    int
	}{
		{Column{}, 20, 8},              // default floor
		{Column{}, 5, 5},               // floor capped at natural
		{Column{MinWidth: 12}, 20, 12}, // explicit
		{Column{MinWidth: 12}, 6, 6},   // explicit capped at natural
	}
	for _, c := range cases {
		if got := effectiveMinWidth(c.col, c.natural); got != c.want {
			t.Errorf("effectiveMinWidth(%+v, %d) = %d, want %d", c.col, c.natural, got, c.want)
		}
	}
}

func TestTruncateSpansToWidthShortIsUntouched(t *testing.T) {
	spans := []Span{{Text: "abcd"}}
	got := truncateSpansToWidth(spans, 10)
	if len(got) != 1 || got[0].Text != "abcd" {
		t.Errorf("short cell truncated: %+v", got)
	}
}

func TestTruncateSpansToWidthEllipsizes(t *testing.T) {
	got := truncateSpansToWidth([]Span{{Text: "helloworld"}}, 6)
	plain := ""
	for _, sp := range got {
		plain += sp.Text
	}
	if !strings.HasSuffix(plain, "…") {
		t.Errorf("truncated cell missing ellipsis: %q", plain)
	}
	if w := displayWidth(plain); w > 6 {
		t.Errorf("truncated width %d exceeds max 6: %q", w, plain)
	}
}

func TestTruncateSpansPreservesLeadingStyledSpan(t *testing.T) {
	// A composite status-style cell: a colored glyph then plain text.
	got := truncateSpansToWidth([]Span{{Text: "●", Sev: OK}, {Text: " attached"}}, 5)
	if len(got) == 0 || got[0].Text != "●" || got[0].Sev != OK {
		t.Errorf("leading styled span not preserved: %+v", got)
	}
	plain := ""
	for _, sp := range got {
		plain += sp.Text
	}
	if w := displayWidth(plain); w > 5 {
		t.Errorf("truncated width %d exceeds max 5: %q", w, plain)
	}
}

func TestRenderStyledWidthTruncatesFlex(t *testing.T) {
	long := "this is a very long description that must be truncated"
	tbl := New().
		Col("ID", Pin).
		Col("DESC", Flex).
		Row(Text("api"), Text(long))

	var buf bytes.Buffer
	if err := tbl.Render(&buf, ForceStyle(), Width(30)); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "…") {
		t.Errorf("expected an ellipsis in width-constrained output:\n%s", out)
	}
	if !strings.Contains(out, "api") {
		t.Errorf("pin column content missing (should not be truncated):\n%s", out)
	}
	if strings.Contains(out, "truncated") {
		t.Errorf("flex column was not truncated (tail still present):\n%s", out)
	}
}
