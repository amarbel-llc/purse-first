package git

import (
	"testing"
)

func TestParseStatus(t *testing.T) {
	input := `# branch.oid abc123def456
# branch.head main
# branch.upstream origin/main
# branch.ab +2 -1
1 .M N... 100644 100644 100644 abc123 def456 file.go
1 M. N... 100644 100644 100644 abc123 def456 staged.go
? untracked.txt
`

	result := ParseStatus(input)

	if result.Branch.OID != "abc123def456" {
		t.Errorf("branch OID = %q, want %q", result.Branch.OID, "abc123def456")
	}

	if result.Branch.Head != "main" {
		t.Errorf("branch head = %q, want %q", result.Branch.Head, "main")
	}

	if result.Branch.Upstream != "origin/main" {
		t.Errorf("branch upstream = %q, want %q", result.Branch.Upstream, "origin/main")
	}

	if result.Branch.Ahead != 2 {
		t.Errorf("branch ahead = %d, want %d", result.Branch.Ahead, 2)
	}

	if result.Branch.Behind != 1 {
		t.Errorf("branch behind = %d, want %d", result.Branch.Behind, 1)
	}

	if len(result.Entries) != 3 {
		t.Fatalf("entries count = %d, want %d", len(result.Entries), 3)
	}

	if result.Entries[0].State != ".M" {
		t.Errorf("entry 0 state = %q, want %q", result.Entries[0].State, ".M")
	}

	if result.Entries[0].Path != "file.go" {
		t.Errorf("entry 0 path = %q, want %q", result.Entries[0].Path, "file.go")
	}

	if result.Entries[1].State != "M." {
		t.Errorf("entry 1 state = %q, want %q", result.Entries[1].State, "M.")
	}

	if result.Entries[2].State != "?" {
		t.Errorf("entry 2 state = %q, want %q", result.Entries[2].State, "?")
	}

	if result.Entries[2].Path != "untracked.txt" {
		t.Errorf("entry 2 path = %q, want %q", result.Entries[2].Path, "untracked.txt")
	}
}

func TestParseStatusEmpty(t *testing.T) {
	input := `# branch.oid abc123
# branch.head main
`

	result := ParseStatus(input)

	if len(result.Entries) != 0 {
		t.Errorf("entries count = %d, want 0", len(result.Entries))
	}
}

func TestParseStatusRename(t *testing.T) {
	input := `# branch.oid abc123
# branch.head main
2 R. N... 100644 100644 100644 abc123 def456 R100 new.go	old.go
`

	result := ParseStatus(input)

	if len(result.Entries) != 1 {
		t.Fatalf("entries count = %d, want 1", len(result.Entries))
	}

	if result.Entries[0].State != "R." {
		t.Errorf("entry state = %q, want %q", result.Entries[0].State, "R.")
	}

	if result.Entries[0].Path != "new.go" {
		t.Errorf("entry path = %q, want %q", result.Entries[0].Path, "new.go")
	}

	if result.Entries[0].OrigPath != "old.go" {
		t.Errorf("entry orig_path = %q, want %q", result.Entries[0].OrigPath, "old.go")
	}
}

func TestParseDiffNumstat(t *testing.T) {
	input := `10	5	file.go
0	3	deleted.go
-	-	binary.png
`

	stats := ParseDiffNumstat(input)

	if len(stats) != 3 {
		t.Fatalf("stats count = %d, want 3", len(stats))
	}

	if stats[0].Additions != 10 || stats[0].Deletions != 5 || stats[0].Path != "file.go" {
		t.Errorf("stat 0 = %+v, want additions=10 deletions=5 path=file.go", stats[0])
	}

	if stats[1].Additions != 0 || stats[1].Deletions != 3 {
		t.Errorf("stat 1 = %+v, want additions=0 deletions=3", stats[1])
	}

	if !stats[2].Binary || stats[2].Path != "binary.png" {
		t.Errorf("stat 2 = %+v, want binary=true path=binary.png", stats[2])
	}
}

func TestParseDiffNumstatEmpty(t *testing.T) {
	stats := ParseDiffNumstat("")

	if len(stats) != 0 {
		t.Errorf("stats count = %d, want 0", len(stats))
	}
}
