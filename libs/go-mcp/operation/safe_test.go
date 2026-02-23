package operation

import (
	"errors"
	"strings"
	"testing"
)

func TestCallSafeReturnsNilOnSuccess(t *testing.T) {
	err := callSafe(func() error { return nil })
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestCallSafeReturnsError(t *testing.T) {
	expected := errors.New("oops")
	err := callSafe(func() error { return expected })
	if err != expected {
		t.Errorf("expected %v, got %v", expected, err)
	}
}

func TestCallSafeRecoversPanic(t *testing.T) {
	err := callSafe(func() error { panic("boom") })
	if err == nil {
		t.Fatal("expected error from panic")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("expected panic message in error, got: %v", err)
	}
}

func TestCallSafeRecoversPanicWithError(t *testing.T) {
	err := callSafe(func() error { panic(errors.New("kaboom")) })
	if err == nil {
		t.Fatal("expected error from panic")
	}
	if !strings.Contains(err.Error(), "kaboom") {
		t.Errorf("expected panic message in error, got: %v", err)
	}
}
