package operation

import (
	"errors"
	"testing"
)

func TestAfterRegistersCallback(t *testing.T) {
	c := &ctx{}
	called := false
	c.After(func() error { called = true; return nil })
	if len(c.afters) != 1 {
		t.Fatalf("expected 1 after, got %d", len(c.afters))
	}
	c.runAfter()
	if !called {
		t.Error("expected after to be called")
	}
}

func TestAfterRunsLIFO(t *testing.T) {
	c := &ctx{}
	var order []int
	c.After(func() error { order = append(order, 1); return nil })
	c.After(func() error { order = append(order, 2); return nil })
	c.runAfter()
	if len(order) != 2 || order[0] != 2 || order[1] != 1 {
		t.Errorf("expected LIFO [2,1], got %v", order)
	}
}

func TestAfterSwallowsErrors(t *testing.T) {
	c := &ctx{}
	c.After(func() error { return errors.New("ignored") })
	c.runAfter()
}

func TestAfterRecoversPanics(t *testing.T) {
	c := &ctx{}
	c.After(func() error { panic("boom") })
	c.runAfter()
}

func TestMustRegistersCallback(t *testing.T) {
	c := &ctx{outcome: Success}
	c.Must(func() error { return nil })
	if len(c.musts) != 1 {
		t.Fatalf("expected 1 must, got %d", len(c.musts))
	}
	c.runMust()
	if c.outcome != Success {
		t.Error("expected outcome to remain Success")
	}
}

func TestMustErrorFlipsSuccessToFailure(t *testing.T) {
	c := &ctx{outcome: Success}
	c.Must(func() error { return errors.New("flush failed") })
	c.runMust()
	if c.outcome != Failure {
		t.Errorf("expected Failure, got %d", c.outcome)
	}
	if len(c.event.MustErrors) != 1 {
		t.Fatalf("expected 1 must error, got %d", len(c.event.MustErrors))
	}
}

func TestMustMultipleErrors(t *testing.T) {
	c := &ctx{outcome: Success}
	c.Must(func() error { return errors.New("err1") })
	c.Must(func() error { return errors.New("err2") })
	c.runMust()
	if len(c.event.MustErrors) != 2 {
		t.Fatalf("expected 2 must errors, got %d", len(c.event.MustErrors))
	}
}

func TestMustRecoversPanics(t *testing.T) {
	c := &ctx{outcome: Success}
	c.Must(func() error { panic("kaboom") })
	c.runMust()
	if c.outcome != Failure {
		t.Error("expected Failure from panicking Must")
	}
	if len(c.event.MustErrors) != 1 {
		t.Fatalf("expected 1 must error, got %d", len(c.event.MustErrors))
	}
}

func TestMustDoesNotFlipFailure(t *testing.T) {
	c := &ctx{outcome: Failure}
	c.Must(func() error { return errors.New("also failed") })
	c.runMust()
	if c.outcome != Failure {
		t.Error("expected outcome to remain Failure")
	}
}
