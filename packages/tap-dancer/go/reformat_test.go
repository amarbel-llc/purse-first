package tap

import (
	"bytes"
	"strings"
	"testing"
)

func TestReformatTAPAddsVersionHeader(t *testing.T) {
	input := "1..1\nok 1 - pass\n"
	var buf bytes.Buffer
	ReformatTAP(strings.NewReader(input), &buf, false)
	lines := strings.SplitN(buf.String(), "\n", 2)
	if lines[0] != "TAP version 14" {
		t.Errorf("expected first line TAP version 14, got %q", lines[0])
	}
}

func TestReformatTAPDropsExistingVersionLine(t *testing.T) {
	input := "TAP version 14\n1..1\nok 1 - pass\n"
	var buf bytes.Buffer
	ReformatTAP(strings.NewReader(input), &buf, false)
	count := strings.Count(buf.String(), "TAP version 14")
	if count != 1 {
		t.Errorf("expected exactly one version line, got %d in:\n%s", count, buf.String())
	}
}

func TestReformatTAPColorizesOk(t *testing.T) {
	input := "ok 1 - pass\n"
	var buf bytes.Buffer
	ReformatTAP(strings.NewReader(input), &buf, true)
	expected := ansiGreen + "ok" + ansiReset + " 1 - pass\n"
	if !strings.Contains(buf.String(), expected) {
		t.Errorf("expected colorized ok line %q, got:\n%s", expected, buf.String())
	}
}

func TestReformatTAPColorizesNotOk(t *testing.T) {
	input := "not ok 1 - fail\n"
	var buf bytes.Buffer
	ReformatTAP(strings.NewReader(input), &buf, true)
	expected := ansiRed + "not ok" + ansiReset + " 1 - fail\n"
	if !strings.Contains(buf.String(), expected) {
		t.Errorf("expected colorized not ok line %q, got:\n%s", expected, buf.String())
	}
}

func TestReformatTAPNormalizesAndColorizesSkip(t *testing.T) {
	input := "ok 1 - skipped # skip not ready\n"
	var buf bytes.Buffer
	ReformatTAP(strings.NewReader(input), &buf, true)
	out := buf.String()
	if !strings.Contains(out, ansiYellow+"# SKIP"+ansiReset) {
		t.Errorf("expected colorized # SKIP, got:\n%s", out)
	}
	if strings.Contains(out, "# skip") {
		t.Errorf("expected # skip to be normalized to # SKIP, got:\n%s", out)
	}
}

func TestReformatTAPNormalizesSkipNoColor(t *testing.T) {
	input := "ok 1 - skipped # skip not ready\n"
	var buf bytes.Buffer
	ReformatTAP(strings.NewReader(input), &buf, false)
	out := buf.String()
	if !strings.Contains(out, "# SKIP") {
		t.Errorf("expected # SKIP, got:\n%s", out)
	}
}

func TestReformatTAPColorizesTodo(t *testing.T) {
	input := "not ok 1 - todo # TODO not implemented\n"
	var buf bytes.Buffer
	ReformatTAP(strings.NewReader(input), &buf, true)
	if !strings.Contains(buf.String(), ansiYellow+"# TODO"+ansiReset) {
		t.Errorf("expected colorized # TODO, got:\n%s", buf.String())
	}
}

func TestReformatTAPColorizesBailOut(t *testing.T) {
	input := "Bail out! database down\n"
	var buf bytes.Buffer
	ReformatTAP(strings.NewReader(input), &buf, true)
	expected := ansiRed + "Bail out!" + ansiReset + " database down\n"
	if !strings.Contains(buf.String(), expected) {
		t.Errorf("expected colorized bail out %q, got:\n%s", expected, buf.String())
	}
}

func TestReformatTAPNoColorWhenDisabled(t *testing.T) {
	input := "ok 1 - pass\nnot ok 2 - fail\nBail out! reason\n"
	var buf bytes.Buffer
	ReformatTAP(strings.NewReader(input), &buf, false)
	out := buf.String()
	if strings.Contains(out, "\033[") {
		t.Errorf("expected no ANSI sequences with color=false, got:\n%s", out)
	}
}

func TestReformatTAPPassesThroughOtherLines(t *testing.T) {
	input := "1..3\n# comment\nok 1 - pass\n  ---\n  message: hello\n  ...\nnot ok 2 - fail\nok 3 - last\n"
	var buf bytes.Buffer
	ReformatTAP(strings.NewReader(input), &buf, false)
	out := buf.String()
	if !strings.Contains(out, "1..3\n") {
		t.Errorf("expected plan line passthrough, got:\n%s", out)
	}
	if !strings.Contains(out, "# comment\n") {
		t.Errorf("expected comment passthrough, got:\n%s", out)
	}
	if !strings.Contains(out, "  ---\n") {
		t.Errorf("expected YAML start passthrough, got:\n%s", out)
	}
	if !strings.Contains(out, "  message: hello\n") {
		t.Errorf("expected YAML content passthrough, got:\n%s", out)
	}
	if !strings.Contains(out, "  ...\n") {
		t.Errorf("expected YAML end passthrough, got:\n%s", out)
	}
}

func TestReformatTAPOutputValidatesWithReader(t *testing.T) {
	input := "1..3\nok 1 - pass\nnot ok 2 - fail\nok 3 - skip # skip lazy\n"
	var buf bytes.Buffer
	ReformatTAP(strings.NewReader(input), &buf, false)
	reader := NewReader(strings.NewReader(buf.String()))
	summary := reader.Summary()
	if !summary.Valid {
		diags := reader.Diagnostics()
		for _, d := range diags {
			t.Errorf("diagnostic: line %d: %s: %s", d.Line, d.Severity, d.Message)
		}
		t.Fatalf("reformatted output did not validate as TAP-14:\n%s", buf.String())
	}
}
