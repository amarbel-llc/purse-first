package tap

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewWriterEmitsVersionHeader(t *testing.T) {
	var buf bytes.Buffer
	NewWriter(&buf)
	if buf.String() != "TAP version 14\n" {
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

func TestNotOkWithoutDiagnostics(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	n := tw.NotOk("failing test", nil)
	if n != 1 {
		t.Errorf("expected test number 1, got %d", n)
	}
	if !strings.Contains(buf.String(), "not ok 1 - failing test\n") {
		t.Errorf("expected not ok line, got: %q", buf.String())
	}
	if strings.Contains(buf.String(), "---") {
		t.Error("should not contain YAML block without diagnostics")
	}
}

func TestNotOkWithDiagnostics(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.NotOk("error case", map[string]string{
		"message":  "something broke",
		"severity": "fail",
	})
	out := buf.String()
	if !strings.Contains(out, "  ---\n") {
		t.Errorf("expected YAML start, got: %q", out)
	}
	if !strings.Contains(out, "  message: something broke\n") {
		t.Errorf("expected message diagnostic, got: %q", out)
	}
	if !strings.Contains(out, "  severity: fail\n") {
		t.Errorf("expected severity diagnostic, got: %q", out)
	}
	if !strings.Contains(out, "  ...\n") {
		t.Errorf("expected YAML end, got: %q", out)
	}
}

func TestNotOkWithMultilineDiagnostic(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.NotOk("multiline", map[string]string{
		"output": "line one\nline two",
	})
	out := buf.String()
	if !strings.Contains(out, "output: |\n") {
		t.Errorf("expected YAML block scalar, got: %q", out)
	}
	if !strings.Contains(out, "    line one\n") {
		t.Errorf("expected indented line one, got: %q", out)
	}
	if !strings.Contains(out, "    line two\n") {
		t.Errorf("expected indented line two, got: %q", out)
	}
}

func TestDiagnosticKeysAreSorted(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.NotOk("sorted", map[string]string{
		"zebra": "last",
		"alpha": "first",
	})
	out := buf.String()
	alphaIdx := strings.Index(out, "alpha:")
	zebraIdx := strings.Index(out, "zebra:")
	if alphaIdx >= zebraIdx {
		t.Errorf("expected alpha before zebra in YAML block")
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

func TestTodoEmitsDirective(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	n := tw.Todo("unfinished", "not implemented yet")
	if n != 1 {
		t.Errorf("expected test number 1, got %d", n)
	}
	if !strings.Contains(buf.String(), "not ok 1 - unfinished # TODO not implemented yet\n") {
		t.Errorf("expected todo line, got: %q", buf.String())
	}
}

func TestPlanAhead(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.PlanAhead(5)
	if !strings.Contains(buf.String(), "1..5\n") {
		t.Errorf("expected plan line 1..5, got: %q", buf.String())
	}
	_ = tw
}

func TestPlanAfterTests(t *testing.T) {
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

func TestBailOut(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.BailOut("database unavailable")
	if !strings.Contains(buf.String(), "Bail out! database unavailable\n") {
		t.Errorf("expected bail out line, got: %q", buf.String())
	}
}

func TestComment(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.Comment("this is a comment")
	if !strings.Contains(buf.String(), "# this is a comment\n") {
		t.Errorf("expected comment line, got: %q", buf.String())
	}
}

func TestSubtestEmitsIndentedBlock(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	sub := tw.Subtest("nested")
	sub.Ok("inner pass")
	sub.Plan()
	tw.Ok("nested")

	expected := "TAP version 14\n" +
		"    # Subtest: nested\n" +
		"    ok 1 - inner pass\n" +
		"    1..1\n" +
		"ok 1 - nested\n"

	if buf.String() != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, buf.String())
	}
}

func TestSequentialNumbering(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	n1 := tw.Ok("pass")
	n2 := tw.NotOk("fail", nil)
	n3 := tw.Skip("skip", "lazy")
	n4 := tw.Todo("todo", "later")
	tw.Plan()

	if n1 != 1 || n2 != 2 || n3 != 3 || n4 != 4 {
		t.Errorf("expected 1,2,3,4 got %d,%d,%d,%d", n1, n2, n3, n4)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if lines[1] != "ok 1 - pass" {
		t.Errorf("line 1: %q", lines[1])
	}
	if lines[2] != "not ok 2 - fail" {
		t.Errorf("line 2: %q", lines[2])
	}
	if lines[3] != "ok 3 - skip # SKIP lazy" {
		t.Errorf("line 3: %q", lines[3])
	}
	if lines[4] != "not ok 4 - todo # TODO later" {
		t.Errorf("line 4: %q", lines[4])
	}
	if lines[5] != "1..4" {
		t.Errorf("plan line: %q", lines[5])
	}
}

func TestNestedSubtestTwoLevelsDeep(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	outer := tw.Subtest("outer")
	inner := outer.Subtest("inner")
	inner.Ok("deep test")
	inner.Plan()
	outer.Ok("inner")
	outer.Plan()
	tw.Ok("outer")

	expected := "TAP version 14\n" +
		"    # Subtest: outer\n" +
		"        # Subtest: inner\n" +
		"        ok 1 - deep test\n" +
		"        1..1\n" +
		"    ok 1 - inner\n" +
		"    1..1\n" +
		"ok 1 - outer\n"

	if buf.String() != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, buf.String())
	}
}

func TestSubtestNotOkWithDiagnostics(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	sub := tw.Subtest("pkg")
	sub.NotOk("failing", map[string]string{
		"message": "broke",
	})
	sub.Plan()
	tw.NotOk("pkg", nil)

	out := buf.String()
	if !strings.Contains(out, "    not ok 1 - failing\n") {
		t.Errorf("expected indented not ok, got:\n%s", out)
	}
	if !strings.Contains(out, "      ---\n") {
		t.Errorf("expected indented YAML start, got:\n%s", out)
	}
	if !strings.Contains(out, "      message: broke\n") {
		t.Errorf("expected indented diagnostic, got:\n%s", out)
	}
	if !strings.Contains(out, "      ...\n") {
		t.Errorf("expected indented YAML end, got:\n%s", out)
	}
}

func TestSubtestBailOut(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	sub := tw.Subtest("broken-pkg")
	sub.BailOut("build failed")
	tw.NotOk("broken-pkg", nil)

	out := buf.String()
	if !strings.Contains(out, "    Bail out! build failed\n") {
		t.Errorf("expected indented bail out, got:\n%s", out)
	}
}

func TestSubtestHasIndependentCounter(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	sub1 := tw.Subtest("first")
	sub1.Ok("a")
	sub1.Ok("b")
	sub1.Plan()
	tw.Ok("first")

	sub2 := tw.Subtest("second")
	n := sub2.Ok("c")
	sub2.Plan()
	tw.Ok("second")

	if n != 1 {
		t.Errorf("expected sub2 counter to start at 1, got %d", n)
	}
}

func TestSubtestOutputValidatesWithReader(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)

	sub := tw.Subtest("mypackage")
	sub.Ok("TestOne")
	sub.NotOk("TestTwo", map[string]string{"message": "failed"})
	sub.Plan()
	tw.NotOk("mypackage", nil)
	tw.Plan()

	reader := NewReader(strings.NewReader(buf.String()))
	summary := reader.Summary()
	if !summary.Valid {
		diags := reader.Diagnostics()
		for _, d := range diags {
			t.Errorf("diagnostic: line %d: %s: %s", d.Line, d.Severity, d.Message)
		}
		t.Fatalf("writer output did not validate as TAP-14:\n%s", buf.String())
	}
}
