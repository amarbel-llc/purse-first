package dagnabit

import (
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJenTypeBasic(t *testing.T) {
	code := jenType(types.Typ[types.Int])
	if code == nil {
		t.Fatal("jenType returned nil for int")
	}
}

func TestJenTypePointer(t *testing.T) {
	code := jenType(types.NewPointer(types.Typ[types.String]))
	if code == nil {
		t.Fatal("jenType returned nil for *string")
	}
}

func TestJenTypeSlice(t *testing.T) {
	code := jenType(types.NewSlice(types.Typ[types.Byte]))
	if code == nil {
		t.Fatal("jenType returned nil for []byte")
	}
}

func TestJenTypeMap(t *testing.T) {
	code := jenType(types.NewMap(types.Typ[types.String], types.Typ[types.Int]))
	if code == nil {
		t.Fatal("jenType returned nil for map[string]int")
	}
}

func TestScanForExportDirectives(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a package with the directive
	pkgDir := filepath.Join(tmpDir, "internal", "alfa", "has_directive")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(pkgDir, "main.go"),
		[]byte("package has_directive\n\n//go:generate dagnabit export\n\ntype Foo struct{}\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	// Create a package without the directive
	noDirPkg := filepath.Join(tmpDir, "internal", "bravo", "no_directive")
	if err := os.MkdirAll(noDirPkg, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(noDirPkg, "main.go"),
		[]byte("package no_directive\n\ntype Bar struct{}\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	dirs, err := scanForExportDirectives(filepath.Join(tmpDir, "internal"))
	if err != nil {
		t.Fatal(err)
	}

	if len(dirs) != 1 {
		t.Fatalf("expected 1 directory, got %d: %v", len(dirs), dirs)
	}

	if dirs[0] != pkgDir {
		t.Errorf("expected %s, got %s", pkgDir, dirs[0])
	}
}

func TestFileContainsDirective(t *testing.T) {
	tmpDir := t.TempDir()

	withDirective := filepath.Join(tmpDir, "with.go")
	if err := os.WriteFile(withDirective, []byte("package foo\n\n//go:generate dagnabit export\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	withoutDirective := filepath.Join(tmpDir, "without.go")
	if err := os.WriteFile(withoutDirective, []byte("package foo\n\n//go:generate stringer -type=Foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	has, err := fileContainsDirective(withDirective)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Error("expected directive to be found")
	}

	has, err = fileContainsDirective(withoutDirective)
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("expected directive not to be found")
	}
}

func TestScanForExportDirectivesEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(tmpDir, "internal"), 0o755); err != nil {
		t.Fatal(err)
	}

	dirs, err := scanForExportDirectives(filepath.Join(tmpDir, "internal"))
	if err != nil {
		t.Fatal(err)
	}

	if len(dirs) != 0 {
		t.Fatalf("expected 0 directories, got %d: %v", len(dirs), dirs)
	}
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("output does not contain %q\n\ngot:\n%s", want, got)
	}
}

func assertNotContains(t *testing.T, got, unwanted string) {
	t.Helper()
	if strings.Contains(got, unwanted) {
		t.Errorf("output should not contain %q\n\ngot:\n%s", unwanted, got)
	}
}
