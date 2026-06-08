package dagnabit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/amarbel-llc/purse-first/libs/dewey/internal/alfa/test_ui"
)

// TestExportPropagatesDocComments verifies that doc comments attached to
// exported declarations in an internal package are carried over to the
// generated facade.
func TestExportPropagatesDocComments(t *testing.T) {
	tt := test_ui.T{T: t}
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"),
		[]byte("module example.com/mod\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	pkgDir := filepath.Join(tmpDir, "internal", "alfa", "widget")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "main.go"), []byte(`package widget

// Widget is an example exported type.
type Widget struct{}

// New constructs a [Widget].
func New() Widget { return Widget{} }

// MaxSize is the upper bound on a widget's dimensions.
const MaxSize = 42

// Deprecated: use New instead.
var Make = New

// Singleton is the canonical widget instance.
var Singleton = Widget{}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	exporter := &Exporter{
		ModulePath:          "example.com/mod",
		Dir:                 tmpDir,
		OutputDir:           "pkgs",
		SkipConsumerRewrite: true,
		Env:                 append(os.Environ(), "GOWORK=off"),
	}

	if err := exporter.ExportPackage("./internal/alfa/widget"); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "pkgs", "widget", "main.go"))
	if err != nil {
		t.Fatalf("pkgs/widget/main.go not generated: %v", err)
	}
	got := string(content)

	// Every doc comment line should appear in the facade.
	wants := []string{
		"// Widget is an example exported type.",
		"// New constructs a [Widget].",
		"// MaxSize is the upper bound on a widget's dimensions.",
		"// Deprecated: use New instead.",
		"// Singleton is the canonical widget instance.",
	}
	for _, want := range wants {
		assertContains(tt, got, want)
	}
}

// TestExportSkipsGoDirectives verifies that //go:* compiler directives
// in the source doc-block are NOT propagated to the facade. The facade
// is a re-export; compiler directives apply to the original symbol's
// implementation, not to its alias.
func TestExportSkipsGoDirectives(t *testing.T) {
	tt := test_ui.T{T: t}
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"),
		[]byte("module example.com/mod\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	pkgDir := filepath.Join(tmpDir, "internal", "alfa", "widget")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "main.go"), []byte(`package widget

// HotPath is a frequently-called helper.
//
//go:noinline
func HotPath() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	exporter := &Exporter{
		ModulePath:          "example.com/mod",
		Dir:                 tmpDir,
		OutputDir:           "pkgs",
		SkipConsumerRewrite: true,
		Env:                 append(os.Environ(), "GOWORK=off"),
	}

	if err := exporter.ExportPackage("./internal/alfa/widget"); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "pkgs", "widget", "main.go"))
	if err != nil {
		t.Fatalf("pkgs/widget/main.go not generated: %v", err)
	}
	got := string(content)

	assertContains(tt, got, "// HotPath is a frequently-called helper.")
	assertNotContains(tt, got, "//go:noinline")
}
