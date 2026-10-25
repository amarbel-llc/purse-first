package output

import "testing"

func TestStandardDefaults(t *testing.T) {
	d := StandardDefaults()

	if d.MaxBytes != 100_000 {
		t.Fatalf("expected MaxBytes=100000, got %d", d.MaxBytes)
	}

	if d.MaxLines != 2000 {
		t.Fatalf("expected MaxLines=2000, got %d", d.MaxLines)
	}

	if d.MaxItems != 100 {
		t.Fatalf("expected MaxItems=100, got %d", d.MaxItems)
	}
}

func TestMergeTextLimitsAllZero(t *testing.T) {
	d := StandardDefaults()
	merged := d.MergeTextLimits(TextLimits{})

	if merged.MaxBytes != 100_000 {
		t.Fatalf("expected MaxBytes filled from default, got %d", merged.MaxBytes)
	}

	if merged.MaxLines != 2000 {
		t.Fatalf("expected MaxLines filled from default, got %d", merged.MaxLines)
	}

	if merged.Head != 0 {
		t.Fatalf("expected Head=0 (never defaulted), got %d", merged.Head)
	}

	if merged.Tail != 0 {
		t.Fatalf("expected Tail=0 (never defaulted), got %d", merged.Tail)
	}
}

func TestMergeTextLimitsUserOverrides(t *testing.T) {
	d := StandardDefaults()
	merged := d.MergeTextLimits(TextLimits{MaxBytes: 500, MaxLines: 10})

	if merged.MaxBytes != 500 {
		t.Fatalf("expected user MaxBytes=500 preserved, got %d", merged.MaxBytes)
	}

	if merged.MaxLines != 10 {
		t.Fatalf("expected user MaxLines=10 preserved, got %d", merged.MaxLines)
	}
}

func TestMergeTextLimitsHeadTailPreserved(t *testing.T) {
	d := StandardDefaults()
	merged := d.MergeTextLimits(TextLimits{Head: 50, Tail: 25})

	if merged.Head != 50 {
		t.Fatalf("expected Head=50 preserved, got %d", merged.Head)
	}

	if merged.Tail != 25 {
		t.Fatalf("expected Tail=25 preserved, got %d", merged.Tail)
	}
}

func TestMergeArrayLimitsAllZero(t *testing.T) {
	d := StandardDefaults()
	merged := d.MergeArrayLimits(ArrayLimits{})

	if merged.Limit != 100 {
		t.Fatalf("expected Limit filled from default, got %d", merged.Limit)
	}

	if merged.Offset != 0 {
		t.Fatalf("expected Offset=0 (never defaulted), got %d", merged.Offset)
	}
}

func TestMergeArrayLimitsUserOverrides(t *testing.T) {
	d := StandardDefaults()
	merged := d.MergeArrayLimits(ArrayLimits{Limit: 50})

	if merged.Limit != 50 {
		t.Fatalf("expected user Limit=50 preserved, got %d", merged.Limit)
	}
}

func TestMergeArrayLimitsOffsetPreserved(t *testing.T) {
	d := StandardDefaults()
	merged := d.MergeArrayLimits(ArrayLimits{Offset: 10})

	if merged.Offset != 10 {
		t.Fatalf("expected Offset=10 preserved, got %d", merged.Offset)
	}

	if merged.Limit != 100 {
		t.Fatalf("expected Limit filled from default, got %d", merged.Limit)
	}
}
