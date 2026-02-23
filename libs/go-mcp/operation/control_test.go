package operation

import (
	"errors"
	"testing"
)

func TestControlFailPanicsWithFailSentinel(t *testing.T) {
	c := &ctx{}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		s, ok := r.(failSentinel)
		if !ok {
			t.Fatalf("expected failSentinel, got %T", r)
		}
		if s.diag.Message != "bad input" {
			t.Errorf("expected message 'bad input', got %q", s.diag.Message)
		}
		if s.diag.Severity != "error" {
			t.Errorf("expected severity 'error', got %q", s.diag.Severity)
		}
		if s.diag.File == "" {
			t.Error("expected file to be captured")
		}
		if s.diag.Line == 0 {
			t.Error("expected line to be captured")
		}
	}()
	c.ControlFail("bad input")
}

func TestControlFailfFormatsMessage(t *testing.T) {
	c := &ctx{}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		s := r.(failSentinel)
		if s.diag.Message != "bad input: 42" {
			t.Errorf("expected formatted message, got %q", s.diag.Message)
		}
	}()
	c.ControlFailf("bad input: %d", 42)
}

func TestControlWrapPanicsWithFailSentinel(t *testing.T) {
	c := &ctx{}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		s, ok := r.(failSentinel)
		if !ok {
			t.Fatalf("expected failSentinel, got %T", r)
		}
		if s.diag.Source != "external" {
			t.Errorf("expected source 'external', got %q", s.diag.Source)
		}
		if s.diag.Message != "db error" {
			t.Errorf("expected message 'db error', got %q", s.diag.Message)
		}
	}()
	c.ControlWrap(errors.New("db error"))
}

func TestControlWrapfFormatsMessage(t *testing.T) {
	c := &ctx{}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		s := r.(failSentinel)
		expected := "migration 002: db error"
		if s.diag.Message != expected {
			t.Errorf("expected %q, got %q", expected, s.diag.Message)
		}
		if s.diag.Source != "external" {
			t.Errorf("expected source 'external', got %q", s.diag.Source)
		}
	}()
	c.ControlWrapf(errors.New("db error"), "migration %s", "002")
}

func TestControlSkipPanicsWithSkipSentinel(t *testing.T) {
	c := &ctx{}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		s, ok := r.(skipSentinel)
		if !ok {
			t.Fatalf("expected skipSentinel, got %T", r)
		}
		if s.diag.Message != "not ready" {
			t.Errorf("expected message 'not ready', got %q", s.diag.Message)
		}
		if s.diag.Severity != "skip" {
			t.Errorf("expected severity 'skip', got %q", s.diag.Severity)
		}
	}()
	c.ControlSkip("not ready")
}

func TestControlAbortPanicsWithAbortSentinel(t *testing.T) {
	c := &ctx{}
	origErr := errors.New("fatal")
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		s, ok := r.(abortSentinel)
		if !ok {
			t.Fatalf("expected abortSentinel, got %T", r)
		}
		if s.err != origErr {
			t.Errorf("expected original error")
		}
	}()
	c.ControlAbort(origErr)
}
