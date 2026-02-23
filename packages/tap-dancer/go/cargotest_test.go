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

func TestCargoConvertFailingTest(t *testing.T) {
	jsonEvents := strings.Join([]string{
		`{ "type": "suite", "event": "started", "test_count": 1 }`,
		`{ "type": "test", "event": "started", "name": "tests::test_bad" }`,
		`{ "type": "test", "name": "tests::test_bad", "event": "failed", "exec_time": 0.003, "stdout": "thread 'tests::test_bad' panicked at src/lib.rs:10:5:\nassertion ` + "`" + `left == right` + "`" + ` failed\n  left: 1\n right: 2" }`,
		`{ "type": "suite", "event": "failed", "passed": 0, "failed": 1, "ignored": 0, "measured": 0, "filtered_out": 0, "exec_time": 0.010 }`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := ConvertCargoTest(strings.NewReader(jsonEvents), &buf, false, false)

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}

	out := buf.String()
	if !strings.Contains(out, "not ok") {
		t.Errorf("expected not ok in output:\n%s", out)
	}
	if !strings.Contains(out, "lib.rs") {
		t.Errorf("expected file reference in diagnostics:\n%s", out)
	}

	reader := NewReader(strings.NewReader(out))
	if !reader.Summary().Valid {
		t.Errorf("output is not valid TAP-14:\n%s", out)
	}
}

func TestCargoConvertIgnoredTest(t *testing.T) {
	jsonEvents := strings.Join([]string{
		`{ "type": "suite", "event": "started", "test_count": 2 }`,
		`{ "type": "test", "event": "started", "name": "tests::test_ok" }`,
		`{ "type": "test", "name": "tests::test_ok", "event": "ok", "exec_time": 0.001, "stdout": "" }`,
		`{ "type": "test", "event": "started", "name": "tests::test_ignored" }`,
		`{ "type": "test", "name": "tests::test_ignored", "event": "ignored" }`,
		`{ "type": "suite", "event": "ok", "passed": 1, "failed": 0, "ignored": 1, "measured": 0, "filtered_out": 0, "exec_time": 0.005 }`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := ConvertCargoTest(strings.NewReader(jsonEvents), &buf, false, false)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	out := buf.String()
	if !strings.Contains(out, "# SKIP") {
		t.Errorf("expected SKIP directive for ignored test:\n%s", out)
	}

	reader := NewReader(strings.NewReader(out))
	if !reader.Summary().Valid {
		t.Errorf("output is not valid TAP-14:\n%s", out)
	}
}

func TestCargoConvertMultipleSuites(t *testing.T) {
	jsonEvents := strings.Join([]string{
		`Running unittests src/lib.rs (target/debug/deps/my_crate-abc123)`,
		`{ "type": "suite", "event": "started", "test_count": 1 }`,
		`{ "type": "test", "event": "started", "name": "tests::test_lib" }`,
		`{ "type": "test", "name": "tests::test_lib", "event": "ok", "exec_time": 0.001, "stdout": "" }`,
		`{ "type": "suite", "event": "ok", "passed": 1, "failed": 0, "ignored": 0, "measured": 0, "filtered_out": 0, "exec_time": 0.003 }`,
		`Running tests/integration.rs (target/debug/deps/integration-def456)`,
		`{ "type": "suite", "event": "started", "test_count": 1 }`,
		`{ "type": "test", "event": "started", "name": "test_integration" }`,
		`{ "type": "test", "name": "test_integration", "event": "ok", "exec_time": 0.002, "stdout": "" }`,
		`{ "type": "suite", "event": "ok", "passed": 1, "failed": 0, "ignored": 0, "measured": 0, "filtered_out": 0, "exec_time": 0.004 }`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := ConvertCargoTest(strings.NewReader(jsonEvents), &buf, false, false)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	out := buf.String()
	if !strings.Contains(out, "# Subtest: unittests src/lib.rs") {
		t.Errorf("expected lib suite subtest:\n%s", out)
	}
	if !strings.Contains(out, "# Subtest: tests/integration.rs") {
		t.Errorf("expected integration suite subtest:\n%s", out)
	}
	if !strings.Contains(out, "1..2") {
		t.Errorf("expected plan 1..2:\n%s", out)
	}

	reader := NewReader(strings.NewReader(out))
	if !reader.Summary().Valid {
		for _, d := range reader.Diagnostics() {
			t.Errorf("diagnostic: line %d: %s: %s", d.Line, d.Severity, d.Message)
		}
		t.Fatalf("output is not valid TAP-14:\n%s", out)
	}
}
