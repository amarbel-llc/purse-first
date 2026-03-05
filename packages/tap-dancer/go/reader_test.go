package tap

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"golang.org/x/text/language"
)

func collectEvents(input string) ([]Event, []Diagnostic, Summary) {
	r := NewReader(strings.NewReader(input))
	var events []Event
	for {
		ev, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		events = append(events, ev)
	}
	return events, r.Diagnostics(), r.Summary()
}

func TestReaderValidMinimal(t *testing.T) {
	input := "TAP version 14\n1..2\nok 1 - first\nok 2 - second\n"
	events, diags, summary := collectEvents(input)

	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}
	if events[0].Type != EventVersion {
		t.Errorf("event 0: expected Version, got %v", events[0].Type)
	}
	if events[1].Type != EventPlan {
		t.Errorf("event 1: expected Plan, got %v", events[1].Type)
	}
	if events[2].Type != EventTestPoint {
		t.Errorf("event 2: expected TestPoint, got %v", events[2].Type)
	}

	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error diagnostic: %s: %s", d.Rule, d.Message)
		}
	}

	if !summary.Valid {
		t.Error("expected Valid=true")
	}
	if summary.TotalTests != 2 {
		t.Errorf("expected 2 total tests, got %d", summary.TotalTests)
	}
	if summary.Passed != 2 {
		t.Errorf("expected 2 passed, got %d", summary.Passed)
	}
}

func TestReaderTrailingPlan(t *testing.T) {
	input := "TAP version 14\nok 1 - a\nok 2 - b\n1..2\n"
	_, diags, summary := collectEvents(input)

	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error: %s: %s", d.Rule, d.Message)
		}
	}
	if !summary.Valid {
		t.Error("expected Valid=true for trailing plan")
	}
}

func TestReaderMissingVersion(t *testing.T) {
	input := "1..1\nok 1 - test\n"
	_, diags, summary := collectEvents(input)

	if summary.Valid {
		t.Error("expected Valid=false for missing version")
	}
	found := false
	for _, d := range diags {
		if d.Rule == "version-required" {
			found = true
		}
	}
	if !found {
		t.Error("expected version-required diagnostic")
	}
}

func TestReaderPlanCountMismatch(t *testing.T) {
	input := "TAP version 14\n1..3\nok 1 - a\nok 2 - b\n"
	_, diags, summary := collectEvents(input)

	if summary.Valid {
		t.Error("expected Valid=false for plan count mismatch")
	}
	found := false
	for _, d := range diags {
		if d.Rule == "plan-count-mismatch" {
			found = true
		}
	}
	if !found {
		t.Error("expected plan-count-mismatch diagnostic")
	}
}

func TestReaderDuplicatePlan(t *testing.T) {
	input := "TAP version 14\n1..1\nok 1 - a\n1..1\n"
	_, diags, _ := collectEvents(input)

	found := false
	for _, d := range diags {
		if d.Rule == "plan-duplicate" {
			found = true
		}
	}
	if !found {
		t.Error("expected plan-duplicate diagnostic")
	}
}

func TestReaderYAMLBlock(t *testing.T) {
	input := "TAP version 14\n1..1\nnot ok 1 - fail\n  ---\n  message: broken\n  severity: fail\n  ...\n"
	events, diags, _ := collectEvents(input)

	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error: %s: %s", d.Rule, d.Message)
		}
	}

	foundYAML := false
	for _, ev := range events {
		if ev.Type == EventYAMLDiagnostic {
			foundYAML = true
			if ev.YAML["message"] != "broken" {
				t.Errorf("YAML message = %q, want %q", ev.YAML["message"], "broken")
			}
		}
	}
	if !foundYAML {
		t.Error("expected YAML diagnostic event")
	}
}

func TestReaderBailOut(t *testing.T) {
	input := "TAP version 14\n1..3\nok 1 - a\nBail out! database down\n"
	_, _, summary := collectEvents(input)

	if !summary.BailedOut {
		t.Error("expected BailedOut=true")
	}
}

func TestReaderBailOutSuppressesPlanMismatch(t *testing.T) {
	input := "TAP version 14\n1..3\nok 1 - a\nBail out! database down\n"
	r := NewReader(strings.NewReader(input))
	diags := r.Diagnostics()
	summary := r.Summary()

	if !summary.BailedOut {
		t.Error("expected BailedOut=true")
	}
	for _, d := range diags {
		if d.Rule == "plan-count-mismatch" {
			t.Errorf("bail out should suppress plan-count-mismatch, got: %s", d.Message)
		}
	}
	if !summary.Valid {
		t.Error("expected Valid=true when bailed out (plan mismatch suppressed)")
	}
}

