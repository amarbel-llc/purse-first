package tap

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/operation"
)

func TestOperationWriterLeafSuccess(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	ow := NewOperationWriter(tw)

	ow.BeginOperation(1, &operation.OperationEvent{Description: "step"})
	ow.EndOperation(1, &operation.OperationEvent{
		Description: "step",
		Outcome:     operation.Success,
	})
	tw.Plan()

	out := buf.String()
	if !strings.Contains(out, "ok 1 - step") {
		t.Errorf("expected ok line, got:\n%s", out)
	}
}

func TestOperationWriterLeafFailure(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	ow := NewOperationWriter(tw)

	ow.BeginOperation(1, &operation.OperationEvent{Description: "step"})
	ow.EndOperation(1, &operation.OperationEvent{
		Description: "step",
		Outcome:     operation.Failure,
		Diagnostic: &operation.Diagnostic{
			File:     "main.go",
			Line:     42,
			Message:  "broken",
			Severity: "error",
		},
	})
	tw.Plan()

	out := buf.String()
	if !strings.Contains(out, "not ok 1 - step") {
		t.Errorf("expected not ok line, got:\n%s", out)
	}
	if !strings.Contains(out, "file: main.go") {
		t.Errorf("expected file diagnostic, got:\n%s", out)
	}
	if !strings.Contains(out, "message: broken") {
		t.Errorf("expected message diagnostic, got:\n%s", out)
	}
}

func TestOperationWriterSkip(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	ow := NewOperationWriter(tw)

	ow.BeginOperation(1, &operation.OperationEvent{Description: "step"})
	ow.EndOperation(1, &operation.OperationEvent{
		Description: "step",
		Outcome:     operation.Skipped,
		Diagnostic:  &operation.Diagnostic{Message: "not ready"},
	})
	tw.Plan()

	out := buf.String()
	if !strings.Contains(out, "# SKIP not ready") {
		t.Errorf("expected skip directive, got:\n%s", out)
	}
}

func TestOperationWriterNested(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	ow := NewOperationWriter(tw)

	// Parent begins
	ow.BeginOperation(1, &operation.OperationEvent{Description: "parent"})
	// Child
	ow.BeginOperation(2, &operation.OperationEvent{Description: "child"})
	ow.EndOperation(2, &operation.OperationEvent{
		Description: "child",
		Outcome:     operation.Success,
	})
	// Parent ends
	ow.EndOperation(1, &operation.OperationEvent{
		Description: "parent",
		Outcome:     operation.Success,
	})
	tw.Plan()

	out := buf.String()
	if !strings.Contains(out, "# Subtest: parent") {
		t.Errorf("expected subtest header, got:\n%s", out)
	}
	if !strings.Contains(out, "ok 1 - child") {
		t.Errorf("expected child ok line, got:\n%s", out)
	}
	if !strings.Contains(out, "ok 1 - parent") {
		t.Errorf("expected parent ok line, got:\n%s", out)
	}
}

func TestOperationWriterExternalSource(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	ow := NewOperationWriter(tw)

	ow.BeginOperation(1, &operation.OperationEvent{Description: "step"})
	ow.EndOperation(1, &operation.OperationEvent{
		Description: "step",
		Outcome:     operation.Failure,
		Diagnostic: &operation.Diagnostic{
			File:     "main.go",
			Line:     10,
			Message:  "db error",
			Severity: "error",
			Source:   "external",
		},
	})
	tw.Plan()

	out := buf.String()
	if !strings.Contains(out, "source: external") {
		t.Errorf("expected source: external, got:\n%s", out)
	}
}

func TestOperationWriterMustErrors(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	ow := NewOperationWriter(tw)

	ow.BeginOperation(1, &operation.OperationEvent{Description: "step"})
	ow.EndOperation(1, &operation.OperationEvent{
		Description: "step",
		Outcome:     operation.Failure,
		MustErrors:  []error{errors.New("lock release failed")},
	})
	tw.Plan()

	out := buf.String()
	if !strings.Contains(out, "lock release failed") {
		t.Errorf("expected must error in output, got:\n%s", out)
	}
}

func TestOperationWriterAborted(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	ow := NewOperationWriter(tw)

	ow.BeginOperation(1, &operation.OperationEvent{Description: "step"})
	ow.EndOperation(1, &operation.OperationEvent{
		Description: "step",
		Outcome:     operation.Aborted,
		Diagnostic: &operation.Diagnostic{
			Message:  "fatal error",
			Severity: "error",
		},
	})
	tw.Plan()

	out := buf.String()
	if !strings.Contains(out, "not ok 1 - step") {
		t.Errorf("expected not ok line, got:\n%s", out)
	}
	if !strings.Contains(out, "aborted: true") {
		t.Errorf("expected aborted: true, got:\n%s", out)
	}
	if !strings.Contains(out, "message: fatal error") {
		t.Errorf("expected message diagnostic, got:\n%s", out)
	}
}

func TestOperationWriterSuccessWithDiagnostics(t *testing.T) {
	var buf bytes.Buffer
	tw := NewWriter(&buf)
	ow := NewOperationWriter(tw)

	ow.BeginOperation(1, &operation.OperationEvent{Description: "backup"})
	ow.EndOperation(1, &operation.OperationEvent{
		Description: "backup",
		Outcome:     operation.Success,
		Diagnostic:  &operation.Diagnostic{Extras: map[string]any{"size_mb": 420}},
	})
	tw.Plan()

	out := buf.String()
	if !strings.Contains(out, "ok 1 - backup") {
		t.Errorf("expected ok line, got:\n%s", out)
	}
	if !strings.Contains(out, "size_mb: 420") {
		t.Errorf("expected size_mb diagnostic, got:\n%s", out)
	}
}
