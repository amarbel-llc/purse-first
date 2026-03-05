package tap

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"golang.org/x/text/language"
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

func TestPlanAheadPreventsDoublePlan(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.PlanAhead(2)
	tw.Ok("a")
	tw.Ok("b")
	tw.Plan()
	out := buf.String()
	count := strings.Count(out, "1..")
	if count != 1 {
		t.Errorf("expected exactly one plan line, got %d in:\n%s", count, out)
	}
}

func TestWriteDiagnosticsNamedFields(t *testing.T) {
	var buf bytes.Buffer
	writeDiagnostics(&buf, &Diagnostics{
		Message:  "something broke",
		Severity: "fail",
		File:     "main.go",
		Line:     42,
	}, false)
	out := buf.String()
	expected := "  ---\n  file: main.go\n  line: 42\n  message: something broke\n  severity: fail\n  ...\n"
	if out != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, out)
	}
}

func TestWriteDiagnosticsOmitsZeroValues(t *testing.T) {
	var buf bytes.Buffer
	writeDiagnostics(&buf, &Diagnostics{
		Message: "only message",
	}, false)
	out := buf.String()
	if strings.Contains(out, "severity:") || strings.Contains(out, "file:") || strings.Contains(out, "line:") {
		t.Errorf("expected zero-value fields omitted, got:\n%s", out)
	}
	if !strings.Contains(out, "message: only message") {
		t.Errorf("expected message field, got:\n%s", out)
	}
}

func TestWriteDiagnosticsExtras(t *testing.T) {
	var buf bytes.Buffer
	writeDiagnostics(&buf, &Diagnostics{
		Message: "error",
		Extras: map[string]any{
			"exitcode": 1,
			"context":  "test run",
		},
	}, false)
	out := buf.String()
	if !strings.Contains(out, "  context: test run\n") {
		t.Errorf("expected context extra, got:\n%s", out)
	}
	if !strings.Contains(out, "  exitcode: 1\n") {
		t.Errorf("expected exitcode extra, got:\n%s", out)
	}
}

func TestWriteDiagnosticsMultilineExtra(t *testing.T) {
	var buf bytes.Buffer
	writeDiagnostics(&buf, &Diagnostics{
		Extras: map[string]any{
			"output": "line one\nline two",
		},
	}, false)
	out := buf.String()
	if !strings.Contains(out, "  output: |\n    line one\n    line two\n") {
		t.Errorf("expected block scalar for multiline extra, got:\n%s", out)
	}
}

func TestWriteDiagnosticsNil(t *testing.T) {
	var buf bytes.Buffer
	writeDiagnostics(&buf, nil, false)
	if buf.Len() != 0 {
		t.Errorf("expected no output for nil diagnostics, got: %q", buf.String())
	}
}

func TestWriteAllBasicOk(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.WriteAll(slices.Values([]TestPoint{
		{Description: "first", Ok: true},
		{Description: "second", Ok: true},
	}))
	expected := "TAP version 14\n" +
		"ok 1 - first\n" +
		"ok 2 - second\n" +
		"1..2\n"
	if buf.String() != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, buf.String())
	}
}

func TestWriteAllNotOkWithDiagnostics(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.WriteAll(slices.Values([]TestPoint{
		{Description: "failing", Ok: false, Diagnostics: &Diagnostics{
			Message:  "broke",
			Severity: "fail",
		}},
	}))
	out := buf.String()
	if !strings.Contains(out, "not ok 1 - failing\n") {
		t.Errorf("expected not ok line, got:\n%s", out)
	}
	if !strings.Contains(out, "  message: broke\n") {
		t.Errorf("expected message diagnostic, got:\n%s", out)
	}
	if !strings.HasSuffix(out, "1..1\n") {
		t.Errorf("expected trailing plan, got:\n%s", out)
	}
}

func TestWriteAllSkip(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.WriteAll(slices.Values([]TestPoint{
		{Description: "skipped", Skip: "not ready"},
	}))
	if !strings.Contains(buf.String(), "ok 1 - skipped # SKIP not ready\n") {
		t.Errorf("expected skip line, got:\n%s", buf.String())
	}
}

