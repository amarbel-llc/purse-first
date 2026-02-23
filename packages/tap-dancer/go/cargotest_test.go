package tap

import (
	"bytes"
	"strings"
	"testing"
)

func TestCargoConvertSingleSuiteAllPass(t *testing.T) {
	jsonEvents := strings.Join([]string{
		`{ "type": "suite", "event": "started", "test_count": 2 }`,
		`{ "type": "test", "event": "started", "name": "tests::test_a" }`,
		`{ "type": "test", "name": "tests::test_a", "event": "ok", "exec_time": 0.001, "stdout": "" }`,
		`{ "type": "test", "event": "started", "name": "tests::test_b" }`,
		`{ "type": "test", "name": "tests::test_b", "event": "ok", "exec_time": 0.002, "stdout": "" }`,
		`{ "type": "suite", "event": "ok", "passed": 2, "failed": 0, "ignored": 0, "measured": 0, "filtered_out": 0, "exec_time": 0.005 }`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := ConvertCargoTest(strings.NewReader(jsonEvents), &buf, false, false)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	out := buf.String()

	// Should produce valid TAP-14
	reader := NewReader(strings.NewReader(out))
	summary := reader.Summary()
	if !summary.Valid {
		for _, d := range reader.Diagnostics() {
			t.Errorf("diagnostic: line %d: %s: %s", d.Line, d.Severity, d.Message)
		}
		t.Fatalf("output is not valid TAP-14:\n%s", out)
	}

	// Should have a subtest for the suite
	if !strings.Contains(out, "# Subtest:") {
		t.Errorf("expected suite subtest:\n%s", out)
	}
	if !strings.Contains(out, "tests::test_a") {
		t.Errorf("expected test_a in output:\n%s", out)
	}
	if !strings.Contains(out, "tests::test_b") {
		t.Errorf("expected test_b in output:\n%s", out)
	}
}
