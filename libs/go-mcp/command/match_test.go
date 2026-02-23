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
