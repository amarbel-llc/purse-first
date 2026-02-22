package tap

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewWriterEmitsVersionHeader(t *testing.T) {
	var buf bytes.Buffer
	NewWriter(&buf)
	if !strings.HasPrefix(buf.String(), "TAP version 14\n") {
		t.Errorf("expected TAP version 14 header, got: %q", buf.String())
	}
}

func TestOkEmitsLine(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	n := tw.Ok("first test")
	if n != 1 {
		t.Errorf("expected test number 1, got %d", n)
	}
	if !strings.Contains(buf.String(), "ok 1 - first test\n") {
		t.Errorf("expected ok line, got: %q", buf.String())
	}
}

func TestSequentialNumbering(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.Ok("one")
	tw.Ok("two")
	n := tw.Ok("three")
	if n != 3 {
		t.Errorf("expected test number 3, got %d", n)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if lines[1] != "ok 1 - one" {
		t.Errorf("line 1: %q", lines[1])
	}
	if lines[2] != "ok 2 - two" {
		t.Errorf("line 2: %q", lines[2])
	}
	if lines[3] != "ok 3 - three" {
		t.Errorf("line 3: %q", lines[3])
	}
}

func TestNotOkEmitsLine(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	n := tw.NotOk("failing test", nil)
	if n != 1 {
		t.Errorf("expected test number 1, got %d", n)
	}
	if !strings.Contains(buf.String(), "not ok 1 - failing test\n") {
		t.Errorf("expected not ok line, got: %q", buf.String())
	}
}

func TestNotOkWithDiagnostics(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.NotOk("error case", map[string]string{
		"message": "something broke",
		"error":   "exit status 1",
	})
	out := buf.String()
	if !strings.Contains(out, "  ---\n") {
		t.Errorf("expected YAML start, got: %q", out)
	}
	if !strings.Contains(out, "  error: exit status 1\n") {
		t.Errorf("expected error diagnostic, got: %q", out)
	}
	if !strings.Contains(out, "  message: something broke\n") {
		t.Errorf("expected message diagnostic, got: %q", out)
	}
	if !strings.Contains(out, "  ...\n") {
		t.Errorf("expected YAML end, got: %q", out)
	}
}

func TestSkipEmitsDirective(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	n := tw.Skip("skipped test", "not applicable")
	if n != 1 {
		t.Errorf("expected test number 1, got %d", n)
	}
	if !strings.Contains(buf.String(), "ok 1 - skipped test # SKIP not applicable\n") {
		t.Errorf("expected skip line, got: %q", buf.String())
	}
}

func TestPlanAheadEmitsCount(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.PlanAhead(3)
	tw.Ok("a")
	tw.Ok("b")
	tw.Ok("c")
	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if lines[1] != "1..3" {
		t.Errorf("expected plan-ahead line 1..3 after header, got: %q", lines[1])
	}
	if lines[2] != "ok 1 - a" {
		t.Errorf("expected first test after plan, got: %q", lines[2])
	}
}

func TestPlanEmitsCount(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.Ok("a")
	tw.Ok("b")
	tw.Plan()
	if !strings.HasSuffix(buf.String(), "1..2\n") {
		t.Errorf("expected plan line 1..2, got: %q", buf.String())
	}
}

func TestPlanWithZeroTests(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.Plan()
	if !strings.HasSuffix(buf.String(), "1..0\n") {
		t.Errorf("expected plan line 1..0, got: %q", buf.String())
	}
}

func TestMixedOperations(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.Ok("pass")
	tw.NotOk("fail", map[string]string{"reason": "bad"})
	tw.Skip("skip", "lazy")
	tw.Plan()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if lines[0] != "TAP version 14" {
		t.Errorf("line 0: %q", lines[0])
	}
	if lines[1] != "ok 1 - pass" {
		t.Errorf("line 1: %q", lines[1])
	}
	if lines[2] != "not ok 2 - fail" {
		t.Errorf("line 2: %q", lines[2])
	}
	if lines[len(lines)-2] != "ok 3 - skip # SKIP lazy" {
		t.Errorf("skip line: %q", lines[len(lines)-2])
	}
	if lines[len(lines)-1] != "1..3" {
		t.Errorf("plan line: %q", lines[len(lines)-1])
	}
}
