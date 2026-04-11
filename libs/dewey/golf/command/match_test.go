package command

import (
	"testing"
)

func TestMatchByCommandPrefix(t *testing.T) {
	mappings := []ToolMapping{
		{
			Replaces:        "Bash",
			CommandPrefixes: []string{"git status"},
			UseWhen:         "checking repository status",
		},
	}

	got := FindToolMatch(mappings, "Bash", "", "git status --short")
	if got == nil {
		t.Fatal("expected match, got nil")
	}
	if got.Replaces != "Bash" {
		t.Errorf("Replaces = %q, want %q", got.Replaces, "Bash")
	}
}

func TestMatchByExtension(t *testing.T) {
	mappings := []ToolMapping{
		{
			Replaces:   "Read",
			Extensions: []string{".go", ".py"},
			UseWhen:    "getting type info",
		},
	}

	got := FindToolMatch(mappings, "Read", "/foo/bar.go", "")
	if got == nil {
		t.Fatal("expected match, got nil")
	}
	if got.Replaces != "Read" {
		t.Errorf("Replaces = %q, want %q", got.Replaces, "Read")
	}
}

func TestNoMatchWrongTool(t *testing.T) {
	mappings := []ToolMapping{
		{
			Replaces:        "Bash",
			CommandPrefixes: []string{"git status"},
			UseWhen:         "checking repository status",
		},
	}

	got := FindToolMatch(mappings, "Read", "", "git status")
	if got != nil {
		t.Errorf("expected nil for wrong tool name, got %+v", got)
	}
}

func TestNoMatchWrongPrefix(t *testing.T) {
	mappings := []ToolMapping{
		{
			Replaces:        "Bash",
			CommandPrefixes: []string{"git status"},
			UseWhen:         "checking repository status",
		},
	}

	got := FindToolMatch(mappings, "Bash", "", "docker ps")
	if got != nil {
		t.Errorf("expected nil for non-matching prefix, got %+v", got)
	}
}

func TestNoMatchWrongExtension(t *testing.T) {
	mappings := []ToolMapping{
		{
			Replaces:   "Read",
			Extensions: []string{".go"},
			UseWhen:    "reading Go source files",
		},
	}

	got := FindToolMatch(mappings, "Read", "/foo/bar.txt", "")
	if got != nil {
		t.Errorf("expected nil for non-matching extension, got %+v", got)
	}
}

func TestMatchExtensionOrPrefix(t *testing.T) {
	mappings := []ToolMapping{
		{
			Replaces:        "Bash",
			Extensions:      []string{".sh"},
			CommandPrefixes: []string{"git "},
			UseWhen:         "running shell scripts or git commands",
		},
	}

	// Matches by prefix alone (no matching extension).
	got := FindToolMatch(mappings, "Bash", "/foo/bar.py", "git commit -m msg")
	if got == nil {
		t.Fatal("expected match by prefix, got nil")
	}

	// Matches by extension alone (no matching prefix).
	got = FindToolMatch(mappings, "Bash", "/foo/bar.sh", "echo hello")
	if got == nil {
		t.Fatal("expected match by extension, got nil")
	}

	// Neither matches → nil.
	got = FindToolMatch(mappings, "Bash", "/foo/bar.py", "docker ps")
	if got != nil {
		t.Errorf("expected nil when neither extension nor prefix matches, got %+v", got)
	}
}

func TestMatchCaseInsensitiveExtension(t *testing.T) {
	mappings := []ToolMapping{
		{
			Replaces:   "Read",
			Extensions: []string{".Go"},
			UseWhen:    "reading Go source files",
		},
	}

	got := FindToolMatch(mappings, "Read", "/foo/bar.GO", "")
	if got == nil {
		t.Fatal("expected case-insensitive extension match, got nil")
	}
	if got.Replaces != "Read" {
		t.Errorf("Replaces = %q, want %q", got.Replaces, "Read")
	}
}

func TestMatchCatchAll(t *testing.T) {
	mappings := []ToolMapping{
		{
			Replaces: "Bash",
			UseWhen:  "always use this tool",
		},
	}

	got := FindToolMatch(mappings, "Bash", "", "anything at all")
	if got == nil {
		t.Fatal("expected catch-all match, got nil")
	}
	if got.Replaces != "Bash" {
		t.Errorf("Replaces = %q, want %q", got.Replaces, "Bash")
	}
}
