package git

import (
	"testing"
)

func TestParseCommit(t *testing.T) {
	input := "[main abc1234] Add new feature\n 2 files changed, 10 insertions(+), 3 deletions(-)\n"

	result := ParseCommit(input)

	if result.Status != "committed" {
		t.Errorf("status = %q, want %q", result.Status, "committed")
	}

	if result.Branch != "main" {
		t.Errorf("branch = %q, want %q", result.Branch, "main")
	}

	if result.Hash != "abc1234" {
		t.Errorf("hash = %q, want %q", result.Hash, "abc1234")
	}

	if result.Subject != "Add new feature" {
		t.Errorf("subject = %q, want %q", result.Subject, "Add new feature")
	}
}

func TestParseCommitDetachedHead(t *testing.T) {
	input := "[detached HEAD abc1234] Fix bug\n"

	result := ParseCommit(input)

	if result.Branch != "detached HEAD" {
		t.Errorf("branch = %q, want %q", result.Branch, "detached HEAD")
	}
}

func TestParseCommitUnexpectedFormat(t *testing.T) {
	input := "something unexpected"

	result := ParseCommit(input)

	if result.Status != "committed" {
		t.Errorf("status = %q, want %q", result.Status, "committed")
	}

	if result.Subject != "something unexpected" {
		t.Errorf("subject = %q, want %q", result.Subject, "something unexpected")
	}
}
