package git

import (
	"testing"
)

func TestParseLog(t *testing.T) {
	input := "abc123def456\x1fJohn Doe\x1fjohn@example.com\x1f2024-01-15T10:30:00-05:00\x1fInitial commit\x1fThis is the body\x1edef789abc123\x1fJane Smith\x1fjane@example.com\x1f2024-01-14T09:00:00-05:00\x1fAdd feature\x1f\x1e"

	entries := ParseLog(input)

	if len(entries) != 2 {
		t.Fatalf("entries count = %d, want 2", len(entries))
	}

	if entries[0].Hash != "abc123def456" {
		t.Errorf("entry 0 hash = %q, want %q", entries[0].Hash, "abc123def456")
	}

	if entries[0].AuthorName != "John Doe" {
		t.Errorf("entry 0 author = %q, want %q", entries[0].AuthorName, "John Doe")
	}

	if entries[0].AuthorEmail != "john@example.com" {
		t.Errorf("entry 0 email = %q, want %q", entries[0].AuthorEmail, "john@example.com")
	}

	if entries[0].AuthorDate != "2024-01-15T10:30:00-05:00" {
		t.Errorf("entry 0 date = %q, want %q", entries[0].AuthorDate, "2024-01-15T10:30:00-05:00")
	}

	if entries[0].Subject != "Initial commit" {
		t.Errorf("entry 0 subject = %q, want %q", entries[0].Subject, "Initial commit")
	}

	if entries[0].Body != "This is the body" {
		t.Errorf("entry 0 body = %q, want %q", entries[0].Body, "This is the body")
	}

	if entries[1].Subject != "Add feature" {
		t.Errorf("entry 1 subject = %q, want %q", entries[1].Subject, "Add feature")
	}

	if entries[1].Body != "" {
		t.Errorf("entry 1 body = %q, want empty", entries[1].Body)
	}
}

func TestParseLogEmpty(t *testing.T) {
	entries := ParseLog("")

	if len(entries) != 0 {
		t.Errorf("entries count = %d, want 0", len(entries))
	}
}

func TestParseShow(t *testing.T) {
	metadata := "abc123\x1fJohn Doe\x1fjohn@example.com\x1f2024-01-15T10:30:00-05:00\x1fFix bug\x1fDetailed fix\x1e"
	numstat := "5\t2\tfile.go\n1\t0\tREADME.md\n"
	patch := "diff --git a/file.go b/file.go\n--- a/file.go\n+++ b/file.go\n"

	result := ParseShow(metadata, numstat, patch)

	if result.Hash != "abc123" {
		t.Errorf("hash = %q, want %q", result.Hash, "abc123")
	}

	if result.Subject != "Fix bug" {
		t.Errorf("subject = %q, want %q", result.Subject, "Fix bug")
	}

	if result.Body != "Detailed fix" {
		t.Errorf("body = %q, want %q", result.Body, "Detailed fix")
	}

	if len(result.Stats) != 2 {
		t.Fatalf("stats count = %d, want 2", len(result.Stats))
	}

	if result.Stats[0].Additions != 5 || result.Stats[0].Deletions != 2 {
		t.Errorf("stat 0 = %+v, want additions=5 deletions=2", result.Stats[0])
	}

	if result.Patch != patch {
		t.Errorf("patch mismatch")
	}
}

func TestParseBlame(t *testing.T) {
	input := "abc123def456 1 1 3\nauthor John Doe\nauthor-mail <john@example.com>\nauthor-time 1705312200\nauthor-tz -0500\ncommitter John Doe\ncommitter-mail <john@example.com>\ncommitter-time 1705312200\ncommitter-tz -0500\nsummary Initial commit\nfilename file.go\n\tpackage main\nabc123def456 2 2\nfilename file.go\n\t\nabc123def456 3 3\nfilename file.go\n\tfunc main() {}\n"

	lines := ParseBlame(input)

	if len(lines) != 3 {
		t.Fatalf("lines count = %d, want 3", len(lines))
	}

	if lines[0].Hash != "abc123def456" {
		t.Errorf("line 0 hash = %q, want %q", lines[0].Hash, "abc123def456")
	}

	if lines[0].OrigLine != 1 {
		t.Errorf("line 0 orig = %d, want 1", lines[0].OrigLine)
	}

	if lines[0].FinalLine != 1 {
		t.Errorf("line 0 final = %d, want 1", lines[0].FinalLine)
	}

	if lines[0].AuthorName != "John Doe" {
		t.Errorf("line 0 author = %q, want %q", lines[0].AuthorName, "John Doe")
	}

	if lines[0].AuthorEmail != "john@example.com" {
		t.Errorf("line 0 email = %q, want %q", lines[0].AuthorEmail, "john@example.com")
	}

	if lines[0].Summary != "Initial commit" {
		t.Errorf("line 0 summary = %q, want %q", lines[0].Summary, "Initial commit")
	}

	if lines[0].Content != "package main" {
		t.Errorf("line 0 content = %q, want %q", lines[0].Content, "package main")
	}

	// Abbreviated entries should inherit author info
	if lines[1].AuthorName != "John Doe" {
		t.Errorf("line 1 author = %q, want %q (inherited)", lines[1].AuthorName, "John Doe")
	}

	if lines[1].Content != "" {
		t.Errorf("line 1 content = %q, want empty", lines[1].Content)
	}

	if lines[2].Content != "func main() {}" {
		t.Errorf("line 2 content = %q, want %q", lines[2].Content, "func main() {}")
	}
}

func TestParseBlameEmpty(t *testing.T) {
	lines := ParseBlame("")

	if len(lines) != 0 {
		t.Errorf("lines count = %d, want 0", len(lines))
	}
}