func TestReaderSkipAndTodo(t *testing.T) {
	input := "TAP version 14\n1..3\nok 1 - a\nok 2 - b # SKIP lazy\nnot ok 3 - c # TODO later\n"
	_, _, summary := collectEvents(input)

	if summary.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", summary.Skipped)
	}
	if summary.Todo != 1 {
		t.Errorf("expected 1 todo, got %d", summary.Todo)
	}
}

func TestReaderNumberSequenceWarning(t *testing.T) {
	input := "TAP version 14\n1..2\nok 1 - a\nok 5 - b\n"
	_, diags, _ := collectEvents(input)

	found := false
	for _, d := range diags {
		if d.Rule == "test-number-sequence" {
			found = true
		}
	}
	if !found {
		t.Error("expected test-number-sequence warning")
	}
}

func TestReaderWriteTo(t *testing.T) {
	input := "TAP version 14\n1..1\nok 1 - pass\n"
	r := NewReader(strings.NewReader(input))
	var buf strings.Builder
	n, err := r.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo error: %v", err)
	}
	if n == 0 {
		t.Error("expected non-zero bytes written")
	}
	out := buf.String()
	if !strings.Contains(out, "valid") {
		t.Errorf("expected 'valid' in output, got: %q", out)
	}
}

func TestReaderWriteToWithErrors(t *testing.T) {
	input := "1..1\nok 1 - test\n"
	r := NewReader(strings.NewReader(input))
	var buf strings.Builder
	r.WriteTo(&buf)
	out := buf.String()
	if !strings.Contains(out, "version-required") {
		t.Errorf("expected version-required in output, got: %q", out)
	}
}

func TestReaderSubtest(t *testing.T) {
	input := "TAP version 14\n1..1\n    # Subtest: nested\n    ok 1 - inner pass\n    1..1\nok 1 - nested\n"
	_, diags, summary := collectEvents(input)

	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error: %s: %s", d.Rule, d.Message)
		}
	}
	if !summary.Valid {
		t.Error("expected Valid=true for valid subtest")
	}
}

func TestReaderNestedSubtest(t *testing.T) {
	input := "TAP version 14\n1..1\n    # Subtest: outer\n        # Subtest: inner\n        ok 1 - deep\n        1..1\n    ok 1 - inner result\n    1..1\nok 1 - outer result\n"
	_, diags, summary := collectEvents(input)

	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error: %s: %s", d.Rule, d.Message)
		}
	}
	if !summary.Valid {
		t.Error("expected Valid=true for nested subtests")
	}
}

func TestReaderSubtestPlanMismatch(t *testing.T) {
	input := "TAP version 14\n1..1\n    ok 1 - inner\n    1..3\nok 1 - outer\n"
	_, diags, _ := collectEvents(input)

	found := false
	for _, d := range diags {
		if d.Rule == "plan-count-mismatch" {
			found = true
		}
	}
	if !found {
		t.Error("expected plan-count-mismatch for subtest")
	}
}

func TestReaderSkipAllPlan(t *testing.T) {
	input := "TAP version 14\n1..0 # skip all tests\n"
	_, diags, summary := collectEvents(input)

	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error: %s: %s", d.Rule, d.Message)
		}
	}
	if !summary.Valid {
		t.Error("expected Valid=true for skip-all plan")
	}
}

func TestReaderANSIColoredStream(t *testing.T) {
	// A TAP stream with ANSI color codes around status keywords and directives.
	input := "TAP version 14\n" +
		"1..3\n" +
		"\033[32mok\033[0m 1 - passing test\n" +
		"\033[31mnot ok\033[0m 2 - failing test\n" +
		"\033[32mok\033[0m 3 - skipped \033[33m# SKIP\033[0m not needed\n"

	events, diags, summary := collectEvents(input)

	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error diagnostic: %s: %s", d.Rule, d.Message)
		}
	}
	if !summary.Valid {
		t.Errorf("expected Valid=true for colored stream")
	}
	if summary.TotalTests != 3 {
		t.Errorf("expected 3 total tests, got %d", summary.TotalTests)
	}
	if summary.Passed != 1 {
		t.Errorf("expected 1 passed, got %d", summary.Passed)
	}
	if summary.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", summary.Failed)
	}
	if summary.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", summary.Skipped)
	}

	// Verify the events parsed correctly
	var tpEvents []Event
	for _, ev := range events {
		if ev.Type == EventTestPoint {
			tpEvents = append(tpEvents, ev)
		}
	}
	if len(tpEvents) != 3 {
		t.Fatalf("expected 3 test point events, got %d", len(tpEvents))
	}
	if !tpEvents[0].TestPoint.OK {
		t.Error("test 1 should be ok")
	}
	if tpEvents[1].TestPoint.OK {
		t.Error("test 2 should be not ok")
	}
	if tpEvents[2].TestPoint.Directive != DirectiveSkip {
		t.Error("test 3 should have SKIP directive")
	}
}

