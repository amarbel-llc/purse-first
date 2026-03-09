package executor

import "testing"

func TestParseSessionsMultipleLines(t *testing.T) {
	input := "  session_name=finicky/rich-magnolia\tstatus=Unexpected\t(cleaning up)\n" +
		"→ session_name=purse-first/fresh-oak\tstatus=Unexpected\t(cleaning up)\n" +
		"  session_name=purse-first/ready-sycamore\tstatus=Unexpected\t(cleaning up)\n"
	sessions := parseSessions(input)
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}
	for _, key := range []string{"finicky/rich-magnolia", "purse-first/fresh-oak", "purse-first/ready-sycamore"} {
		if !sessions[key] {
			t.Errorf("expected session %q to be present", key)
		}
	}
}

func TestParseSessionsEmpty(t *testing.T) {
	sessions := parseSessions("")
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestParseSessionsNoMatch(t *testing.T) {
	sessions := parseSessions("some garbage output\nanother line\n")
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}
