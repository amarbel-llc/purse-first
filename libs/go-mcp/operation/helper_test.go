package operation

import (
	"runtime"
	"strings"
	"testing"
)

func helperThatFails(ctx Context) {
	ctx.DiagHelper()
	ctx.ControlFail("failed in helper")
}

func TestDiagHelperSkipsFrame(t *testing.T) {
	w := &recordingWriter{}
	root := New(w)
	root.Run("step", func(ctx Context) error {
		helperThatFails(ctx)
		return nil
	})
	diag := w.ends[0].event.Diagnostic
	if diag == nil {
		t.Fatal("expected diagnostic")
	}
	if diag.File == "" {
		t.Error("expected file in diagnostic")
	}
	if diag.Line == 0 {
		t.Error("expected non-zero line in diagnostic")
	}
	if !strings.HasSuffix(diag.File, "helper_test.go") {
		t.Errorf("expected file to be helper_test.go, got %q", diag.File)
	}
	t.Logf("diagnostic file:line = %s:%d", diag.File, diag.Line)
}

func TestDiagHelperReportsCallSiteLine(t *testing.T) {
	w := &recordingWriter{}
	root := New(w)

	var expectedLine int
	root.Run("step", func(ctx Context) error {
		// runtime.Caller(0) and helperThatFails(ctx) MUST stay on the same
		// source line so expectedLine matches the diagnostic's reported
		// line. See note in the file header about treefmt exclusion.
		_, _, expectedLine, _ = runtime.Caller(0); helperThatFails(ctx)
		return nil
	})

	diag := w.ends[0].event.Diagnostic
	if diag == nil {
		t.Fatal("expected diagnostic")
	}

	if diag.Line != expectedLine {
		t.Errorf("expected diagnostic line %d (call site), got %d",
			expectedLine, diag.Line)
	}

	if !strings.HasSuffix(diag.File, "helper_test.go") {
		t.Errorf("expected file helper_test.go, got %q", diag.File)
	}

	t.Logf("diagnostic file:line = %s:%d (expected line %d)",
		diag.File, diag.Line, expectedLine)
}

func TestDiagHelperWithoutMarkerReportsInsideFunction(t *testing.T) {
	// When a function does NOT call DiagHelper, ControlFail should
	// report the line of the ControlFail call (no frame skipping).
	w := &recordingWriter{}
	root := New(w)

	var controlFailLine int
	root.Run("step", func(ctx Context) error {
		_, _, controlFailLine, _ = runtime.Caller(0); ctx.ControlFail("direct call")
		return nil
	})

	diag := w.ends[0].event.Diagnostic
	if diag == nil {
		t.Fatal("expected diagnostic")
	}

	if !strings.HasSuffix(diag.File, "helper_test.go") {
		t.Errorf("expected file helper_test.go, got %q", diag.File)
	}

	if diag.Line != controlFailLine {
		t.Errorf("expected line %d (ControlFail call), got %d",
			controlFailLine, diag.Line)
	}

	t.Logf("direct ControlFail: file:line = %s:%d", diag.File, diag.Line)
}

func nestedInnerHelper(ctx Context) {
	ctx.DiagHelper()
	ctx.ControlFail("nested failure")
}

func nestedOuterHelper(ctx Context) {
	ctx.DiagHelper()
	nestedInnerHelper(ctx)
}

func TestDiagHelperSkipsNestedHelpers(t *testing.T) {
	w := &recordingWriter{}
	root := New(w)

	var expectedLine int
	root.Run("step", func(ctx Context) error {
		_, _, expectedLine, _ = runtime.Caller(0); nestedOuterHelper(ctx)
		return nil
	})

	diag := w.ends[0].event.Diagnostic
	if diag == nil {
		t.Fatal("expected diagnostic")
	}

	if !strings.HasSuffix(diag.File, "helper_test.go") {
		t.Errorf("expected file to be helper_test.go, got %q", diag.File)
	}

	if diag.Line != expectedLine {
		t.Errorf("expected diagnostic line %d (call site), got %d",
			expectedLine, diag.Line)
	}

	t.Logf("nested helpers: file:line = %s:%d (expected line %d)",
		diag.File, diag.Line, expectedLine)
}

func TestDiagHelperInheritedByChild(t *testing.T) {
	w := &recordingWriter{}
	root := New(w)
	root.Run("parent", func(ctx Context) error {
		ctx.Run("child", func(ctx Context) error {
			helperThatFails(ctx)
			return nil
		})
		return nil
	})

	// ends[0] is the child (inner Run finishes first)
	childEnd := w.ends[0]
	if childEnd.event.Diagnostic == nil {
		t.Fatal("expected diagnostic in child operation")
	}
	if !strings.HasSuffix(childEnd.event.Diagnostic.File, "helper_test.go") {
		t.Errorf("expected file helper_test.go, got %q", childEnd.event.Diagnostic.File)
	}
}

func TestDiagHelperRegisteredInChildWorksInChild(t *testing.T) {
	helperFunc := func(ctx Context) {
		ctx.DiagHelper()
		ctx.ControlFail("helper failure")
	}

	w := &recordingWriter{}
	root := New(w)

	var expectedLine int
	root.Run("parent", func(pctx Context) error {
		pctx.Run("child", func(cctx Context) error {
			_, _, expectedLine, _ = runtime.Caller(0); helperFunc(cctx)
			return nil
		})
		return nil
	})

	childEnd := w.ends[0]
	if childEnd.event.Diagnostic == nil {
		t.Fatal("expected diagnostic in child operation")
	}
	if childEnd.event.Diagnostic.Line != expectedLine {
		t.Errorf("expected line %d, got %d", expectedLine, childEnd.event.Diagnostic.Line)
	}
}