func TestReaderANSIBailOut(t *testing.T) {
	input := "TAP version 14\n" +
		"1..3\n" +
		"\033[32mok\033[0m 1 - a\n" +
		"\033[31mBail out!\033[0m database down\n"

	_, _, summary := collectEvents(input)

	if !summary.BailedOut {
		t.Error("expected BailedOut=true for colored bail out")
	}
	if !summary.Valid {
		t.Error("expected Valid=true when bailed out")
	}
}

func TestReaderStreamedOutputPragma(t *testing.T) {
	input := "TAP version 14\npragma +streamed-output\n1..1\n# compiling\n# linking\nok 1 - build\n"
	events, diags, summary := collectEvents(input)

	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error: %s: %s", d.Rule, d.Message)
		}
	}
	if !summary.Valid {
		t.Error("expected Valid=true")
	}

	var commentEvents []Event
	for _, ev := range events {
		if ev.Type == EventComment {
			commentEvents = append(commentEvents, ev)
		}
	}
	if len(commentEvents) != 2 {
		t.Fatalf("expected 2 comment events, got %d", len(commentEvents))
	}
	for _, ev := range commentEvents {
		if !ev.StreamedOutput {
			t.Errorf("expected StreamedOutput=true for comment %q", ev.Comment)
		}
	}
}

func TestReaderStreamedOutputDeactivation(t *testing.T) {
	input := "TAP version 14\npragma +streamed-output\npragma -streamed-output\n1..1\nok 1 - a\n"
	_, diags, _ := collectEvents(input)

	found := false
	for _, d := range diags {
		if d.Rule == "streamed-output-deactivation" {
			found = true
		}
	}
	if !found {
		t.Error("expected streamed-output-deactivation diagnostic")
	}
}

func TestReaderStreamedOutputSubtestIsolation(t *testing.T) {
	input := "TAP version 14\npragma +streamed-output\n1..1\n# parent comment\n    # Subtest: child\n    # child comment\n    ok 1 - inner\n    1..1\nok 1 - child\n"
	events, diags, summary := collectEvents(input)

	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error: %s: %s", d.Rule, d.Message)
		}
	}
	if !summary.Valid {
		t.Error("expected Valid=true")
	}

	for _, ev := range events {
		if ev.Type == EventComment && ev.Comment == "parent comment" {
			if !ev.StreamedOutput {
				t.Error("expected parent comment to have StreamedOutput=true")
			}
		}
		if ev.Type == EventComment && ev.Comment == "child comment" {
			if ev.StreamedOutput {
				t.Error("expected child comment to have StreamedOutput=false (subtest has no pragma)")
			}
		}
	}
}

func TestReaderStreamedOutputNotActiveByDefault(t *testing.T) {
	input := "TAP version 14\n1..1\n# just a comment\nok 1 - a\n"
	events, _, _ := collectEvents(input)

	for _, ev := range events {
		if ev.Type == EventComment && ev.StreamedOutput {
			t.Error("expected StreamedOutput=false without pragma")
		}
	}
}

func TestReaderUnclosedYAML(t *testing.T) {
	input := "TAP version 14\n1..1\nnot ok 1 - fail\n  ---\n  message: broken\n"
	_, diags, _ := collectEvents(input)

	found := false
	for _, d := range diags {
		if d.Rule == "yaml-unclosed" {
			found = true
		}
	}
	if !found {
		t.Error("expected yaml-unclosed diagnostic")
	}
}

func TestReaderLocaleFormattedPlan(t *testing.T) {
	input := "TAP version 14\npragma +locale-formatting:en-US\n1..1,200\nok 1 - first\nok 2 - second\n"
	_, _, summary := collectEvents(input)
	if summary.PlanCount != 1200 {
		t.Errorf("expected plan count 1200, got %d", summary.PlanCount)
	}
}

