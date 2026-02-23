package operation

import "testing"

func TestOutcomeValues(t *testing.T) {
	if Success != 0 {
		t.Errorf("Success should be 0, got %d", Success)
	}
	if Failure != 1 {
		t.Errorf("Failure should be 1, got %d", Failure)
	}
	if Skipped != 2 {
		t.Errorf("Skipped should be 2, got %d", Skipped)
	}
	if Aborted != 3 {
		t.Errorf("Aborted should be 3, got %d", Aborted)
	}
}

func TestAnnotationBitfield(t *testing.T) {
	combined := Idempotent | Destructive
	if combined&Idempotent == 0 {
		t.Error("expected Idempotent set")
	}
	if combined&Destructive == 0 {
		t.Error("expected Destructive set")
	}
	if combined&ReadOnly != 0 {
		t.Error("expected ReadOnly not set")
	}
}
