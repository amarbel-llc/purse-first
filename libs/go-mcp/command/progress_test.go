package command

import (
	"context"
	"testing"
)

// TestProgressFromContextAbsent verifies the command-level re-export
// delegates to the server accessor: a bare context yields ok = false and
// a no-op emitter that is safe to call.
func TestProgressFromContextAbsent(t *testing.T) {
	emit, ok := ProgressFromContext(context.Background())
	if ok {
		t.Fatal("ProgressFromContext on a bare context: ok = true, want false")
	}
	if err := emit(0.5, nil, "noop"); err != nil {
		t.Fatalf("no-op emit returned error: %v", err)
	}
}
