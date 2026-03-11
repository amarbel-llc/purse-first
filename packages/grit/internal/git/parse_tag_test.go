package git

import (
	"fmt"
	"testing"
)

func TestParseTagListAnnotatedAndLightweight(t *testing.T) {
	input := "v1.0.0\x1fabc1234\x1ftag\x1fRelease 1.0\x1f2024-01-15T10:00:00+00:00\x1fJane Doe\x1f<jane@example.com>\x1fdef5678\x1e" +
		"v0.9.0\x1f1234abc\x1fcommit\x1fPre-release\x1f2024-01-10T09:00:00+00:00\x1f\x1f\x1f\x1e"

	tags := ParseTagList(input)

	if len(tags) != 2 {
		t.Fatalf("tags count = %d, want 2", len(tags))
	}

	if tags[0].Name != "v1.0.0" {
		t.Errorf("tag 0 name = %q, want %q", tags[0].Name, "v1.0.0")
	}

	if tags[0].Hash != "abc1234" {
		t.Errorf("tag 0 hash = %q, want %q", tags[0].Hash, "abc1234")
	}

	if tags[0].Type != "annotated" {
		t.Errorf("tag 0 type = %q, want %q", tags[0].Type, "annotated")
	}

	if tags[0].Subject != "Release 1.0" {
		t.Errorf("tag 0 subject = %q, want %q", tags[0].Subject, "Release 1.0")
	}

	if tags[0].TaggerName != "Jane Doe" {
		t.Errorf("tag 0 tagger_name = %q, want %q", tags[0].TaggerName, "Jane Doe")
	}

	if tags[0].TaggerEmail != "jane@example.com" {
		t.Errorf("tag 0 tagger_email = %q, want %q", tags[0].TaggerEmail, "jane@example.com")
	}

	if tags[0].TaggerDate != "2024-01-15T10:00:00+00:00" {
		t.Errorf("tag 0 tagger_date = %q, want %q", tags[0].TaggerDate, "2024-01-15T10:00:00+00:00")
	}

	if tags[0].TargetHash != "def5678" {
		t.Errorf("tag 0 target_hash = %q, want %q", tags[0].TargetHash, "def5678")
	}

	if tags[1].Name != "v0.9.0" {
		t.Errorf("tag 1 name = %q, want %q", tags[1].Name, "v0.9.0")
	}

	if tags[1].Type != "lightweight" {
		t.Errorf("tag 1 type = %q, want %q", tags[1].Type, "lightweight")
	}

	if tags[1].TaggerName != "" {
		t.Errorf("tag 1 tagger_name = %q, want empty", tags[1].TaggerName)
	}

	if tags[1].TargetHash != "" {
		t.Errorf("tag 1 target_hash = %q, want empty", tags[1].TargetHash)
	}
}

func TestParseTagListEmpty(t *testing.T) {
	tags := ParseTagList("")

	if len(tags) != 0 {
		t.Errorf("tags count = %d, want 0", len(tags))
	}
}

func TestParseTagVerifyValid(t *testing.T) {
	stderr := `object abc1234def5678
type commit
tag v1.0.0
tagger Jane Doe <jane@example.com>

Release 1.0
gpg: Signature made Mon Jan 15 10:00:00 2024 UTC
gpg: Good signature from "Jane Doe <jane@example.com>"`

	result := ParseTagVerify("", stderr, nil)

	if !result.Valid {
		t.Error("expected valid = true")
	}

	if result.Signer != "Jane Doe <jane@example.com>" {
		t.Errorf("signer = %q, want %q", result.Signer, "Jane Doe <jane@example.com>")
	}

	if result.Message != "" {
		t.Errorf("message = %q, want empty", result.Message)
	}
}

func TestParseTagVerifyInvalid(t *testing.T) {
	stderr := `gpg: Signature made Mon Jan 15 10:00:00 2024 UTC
gpg: BAD signature from "Jane Doe <jane@example.com>"`

	result := ParseTagVerify("", stderr, fmt.Errorf("exit status 1"))

	if result.Valid {
		t.Error("expected valid = false")
	}

	if result.Message != `gpg: BAD signature from "Jane Doe <jane@example.com>"` {
		t.Errorf("message = %q, want BAD signature line", result.Message)
	}
}

func TestParseTagVerifyUnsigned(t *testing.T) {
	stderr := "error: no signature found"

	result := ParseTagVerify("", stderr, fmt.Errorf("exit status 1"))

	if result.Valid {
		t.Error("expected valid = false")
	}

	if result.Message != "no signature found" {
		t.Errorf("message = %q, want %q", result.Message, "no signature found")
	}
}
