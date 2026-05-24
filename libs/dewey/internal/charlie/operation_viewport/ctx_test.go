package operation_viewport

import (
	stderrors "errors"
	"os"
	"testing"

	"github.com/amarbel-llc/purse-first/libs/dewey/internal/bravo/errors"
)

// TestPropagateCancel_InterruptedReturnsSignal verifies the Ctrl-C
// branch: when the Model carries Interrupted=true after program.Run
// exits, the inner errors.Context is cancelled with errors.Signal and
// the enclosing ctx.Run returns that as the cause.
func TestPropagateCancel_InterruptedReturnsSignal(t *testing.T) {
	parent := errors.MakeContextDefault()
	ctx := errors.MakeContext(parent)

	err := ctx.Run(func(c errors.Context) {
		m := Model{interrupted: true}
		propagateCancel(c, m, nil, nil)
	})

	if err == nil {
		t.Fatal("expected non-nil error, got nil")
	}
	var sig errors.Signal
	if !stderrors.As(err, &sig) {
		t.Errorf("expected errors.Signal, got %T %v", err, err)
		return
	}
	if sig.Signal != os.Interrupt {
		t.Errorf("Signal.Signal: got %v, want %v", sig.Signal, os.Interrupt)
	}
}

// TestPropagateCancel_BatchErrWins verifies that a non-nil run-loop
// error is propagated when Interrupted is false.
func TestPropagateCancel_BatchErrWins(t *testing.T) {
	sentinel := stderrors.New("sentinel batch failure")
	parent := errors.MakeContextDefault()
	ctx := errors.MakeContext(parent)

	err := ctx.Run(func(c errors.Context) {
		propagateCancel(c, Model{}, sentinel, nil)
	})

	if !stderrors.Is(err, sentinel) {
		t.Errorf("expected sentinel, got %v", err)
	}
}

// TestPropagateCancel_SuccessReturnsNil verifies that funcRun returns
// normally when all error inputs are nil and Interrupted is false.
func TestPropagateCancel_SuccessReturnsNil(t *testing.T) {
	parent := errors.MakeContextDefault()
	ctx := errors.MakeContext(parent)

	err := ctx.Run(func(c errors.Context) {
		propagateCancel(c, Model{}, nil, nil)
	})

	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// TestPropagateCancel_InterruptedWinsOverBatchErr verifies the priority
// when both Ctrl-C and a work-loop error are present.
func TestPropagateCancel_InterruptedWinsOverBatchErr(t *testing.T) {
	sentinel := stderrors.New("sentinel batch failure")
	parent := errors.MakeContextDefault()
	ctx := errors.MakeContext(parent)

	err := ctx.Run(func(c errors.Context) {
		m := Model{interrupted: true}
		propagateCancel(c, m, sentinel, nil)
	})

	var sig errors.Signal
	if !stderrors.As(err, &sig) {
		t.Errorf("expected Signal to win over batch err, got %T %v", err, err)
	}
}
