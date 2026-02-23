package operation

import (
	"errors"
	"strings"
	"testing"
)

func TestRunSuccess(t *testing.T) {
	w := &recordingWriter{}
	ctx := New(w)
	err := ctx.Run("step", func(ctx Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(w.ends) != 1 {
		t.Fatalf("expected 1 end event, got %d", len(w.ends))
	}
	if w.ends[0].event.Outcome != Success {
		t.Errorf("expected Success, got %d", w.ends[0].event.Outcome)
	}
}

func TestRunPlainError(t *testing.T) {
	w := &recordingWriter{}
	ctx := New(w)
	err := ctx.Run("step", func(ctx Context) error {
		return errors.New("oops")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if w.ends[0].event.Outcome != Failure {
		t.Errorf("expected Failure, got %d", w.ends[0].event.Outcome)
	}
	if w.ends[0].event.Diagnostic == nil {
		t.Fatal("expected diagnostic")
	}
	if w.ends[0].event.Diagnostic.Message != "oops" {
		t.Errorf("expected message 'oops', got %q", w.ends[0].event.Diagnostic.Message)
	}
}

func TestRunControlFail(t *testing.T) {
	w := &recordingWriter{}
	ctx := New(w)
	err := ctx.Run("step", func(ctx Context) error {
		return ctx.ControlFail("bad input")
	})
	if err != nil {
		t.Fatalf("ControlFail should not propagate as error, got %v", err)
	}
	if w.ends[0].event.Outcome != Failure {
		t.Errorf("expected Failure, got %d", w.ends[0].event.Outcome)
	}
	if w.ends[0].event.Diagnostic.File == "" {
		t.Error("expected file:line in diagnostic")
	}
}

func TestRunControlSkip(t *testing.T) {
	w := &recordingWriter{}
	ctx := New(w)
	err := ctx.Run("step", func(ctx Context) error {
		return ctx.ControlSkip("not ready")
	})
	if err != nil {
		t.Fatalf("ControlSkip should not propagate as error, got %v", err)
	}
	if w.ends[0].event.Outcome != Skipped {
		t.Errorf("expected Skipped, got %d", w.ends[0].event.Outcome)
	}
}

func TestRunControlAbort(t *testing.T) {
	w := &recordingWriter{}
	ctx := New(w)
	origErr := errors.New("fatal")
	err := ctx.Run("step", func(ctx Context) error {
		return ctx.ControlAbort(origErr)
	})
	if err != origErr {
		t.Errorf("expected abort error to propagate, got %v", err)
	}
	if w.ends[0].event.Outcome != Aborted {
		t.Errorf("expected Aborted, got %d", w.ends[0].event.Outcome)
	}
}

func TestRunUnknownPanicCaptured(t *testing.T) {
	w := &recordingWriter{}
	ctx := New(w)
	err := ctx.Run("step", func(ctx Context) error {
		panic("nil pointer bug")
	})
	if err != nil {
		t.Fatalf("unknown panics should not propagate as error, got %v", err)
	}
	if w.ends[0].event.Outcome != Failure {
		t.Errorf("expected Failure, got %d", w.ends[0].event.Outcome)
	}
	if w.ends[0].event.Diagnostic.Severity != "panic" {
		t.Errorf("expected severity 'panic', got %q", w.ends[0].event.Diagnostic.Severity)
	}
	if !strings.Contains(w.ends[0].event.Diagnostic.Message, "nil pointer bug") {
		t.Error("expected panic message in diagnostic")
	}
}

func TestRunAnnotationsPassedToEvent(t *testing.T) {
	w := &recordingWriter{}
	ctx := New(w)
	ctx.Run("step", func(ctx Context) error {
		return nil
	}, Destructive, Idempotent)
	if len(w.ends[0].event.Annotations) != 2 {
		t.Errorf("expected 2 annotations, got %d", len(w.ends[0].event.Annotations))
	}
}

func TestRunNestedDepth(t *testing.T) {
	w := &recordingWriter{}
	ctx := New(w)
	ctx.Run("parent", func(ctx Context) error {
		ctx.Run("child", func(ctx Context) error {
			return nil
		})
		return nil
	})
	if w.begins[0].depth != 1 {
		t.Errorf("parent depth: expected 1, got %d", w.begins[0].depth)
	}
	if w.begins[1].depth != 2 {
		t.Errorf("child depth: expected 2, got %d", w.begins[1].depth)
	}
}

func TestRunDiagSetAppearsInEvent(t *testing.T) {
	w := &recordingWriter{}
	ctx := New(w)
	ctx.Run("step", func(ctx Context) error {
		ctx.DiagSet("key", "value")
		return nil
	})
	if w.ends[0].event.Diagnostic == nil {
		t.Fatal("expected diagnostic with extras")
	}
	if w.ends[0].event.Diagnostic.Extras["key"] != "value" {
		t.Error("expected extras[key]=value")
	}
}

func TestRunMustAndAfterExecute(t *testing.T) {
	w := &recordingWriter{}
	ctx := New(w)
	mustRan := false
	afterRan := false
	ctx.Run("step", func(ctx Context) error {
		ctx.Must(func() error { mustRan = true; return nil })
		ctx.After(func() error { afterRan = true; return nil })
		return nil
	})
	if !mustRan {
		t.Error("expected Must to run")
	}
	if !afterRan {
		t.Error("expected After to run")
	}
}

func TestRunMustFailureFlipsEventOutcome(t *testing.T) {
	w := &recordingWriter{}
	ctx := New(w)
	ctx.Run("step", func(ctx Context) error {
		ctx.Must(func() error { return errors.New("flush failed") })
		return nil
	})
	if w.ends[0].event.Outcome != Failure {
		t.Errorf("expected Failure from Must error, got %d", w.ends[0].event.Outcome)
	}
	if len(w.ends[0].event.MustErrors) != 1 {
		t.Fatalf("expected 1 MustError in event, got %d", len(w.ends[0].event.MustErrors))
	}
}

func TestRunMustAndAfterRunEvenOnPanic(t *testing.T) {
	w := &recordingWriter{}
	ctx := New(w)
	mustRan := false
	afterRan := false
	ctx.Run("step", func(ctx Context) error {
		ctx.Must(func() error { mustRan = true; return nil })
		ctx.After(func() error { afterRan = true; return nil })
		panic("boom")
	})
	if !mustRan {
		t.Error("expected Must to run after panic")
	}
	if !afterRan {
		t.Error("expected After to run after panic")
	}
}
