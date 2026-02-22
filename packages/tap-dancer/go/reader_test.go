package tap

import (
	"io"
	"strings"
	"testing"
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