func TestWriteAllTodo(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.WriteAll(slices.Values([]TestPoint{
		{Description: "unfinished", Todo: "later"},
	}))
	if !strings.Contains(buf.String(), "not ok 1 - unfinished # TODO later\n") {
		t.Errorf("expected todo line, got:\n%s", buf.String())
	}
}

func TestWriteAllPlanAheadSkipsTrailingPlan(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.PlanAhead(2)
	tw.WriteAll(slices.Values([]TestPoint{
		{Description: "a", Ok: true},
		{Description: "b", Ok: true},
	}))
	count := strings.Count(buf.String(), "1..")
	if count != 1 {
		t.Errorf("expected exactly one plan line, got %d in:\n%s", count, buf.String())
	}
}

func TestWriteAllSubtest(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.WriteAll(slices.Values([]TestPoint{
		{Description: "nested", Subtests: func(sub *Writer) {
			sub.Ok("inner pass")
		}},
	}))
	expected := "TAP version 14\n" +
		"    # Subtest: nested\n" +
		"    ok 1 - inner pass\n" +
		"    1..1\n" +
		"ok 1 - nested\n" +
		"1..1\n"
	if buf.String() != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, buf.String())
	}
}

func TestWriteAllNestedWriteAll(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.WriteAll(slices.Values([]TestPoint{
		{Description: "outer", Subtests: func(sub *Writer) {
			sub.WriteAll(slices.Values([]TestPoint{
				{Description: "inner-a", Ok: true},
				{Description: "inner-b", Ok: false, Diagnostics: &Diagnostics{
					Message: "broke",
				}},
			}))
		}},
	}))
	out := buf.String()
	if !strings.Contains(out, "    ok 1 - inner-a\n") {
		t.Errorf("expected inner-a, got:\n%s", out)
	}
	if !strings.Contains(out, "    not ok 2 - inner-b\n") {
		t.Errorf("expected inner-b, got:\n%s", out)
	}
	if !strings.Contains(out, "    1..2\n") {
		t.Errorf("expected subtest plan, got:\n%s", out)
	}
	if !strings.Contains(out, "ok 1 - outer\n") {
		t.Errorf("expected parent ok, got:\n%s", out)
	}
}

func TestWriteAllMixedImperativeAndIterator(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.WriteAll(slices.Values([]TestPoint{
		{Description: "mixed", Subtests: func(sub *Writer) {
			sub.Ok("imperative")
			sub.WriteAll(slices.Values([]TestPoint{
				{Description: "from-iter", Ok: true},
			}))
		}},
	}))
	out := buf.String()
	if !strings.Contains(out, "    ok 1 - imperative\n") {
		t.Errorf("expected imperative test, got:\n%s", out)
	}
	if !strings.Contains(out, "    ok 2 - from-iter\n") {
		t.Errorf("expected iterator test, got:\n%s", out)
	}
	if !strings.Contains(out, "    1..2\n") {
		t.Errorf("expected combined plan 1..2, got:\n%s", out)
	}
}

func TestPragma(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.Pragma("streamed-output", true)
	tw.Pragma("strict", false)
	out := buf.String()
	if !strings.Contains(out, "pragma +streamed-output\n") {
		t.Errorf("expected pragma +streamed-output, got: %q", out)
	}
	if !strings.Contains(out, "pragma -strict\n") {
		t.Errorf("expected pragma -strict, got: %q", out)
	}
}

func TestStreamedOutput(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.StreamedOutput("compiling main.rs")
	tw.StreamedOutput("linking binary")
	out := buf.String()
	if !strings.Contains(out, "# compiling main.rs\n") {
		t.Errorf("expected streamed output line, got: %q", out)
	}
	if !strings.Contains(out, "# linking binary\n") {
		t.Errorf("expected streamed output line, got: %q", out)
	}
}