func TestReaderLocaleFormattedTestPoint(t *testing.T) {
	input := "TAP version 14\npragma +locale-formatting:en-US\n1..2\nok 1 - first\nok 1,234 - big\n"
	events, _, _ := collectEvents(input)
	for _, ev := range events {
		if ev.Type == EventTestPoint && ev.TestPoint.Number == 1234 {
			return
		}
	}
	t.Error("expected test point with number 1234 parsed from '1,234'")
}

func TestReaderLocaleGermanPlan(t *testing.T) {
	input := "TAP version 14\npragma +locale-formatting:de-DE\n1..1.200\nok 1 - test\n"
	_, _, summary := collectEvents(input)
	if summary.PlanCount != 1200 {
		t.Errorf("expected plan count 1200 from German format, got %d", summary.PlanCount)
	}
}

func TestReaderLocaleFrenchPlan(t *testing.T) {
	// fr-FR uses non-breaking space (U+00A0) as grouping separator in x/text
	input := "TAP version 14\npragma +locale-formatting:fr-FR\n1..1\u00a0200\nok 1 - test\n"
	_, _, summary := collectEvents(input)
	if summary.PlanCount != 1200 {
		t.Errorf("expected plan count 1200 from French format, got %d", summary.PlanCount)
	}
}

func TestReaderLocaleFormattingSubtestScoping(t *testing.T) {
	// Subtest without its own pragma should NOT use locale parsing
	input := "TAP version 14\npragma +locale-formatting:en-US\n1..1\n" +
		"    # Subtest: child\n    1..1\n    ok 1 - inner\n" +
		"ok 1 - child\n"
	_, diags, summary := collectEvents(input)
	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error: %s: %s", d.Rule, d.Message)
		}
	}
	if !summary.Valid {
		t.Error("expected Valid=true")
	}
}

func TestReaderNoLocaleRejectsFormattedNumbers(t *testing.T) {
	// Without locale pragma, comma in plan should cause parse issues
	input := "TAP version 14\n1..1,200\nok 1 - test\n"
	_, _, summary := collectEvents(input)
	if summary.PlanCount == 1200 {
		t.Error("expected plan NOT to parse as 1200 without locale pragma")
	}
}

func TestReaderYAMLPreservesANSI(t *testing.T) {
	input := "TAP version 14\n1..1\nnot ok 1 - fail\n  ---\n  message: \033[31merror\033[0m text\n  ...\n"
	events, diags, _ := collectEvents(input)

	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error: %s: %s", d.Rule, d.Message)
		}
	}

	for _, ev := range events {
		if ev.Type == EventYAMLDiagnostic {
			msg := ev.YAML["message"]
			if msg != "\033[31merror\033[0m text" {
				t.Errorf("expected ANSI preserved in YAML value, got %q", msg)
			}
			return
		}
	}
	t.Error("expected YAML diagnostic event")
}

func TestReaderYAMLStripsANSIFromProtocolButNotContent(t *testing.T) {
	// ANSI on protocol lines (test point) is stripped for classification,
	// but ANSI in YAML values is preserved.
	input := "TAP version 14\n1..1\n\033[31mnot ok\033[0m 1 - fail\n  ---\n  output: \033[33mwarning\033[0m here\n  ...\n"
	events, diags, summary := collectEvents(input)

	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Errorf("unexpected error: %s: %s", d.Rule, d.Message)
		}
	}
	if !summary.Valid {
		t.Error("expected valid TAP")
	}

	for _, ev := range events {
		if ev.Type == EventTestPoint {
			if ev.TestPoint.OK {
				t.Error("expected not ok test point")
			}
		}
		if ev.Type == EventYAMLDiagnostic {
			output := ev.YAML["output"]
			if output != "\033[33mwarning\033[0m here" {
				t.Errorf("expected ANSI preserved in YAML output value, got %q", output)
			}
		}
	}
}

func TestReaderLocaleRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	tw := NewLocaleWriter(&buf, language.MustParse("en-US"))
	for i := 0; i < 1234; i++ {
		tw.Ok(fmt.Sprintf("test %d", i+1))
	}
	tw.Plan()

	reader := NewReader(strings.NewReader(buf.String()))
	summary := reader.Summary()
	if !summary.Valid {
		diags := reader.Diagnostics()
		for _, d := range diags {
			t.Errorf("diagnostic: line %d: %s: %s", d.Line, d.Severity, d.Message)
		}
		t.Fatalf("locale-formatted writer output did not validate")
	}
	if summary.TotalTests != 1234 {
		t.Errorf("expected 1234 tests, got %d", summary.TotalTests)
	}
	if summary.PlanCount != 1234 {
		t.Errorf("expected plan count 1234, got %d", summary.PlanCount)
	}
}
