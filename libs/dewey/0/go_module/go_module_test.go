package go_module

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveModulePathExplicitWins(t *testing.T) {
	got, err := ResolveModulePath("/nowhere", "example.com/explicit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != "example.com/explicit" {
		t.Errorf("got %q, want %q", got, "example.com/explicit")
	}
}

func TestResolveModulePathReadsFromGoMod(t *testing.T) {
	dir := t.TempDir()

	writeGoMod(t, dir, "module example.com/from-mod\n\ngo 1.21\n")

	got, err := ResolveModulePath(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != "example.com/from-mod" {
		t.Errorf("got %q, want %q", got, "example.com/from-mod")
	}
}

func TestResolveModulePathMissingGoMod(t *testing.T) {
	dir := t.TempDir()

	_, err := ResolveModulePath(dir, "")
	if err == nil {
		t.Fatal("expected error when go.mod missing, got nil")
	}

	if !strings.Contains(err.Error(), "go.mod") {
		t.Errorf("expected error to mention go.mod, got %q", err.Error())
	}
}

func TestReadModulePathHandlesCommentsAndBlankLines(t *testing.T) {
	dir := t.TempDir()

	writeGoMod(
		t,
		dir,
		`// a comment

module example.com/leading-whitespace

go 1.21
`,
	)

	got, err := ReadModulePath(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != "example.com/leading-whitespace" {
		t.Errorf("got %q, want %q", got, "example.com/leading-whitespace")
	}
}

func TestReadModulePathMissingDirective(t *testing.T) {
	dir := t.TempDir()

	writeGoMod(t, dir, "go 1.21\n")

	_, err := ReadModulePath(filepath.Join(dir, "go.mod"))
	if err == nil {
		t.Fatal("expected error for missing module directive, got nil")
	}

	if !strings.Contains(err.Error(), "no module directive") {
		t.Errorf("expected error to mention missing directive, got %q", err.Error())
	}
}

func writeGoMod(t *testing.T, dir, content string) {
	t.Helper()

	p := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
