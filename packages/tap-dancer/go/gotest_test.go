package tap

import (
	"bytes"
	"strings"
	"testing"
)

func TestConvertSinglePackageAllPass(t *testing.T) {
	jsonEvents := strings.Join([]string{
		`{"Action":"start","Package":"example.com/foo"}`,
		`{"Action":"run","Package":"example.com/foo","Test":"TestA"}`,
		`{"Action":"output","Package":"example.com/foo","Test":"TestA","Output":"=== RUN   TestA\n"}`,
		`{"Action":"pass","Package":"example.com/foo","Test":"TestA","Elapsed":0.001}`,
		`{"Action":"run","Package":"example.com/foo","Test":"TestB"}`,
		`{"Action":"output","Package":"example.com/foo","Test":"TestB","Output":"=== RUN   TestB\n"}`,
		`{"Action":"pass","Package":"example.com/foo","Test":"TestB","Elapsed":0.002}`,
		`{"Action":"output","Package":"example.com/foo","Output":"PASS\n"}`,
		`{"Action":"pass","Package":"example.com/foo","Elapsed":0.010}`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := ConvertGoTest(strings.NewReader(jsonEvents), &buf, false, false, false)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	out := buf.String()

	// Validate the output is valid TAP-14
	reader := NewReader(strings.NewReader(out))
	summary := reader.Summary()
	if !summary.Valid {
		for _, d := range reader.Diagnostics() {
			t.Errorf("diagnostic: line %d: %s: %s", d.Line, d.Severity, d.Message)
		}
		t.Fatalf("output is not valid TAP-14:\n%s", out)
	}

	// Should have package as subtest with 2 inner tests
	if !strings.Contains(out, "# Subtest: example.com/foo") {
		t.Errorf("expected package subtest, got:\n%s", out)
	}
	if !strings.Contains(out, "ok 1 - example.com/foo") {
		t.Errorf("expected parent ok for package, got:\n%s", out)
	}
}

func TestConvertFailingTest(t *testing.T) {
	jsonEvents := strings.Join([]string{
		`{"Action":"run","Package":"example.com/foo","Test":"TestBad"}`,
		`{"Action":"output","Package":"example.com/foo","Test":"TestBad","Output":"=== RUN   TestBad\n"}`,
		`{"Action":"output","Package":"example.com/foo","Test":"TestBad","Output":"    foo_test.go:10: expected 1, got 2\n"}`,
		`{"Action":"fail","Package":"example.com/foo","Test":"TestBad","Elapsed":0.003}`,
		`{"Action":"output","Package":"example.com/foo","Output":"FAIL\n"}`,
		`{"Action":"fail","Package":"example.com/foo","Elapsed":0.010}`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := ConvertGoTest(strings.NewReader(jsonEvents), &buf, false, false, false)

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}

	out := buf.String()
	if !strings.Contains(out, "not ok") {
		t.Errorf("expected not ok in output:\n%s", out)
	}
	if !strings.Contains(out, "foo_test.go") {
		t.Errorf("expected file reference in diagnostics:\n%s", out)
	}

	reader := NewReader(strings.NewReader(out))
	if !reader.Summary().Valid {
		t.Errorf("output is not valid TAP-14:\n%s", out)
	}
}

func TestConvertSkippedTest(t *testing.T) {
	jsonEvents := strings.Join([]string{
		`{"Action":"run","Package":"example.com/foo","Test":"TestSkip"}`,
		`{"Action":"output","Package":"example.com/foo","Test":"TestSkip","Output":"=== RUN   TestSkip\n"}`,
		`{"Action":"output","Package":"example.com/foo","Test":"TestSkip","Output":"    foo_test.go:5: not applicable\n"}`,
		`{"Action":"output","Package":"example.com/foo","Test":"TestSkip","Output":"--- SKIP: TestSkip (0.00s)\n"}`,
		`{"Action":"skip","Package":"example.com/foo","Test":"TestSkip","Elapsed":0.0}`,
		`{"Action":"output","Package":"example.com/foo","Output":"PASS\n"}`,
		`{"Action":"pass","Package":"example.com/foo","Elapsed":0.005}`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := ConvertGoTest(strings.NewReader(jsonEvents), &buf, false, false, false)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	out := buf.String()
	if !strings.Contains(out, "# SKIP") {
		t.Errorf("expected SKIP directive:\n%s", out)
	}

	reader := NewReader(strings.NewReader(out))
	if !reader.Summary().Valid {
		t.Errorf("output is not valid TAP-14:\n%s", out)
	}
}

func TestConvertSubtests(t *testing.T) {
	jsonEvents := strings.Join([]string{
		`{"Action":"run","Package":"example.com/foo","Test":"TestParent"}`,
		`{"Action":"output","Package":"example.com/foo","Test":"TestParent","Output":"=== RUN   TestParent\n"}`,
		`{"Action":"run","Package":"example.com/foo","Test":"TestParent/child_a"}`,
		`{"Action":"output","Package":"example.com/foo","Test":"TestParent/child_a","Output":"=== RUN   TestParent/child_a\n"}`,
		`{"Action":"pass","Package":"example.com/foo","Test":"TestParent/child_a","Elapsed":0.001}`,
		`{"Action":"run","Package":"example.com/foo","Test":"TestParent/child_b"}`,
		`{"Action":"output","Package":"example.com/foo","Test":"TestParent/child_b","Output":"=== RUN   TestParent/child_b\n"}`,
		`{"Action":"pass","Package":"example.com/foo","Test":"TestParent/child_b","Elapsed":0.001}`,
		`{"Action":"pass","Package":"example.com/foo","Test":"TestParent","Elapsed":0.003}`,
		`{"Action":"output","Package":"example.com/foo","Output":"PASS\n"}`,
		`{"Action":"pass","Package":"example.com/foo","Elapsed":0.010}`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := ConvertGoTest(strings.NewReader(jsonEvents), &buf, false, false, false)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	out := buf.String()

	// Should have nested subtest for TestParent
	if !strings.Contains(out, "# Subtest: TestParent") {
		t.Errorf("expected TestParent subtest:\n%s", out)
	}
	if !strings.Contains(out, "child_a") {
		t.Errorf("expected child_a in output:\n%s", out)
	}

	reader := NewReader(strings.NewReader(out))
	if !reader.Summary().Valid {
		for _, d := range reader.Diagnostics() {
			t.Errorf("diagnostic: line %d: %s: %s", d.Line, d.Severity, d.Message)
		}
		t.Fatalf("output is not valid TAP-14:\n%s", out)
	}
}

func TestConvertMultiplePackages(t *testing.T) {
	jsonEvents := strings.Join([]string{
		`{"Action":"run","Package":"example.com/foo","Test":"TestFoo"}`,
		`{"Action":"run","Package":"example.com/bar","Test":"TestBar"}`,
		`{"Action":"output","Package":"example.com/foo","Test":"TestFoo","Output":"=== RUN   TestFoo\n"}`,
		`{"Action":"output","Package":"example.com/bar","Test":"TestBar","Output":"=== RUN   TestBar\n"}`,
		`{"Action":"pass","Package":"example.com/foo","Test":"TestFoo","Elapsed":0.001}`,
		`{"Action":"pass","Package":"example.com/bar","Test":"TestBar","Elapsed":0.002}`,
		`{"Action":"output","Package":"example.com/foo","Output":"PASS\n"}`,
		`{"Action":"pass","Package":"example.com/foo","Elapsed":0.005}`,
		`{"Action":"output","Package":"example.com/bar","Output":"PASS\n"}`,
		`{"Action":"pass","Package":"example.com/bar","Elapsed":0.006}`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := ConvertGoTest(strings.NewReader(jsonEvents), &buf, false, false, false)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	out := buf.String()
	if !strings.Contains(out, "# Subtest: example.com/foo") {
		t.Errorf("expected foo package subtest:\n%s", out)
	}
	if !strings.Contains(out, "# Subtest: example.com/bar") {
		t.Errorf("expected bar package subtest:\n%s", out)
	}
	if !strings.Contains(out, "1..2") {
		t.Errorf("expected plan 1..2:\n%s", out)
	}

	reader := NewReader(strings.NewReader(out))
	if !reader.Summary().Valid {
		t.Fatalf("output is not valid TAP-14:\n%s", out)
	}
}

func TestConvertNoTestFilesDefault(t *testing.T) {
	// Package with no test files — go test -json emits skip at package level.
	// Without -skip-empty, this should produce not ok.
	jsonEvents := strings.Join([]string{
		`{"Action":"output","Package":"example.com/empty","Output":"?   \texample.com/empty\t[no test files]\n"}`,
		`{"Action":"skip","Package":"example.com/empty","Elapsed":0}`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := ConvertGoTest(strings.NewReader(jsonEvents), &buf, false, false, false)

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}

	out := buf.String()
	if !strings.Contains(out, "not ok") {
		t.Errorf("expected not ok for empty package:\n%s", out)
	}

	reader := NewReader(strings.NewReader(out))
	if !reader.Summary().Valid {
		for _, d := range reader.Diagnostics() {
			t.Errorf("diagnostic: line %d: %s: %s", d.Line, d.Severity, d.Message)
		}
		t.Fatalf("output is not valid TAP-14:\n%s", out)
	}
}

func TestConvertNoTestFilesSkipEmpty(t *testing.T) {
	// Same as above but with skipEmpty=true — should produce # SKIP.
	jsonEvents := strings.Join([]string{
		`{"Action":"output","Package":"example.com/empty","Output":"?   \texample.com/empty\t[no test files]\n"}`,
		`{"Action":"skip","Package":"example.com/empty","Elapsed":0}`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := ConvertGoTest(strings.NewReader(jsonEvents), &buf, false, true, false)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	out := buf.String()
	if !strings.Contains(out, "# SKIP") {
		t.Errorf("expected SKIP directive:\n%s", out)
	}
	if strings.Contains(out, "not ok") {
		t.Errorf("expected no 'not ok' with skip-empty:\n%s", out)
	}

	reader := NewReader(strings.NewReader(out))
	if !reader.Summary().Valid {
		for _, d := range reader.Diagnostics() {
			t.Errorf("diagnostic: line %d: %s: %s", d.Line, d.Severity, d.Message)
		}
		t.Fatalf("output is not valid TAP-14:\n%s", out)
	}
}

func TestConvertNoTestsToRunDefault(t *testing.T) {
	// Package with test files but no Test* functions — go test emits pass
	// with 0 tests. Without -skip-empty, this should produce not ok.
	jsonEvents := strings.Join([]string{
		`{"Action":"output","Package":"example.com/notest","Output":"testing: warning: no tests to run\n"}`,
		`{"Action":"output","Package":"example.com/notest","Output":"PASS\n"}`,
		`{"Action":"pass","Package":"example.com/notest","Elapsed":0.001}`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := ConvertGoTest(strings.NewReader(jsonEvents), &buf, false, false, false)

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}

	out := buf.String()
	if !strings.Contains(out, "not ok") {
		t.Errorf("expected not ok for empty package:\n%s", out)
	}

	reader := NewReader(strings.NewReader(out))
	if !reader.Summary().Valid {
		for _, d := range reader.Diagnostics() {
			t.Errorf("diagnostic: line %d: %s: %s", d.Line, d.Severity, d.Message)
		}
		t.Fatalf("output is not valid TAP-14:\n%s", out)
	}
}

func TestConvertNoTestsToRunSkipEmpty(t *testing.T) {
	// Same as above but with skipEmpty=true — should produce # SKIP.
	jsonEvents := strings.Join([]string{
		`{"Action":"output","Package":"example.com/notest","Output":"testing: warning: no tests to run\n"}`,
		`{"Action":"output","Package":"example.com/notest","Output":"PASS\n"}`,
		`{"Action":"pass","Package":"example.com/notest","Elapsed":0.001}`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := ConvertGoTest(strings.NewReader(jsonEvents), &buf, false, true, false)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	out := buf.String()
	if !strings.Contains(out, "# SKIP") {
		t.Errorf("expected SKIP directive:\n%s", out)
	}
	if strings.Contains(out, "not ok") {
		t.Errorf("expected no 'not ok' with skip-empty:\n%s", out)
	}

	reader := NewReader(strings.NewReader(out))
	if !reader.Summary().Valid {
		for _, d := range reader.Diagnostics() {
			t.Errorf("diagnostic: line %d: %s: %s", d.Line, d.Severity, d.Message)
		}
		t.Fatalf("output is not valid TAP-14:\n%s", out)
	}
}

func TestConvertEmitsPragmaAndStreamedOutput(t *testing.T) {
	jsonEvents := strings.Join([]string{
		`{"Action":"run","Package":"example.com/foo","Test":"TestBad"}`,
		`{"Action":"output","Package":"example.com/foo","Test":"TestBad","Output":"=== RUN   TestBad\n"}`,
		`{"Action":"output","Package":"example.com/foo","Test":"TestBad","Output":"    foo_test.go:10: expected 1, got 2\n"}`,
		`{"Action":"fail","Package":"example.com/foo","Test":"TestBad","Elapsed":0.003}`,
		`{"Action":"output","Package":"example.com/foo","Output":"FAIL\n"}`,
		`{"Action":"fail","Package":"example.com/foo","Elapsed":0.010}`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	ConvertGoTest(strings.NewReader(jsonEvents), &buf, false, false, false)
	out := buf.String()

	if !strings.Contains(out, "pragma +streamed-output") {
		t.Errorf("expected pragma +streamed-output in output:\n%s", out)
	}

	// Streamed output comments should appear before the not ok line
	pragmaIdx := strings.Index(out, "pragma +streamed-output")
	commentIdx := strings.Index(out, "# foo_test.go:10: expected 1, got 2")
	notOkIdx := strings.Index(out, "not ok")
	if commentIdx < 0 {
		t.Fatalf("expected streamed output comment in output:\n%s", out)
	}
	if commentIdx < pragmaIdx {
		t.Error("streamed output comment should appear after pragma")
	}
	if commentIdx > notOkIdx {
		t.Error("streamed output comment should appear before not ok")
	}

	reader := NewReader(strings.NewReader(out))
	if !reader.Summary().Valid {
		for _, d := range reader.Diagnostics() {
			t.Errorf("diagnostic: line %d: %s: %s", d.Line, d.Severity, d.Message)
		}
		t.Fatalf("output is not valid TAP-14:\n%s", out)
	}
}

func TestConvertMixedEmptyAndRealPackages(t *testing.T) {
	// One package with tests, one with no test files.
	// With skipEmpty, the empty one is skipped, the real one passes.
	jsonEvents := strings.Join([]string{
		`{"Action":"run","Package":"example.com/real","Test":"TestReal"}`,
		`{"Action":"output","Package":"example.com/real","Test":"TestReal","Output":"=== RUN   TestReal\n"}`,
		`{"Action":"pass","Package":"example.com/real","Test":"TestReal","Elapsed":0.001}`,
		`{"Action":"output","Package":"example.com/real","Output":"PASS\n"}`,
		`{"Action":"pass","Package":"example.com/real","Elapsed":0.005}`,
		`{"Action":"output","Package":"example.com/empty","Output":"?   \texample.com/empty\t[no test files]\n"}`,
		`{"Action":"skip","Package":"example.com/empty","Elapsed":0}`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := ConvertGoTest(strings.NewReader(jsonEvents), &buf, false, true, false)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}

	out := buf.String()
	if !strings.Contains(out, "# Subtest: example.com/real") {
		t.Errorf("expected real package subtest:\n%s", out)
	}
	if !strings.Contains(out, "# SKIP") {
		t.Errorf("expected SKIP for empty package:\n%s", out)
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