func TestWriteAllOutputValidatesWithReader(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.WriteAll(slices.Values([]TestPoint{
		{Description: "pass", Ok: true},
		{Description: "fail", Ok: false, Diagnostics: &Diagnostics{
			Message: "broke",
		}},
		{Description: "skipped", Skip: "not ready"},
		{Description: "todo", Todo: "later"},
		{Description: "nested", Subtests: func(sub *Writer) {
			sub.WriteAll(slices.Values([]TestPoint{
				{Description: "inner", Ok: true},
			}))
		}},
	}))

	reader := NewReader(strings.NewReader(buf.String()))
	summary := reader.Summary()
	if !summary.Valid {
		diags := reader.Diagnostics()
		for _, d := range diags {
			t.Errorf("diagnostic: line %d: %s: %s", d.Line, d.Severity, d.Message)
		}
		t.Fatalf("WriteAll output did not validate as TAP-14:\n%s", buf.String())
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

func TestHasFailuresInitiallyFalse(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	if tw.HasFailures() {
		t.Error("expected HasFailures to be false for new writer")
	}
}

func TestHasFailuresAfterOk(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.Ok("pass")
	if tw.HasFailures() {
		t.Error("expected HasFailures to be false after Ok")
	}
}

func TestHasFailuresAfterNotOk(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.NotOk("fail", nil)
	if !tw.HasFailures() {
		t.Error("expected HasFailures to be true after NotOk")
	}
}

func TestHasFailuresAfterOkThenNotOk(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.Ok("pass")
	tw.NotOk("fail", nil)
	if !tw.HasFailures() {
		t.Error("expected HasFailures to be true after any NotOk")
	}
}

func TestHasFailuresNotAffectedBySkipOrTodo(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.Skip("skipped", "reason")
	tw.Todo("todo", "later")
	if tw.HasFailures() {
		t.Error("expected HasFailures to be false after only Skip and Todo")
	}
}

func TestWriteAllOkWithDiagnostics(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.WriteAll(slices.Values([]TestPoint{
		{Description: "pass with info", Ok: true, Diagnostics: &Diagnostics{
			Message: "inserted id=42",
		}},
	}))
	out := buf.String()
	if !strings.Contains(out, "ok 1 - pass with info\n") {
		t.Errorf("expected ok line, got:\n%s", out)
	}
	if !strings.Contains(out, "  ---\n") {
		t.Errorf("expected YAML block after ok, got:\n%s", out)
	}
	if !strings.Contains(out, "  message: inserted id=42\n") {
		t.Errorf("expected message diagnostic, got:\n%s", out)
	}
}

func TestLocaleWriterEmitsPragma(t *testing.T) {
	var buf bytes.Buffer
	tw := NewLocaleWriter(&buf, language.MustParse("en-US"))
	tw.Ok("first")
	tw.Plan()
	out := buf.String()
	if !strings.Contains(out, "pragma +locale-formatting:en-US\n") {
		t.Errorf("expected locale pragma, got:\n%s", out)
	}
}

func TestLocaleWriterFormatsTestPointNumber(t *testing.T) {
	var buf bytes.Buffer
	tw := NewLocaleWriter(&buf, language.MustParse("en-US"))
	for i := 0; i < 1234; i++ {
		tw.Ok("test")
	}
	out := buf.String()
	if !strings.Contains(out, "ok 1,234 - test\n") {
		t.Errorf("expected locale-formatted number ok 1,234, got last lines:\n%s",
			out[max(0, len(out)-200):])
	}
}

func TestLocaleWriterFormatsPlanCount(t *testing.T) {
	var buf bytes.Buffer
	tw := NewLocaleWriter(&buf, language.MustParse("en-US"))
	tw.PlanAhead(10000)
	out := buf.String()
	if !strings.Contains(out, "1..10,000\n") {
		t.Errorf("expected locale-formatted plan 1..10,000, got:\n%s", out)
	}
}

func TestLocaleWriterGermanSeparator(t *testing.T) {
	var buf bytes.Buffer
	tw := NewLocaleWriter(&buf, language.MustParse("de-DE"))
	tw.PlanAhead(10000)
	out := buf.String()
	if !strings.Contains(out, "1..10.000\n") {
		t.Errorf("expected German-formatted plan 1..10.000, got:\n%s", out)
	}
}

func TestLocaleWriterSmallNumbersUnformatted(t *testing.T) {
	var buf bytes.Buffer
	tw := NewLocaleWriter(&buf, language.MustParse("en-US"))
	tw.Ok("test")
	out := buf.String()
	if !strings.Contains(out, "ok 1 - test\n") {
		t.Errorf("expected plain number for small values, got:\n%s", out)
	}
}

func TestNotOkStripsANSIWhenNoColor(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	tw.NotOk("test", map[string]string{
		"message": "\033[31merror\033[0m happened",
	})
	out := buf.String()
	if strings.Contains(out, "\033[") {
		t.Errorf("expected ANSI stripped in no-color mode, got:\n%s", out)
	}
	if !strings.Contains(out, "  message: error happened\n") {
		t.Errorf("expected clean message, got:\n%s", out)
	}
}

func TestNotOkPreservesSGRWhenColor(t *testing.T) {
	var buf bytes.Buffer
	tw := NewColorWriter(&buf, true)
	tw.NotOk("test", map[string]string{
		"message": "\033[31merror\033[0m happened",
	})
	out := buf.String()
	if !strings.Contains(out, "  message: \033[31merror\033[0m happened\n") {
		t.Errorf("expected SGR preserved in color mode, got:\n%s", out)
	}
}

func TestNotOkStripsNonSGRInColorMode(t *testing.T) {
	var buf bytes.Buffer
	tw := NewColorWriter(&buf, true)
	tw.NotOk("test", map[string]string{
		"output": "\033[2J\033[31merror\033[0m text",
	})
	out := buf.String()
	if strings.Contains(out, "\033[2J") {
		t.Errorf("expected non-SGR stripped even in color mode, got:\n%s", out)
	}
	if !strings.Contains(out, "\033[31merror\033[0m") {
		t.Errorf("expected SGR preserved in color mode, got:\n%s", out)
	}
}

func TestWriteDiagnosticsPreservesSGRWhenColor(t *testing.T) {
	var buf bytes.Buffer
	writeDiagnostics(&buf, &Diagnostics{
		File:    "main.go",
		Line:    42,
		Message: "\033[31merror\033[0m text",
		Extras: map[string]any{
			"output": "\033[33mwarning\033[0m details",
		},
	}, true)
	out := buf.String()
	// Message should preserve SGR
	if !strings.Contains(out, "\033[31merror\033[0m text") {
		t.Errorf("expected SGR in message, got:\n%s", out)
	}
	// Extras should preserve SGR
	if !strings.Contains(out, "\033[33mwarning\033[0m details") {
		t.Errorf("expected SGR in extras, got:\n%s", out)
	}
	// File should NOT be sanitized (structured field)
	if !strings.Contains(out, "  file: main.go\n") {
		t.Errorf("expected file unchanged, got:\n%s", out)
	}
}

func TestWriteDiagnosticsStripsANSIWhenNoColor(t *testing.T) {
	var buf bytes.Buffer
	writeDiagnostics(&buf, &Diagnostics{
		Message: "\033[31merror\033[0m text",
	}, false)
	out := buf.String()
	if strings.Contains(out, "\033[") {
		t.Errorf("expected ANSI stripped when color=false, got:\n%s", out)
	}
	if !strings.Contains(out, "  message: error text\n") {
		t.Errorf("expected clean message, got:\n%s", out)
	}
}

func TestLocaleWriterSubtestInheritsLocale(t *testing.T) {
	var buf bytes.Buffer
	tw := NewLocaleWriter(&buf, language.MustParse("en-US"))
	sub := tw.Subtest("nested")
	sub.PlanAhead(10000)
	sub.Plan()
	tw.Ok("nested")
	tw.Plan()
	out := buf.String()
	if !strings.Contains(out, "    pragma +locale-formatting:en-US\n") {
		t.Errorf("expected subtest to inherit and emit locale pragma, got:\n%s", out)
	}
	if !strings.Contains(out, "    1..10,000\n") {
		t.Errorf("expected subtest to use locale formatting, got:\n%s", out)
	}
}
