package buildinfo

import (
	"bytes"
	"strings"
	"testing"
)

// TestPrintEmitsOnlySelfLine asserts the eng-versioning(7) "no pinned
// components" shape: a single self-identification line, no blank line,
// and no component table header (purse-first#118).
func TestPrintEmitsOnlySelfLine(t *testing.T) {
	var buf bytes.Buffer
	Print(&buf, "/path/to/dagnabit")

	got := buf.String()

	want := "dagnabit " + Version + "+" + Commit + "\n"
	if got != want {
		t.Errorf("Print output mismatch:\ngot:  %q\nwant: %q", got, want)
	}

	if strings.Contains(got, "COMPONENT") {
		t.Errorf("Print emitted a component table header; binaries pinning nothing must omit it: %q", got)
	}

	if lines := strings.Count(got, "\n"); lines != 1 {
		t.Errorf("Print emitted %d lines, want exactly 1 (no blank line, no table): %q", lines, got)
	}
}
