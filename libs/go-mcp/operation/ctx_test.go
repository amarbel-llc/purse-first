package operation

import "testing"

func TestNewReturnsContext(t *testing.T) {
	var w nullWriter
	ctx := New(&w)
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
}
