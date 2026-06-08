package dagnabit

import (
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/amarbel-llc/purse-first/libs/dewey/internal/alfa/test_ui"
)

func identity(s string) string { return s }

func TestJenTypeBasic(t *testing.T) {
	code := jenType(types.Typ[types.Int], identity)
	if code == nil {
		t.Fatal("jenType returned nil for int")
	}
}

func TestJenTypePointer(t *testing.T) {
	code := jenType(types.NewPointer(types.Typ[types.String]), identity)
	if code == nil {
		t.Fatal("jenType returned nil for *string")
	}
}

func TestJenTypeSlice(t *testing.T) {
	code := jenType(types.NewSlice(types.Typ[types.Byte]), identity)
	if code == nil {
		t.Fatal("jenType returned nil for []byte")
	}
}

func TestJenTypeMap(t *testing.T) {
	code := jenType(types.NewMap(types.Typ[types.String], types.Typ[types.Int]), identity)
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

func TestExportAllRejectsExportDirectives(t *testing.T) {
	tmpDir := t.TempDir()

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

	exporter := &Exporter{
		ModulePath: "example.com/mod",
		Dir:        tmpDir,
		OutputDir:  "pkgs",
	}

	err := exporter.ExportAll()
	if err == nil {
		t.Fatal("expected error when export directives are present, got nil")
	}

	if !strings.Contains(err.Error(), "--library mode") {
		t.Errorf("expected error to mention --library mode, got: %v", err)
	}
}

func TestCollectBuildTags(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"),
		[]byte("package foo\n\ntype Foo struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "test.go"),
		[]byte("//go:build test\n\npackage foo\n\ntype Bar struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "test2.go"),
		[]byte("//go:build test\n\npackage foo\n\ntype Baz struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tags, err := collectBuildTags(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(tags) != 1 {
		t.Fatalf("expected 1 unique tag, got %d: %v", len(tags), tags)
	}
	if tags[0] != "test" {
		t.Errorf("expected tag %q, got %q", "test", tags[0])
	}
}

func TestExportPackageWithBuildTags(t *testing.T) {
	tt := test_ui.T{T: t}
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"),
		[]byte("module example.com/mod\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	pkgDir := filepath.Join(tmpDir, "internal", "alfa", "widget")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "main.go"),
		[]byte("package widget\n\ntype Widget struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "test.go"),
		[]byte("//go:build test\n\npackage widget\n\ntype TestWidget struct{}\n"), 0o644); err != nil {
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

	mainContent, err := os.ReadFile(filepath.Join(tmpDir, "pkgs", "widget", "main.go"))
	if err != nil {
		t.Fatalf("main.go not generated: %v", err)
	}
	testContent, err := os.ReadFile(filepath.Join(tmpDir, "pkgs", "widget", "test.go"))
	if err != nil {
		t.Fatalf("test.go not generated: %v", err)
	}

	assertContains(tt, string(mainContent), "Widget")
	assertNotContains(tt, string(mainContent), "TestWidget")
	assertNotContains(tt, string(mainContent), "//go:build")

	assertContains(tt, string(testContent), "TestWidget")
	assertContains(tt, string(testContent), "//go:build test")
	assertNotContains(tt, string(testContent), "\tWidget =") // base Widget should not appear (only TestWidget)
}

func TestExportPackageWithNegatedBuildTag(t *testing.T) {
	tt := test_ui.T{T: t}
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"),
		[]byte("module example.com/mod\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	pkgDir := filepath.Join(tmpDir, "internal", "alfa", "widget")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Base symbols (always present, including when !debug)
	if err := os.WriteFile(filepath.Join(pkgDir, "main.go"),
		[]byte("package widget\n\ntype Widget struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Symbols only present under !debug (absent when debug tag is active)
	if err := os.WriteFile(filepath.Join(pkgDir, "normal.go"),
		[]byte("//go:build !debug\n\npackage widget\n\ntype NormalWidget struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Symbols only present when debug is active
	if err := os.WriteFile(filepath.Join(pkgDir, "debug.go"),
		[]byte("//go:build debug\n\npackage widget\n\ntype DebugWidget struct{}\n"), 0o644); err != nil {
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

	// main.go: only symbols present across ALL combinations (Widget only;
	// NormalWidget disappears under debug, DebugWidget disappears without debug)
	mainContent, err := os.ReadFile(filepath.Join(tmpDir, "pkgs", "widget", "main.go"))
	if err != nil {
		t.Fatalf("main.go not generated: %v", err)
	}
	assertContains(tt, string(mainContent), "Widget")
	assertNotContains(tt, string(mainContent), "NormalWidget")
	assertNotContains(tt, string(mainContent), "DebugWidget")
	assertNotContains(tt, string(mainContent), "//go:build")

	// debug.go: DebugWidget (unique to debug build)
	debugContent, err := os.ReadFile(filepath.Join(tmpDir, "pkgs", "widget", "debug.go"))
	if err != nil {
		t.Fatalf("debug.go not generated: %v", err)
	}
	assertContains(tt, string(debugContent), "DebugWidget")
	assertContains(tt, string(debugContent), "//go:build debug")
	assertNotContains(tt, string(debugContent), "NormalWidget")

	// not_debug.go: NormalWidget (unique to !debug build)
	notDebugContent, err := os.ReadFile(filepath.Join(tmpDir, "pkgs", "widget", "not_debug.go"))
	if err != nil {
		t.Fatalf("not_debug.go not generated: %v", err)
	}
	assertContains(tt, string(notDebugContent), "NormalWidget")
	assertContains(tt, string(notDebugContent), "//go:build !debug")
	assertNotContains(tt, string(notDebugContent), "DebugWidget")
}

func TestBuildFlagsForExpression(t *testing.T) {
	cases := []struct{ expr, want string }{
		{"test", "test"},
		{"debug", "debug"},
		{"test && debug", "test,debug"},
		{"!debug", ""},
		{"!debug && test", "test"},
	}
	for _, c := range cases {
		got := buildFlagsForExpression(c.expr)
		if got != c.want {
			t.Errorf("buildFlagsForExpression(%q) = %q, want %q", c.expr, got, c.want)
		}
	}
}

func TestFilenameForExpression(t *testing.T) {
	cases := []struct{ expr, want string }{
		{"test", "test"},
		{"debug", "debug"},
		{"test && debug", "test_debug"},
		{"!debug", "not_debug"},
		{"!debug && test", "not_debug_test"},
	}
	for _, c := range cases {
		got := filenameForExpression(c.expr)
		if got != c.want {
			t.Errorf("filenameForExpression(%q) = %q, want %q", c.expr, got, c.want)
		}
	}
}

// TestExportPackageFacadeImportPath reproduces issue #96: when a package uses
// a type from another internal package as a generic constraint, the generated
// facade must import the pkgs/ facade path, not the internal/ source path.
func TestExportPackageFacadeImportPath(t *testing.T) {
	tt := test_ui.T{T: t}
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"),
		[]byte("module example.com/mod\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// internal/0/interfaces defines the Value constraint type.
	ifaceDir := filepath.Join(tmpDir, "internal", "0", "interfaces")
	if err := os.MkdirAll(ifaceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ifaceDir, "main.go"), []byte(`package interfaces

type Value interface {
	comparable
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// pkgs/interfaces is the facade for internal/0/interfaces.
	pkgsIfaceDir := filepath.Join(tmpDir, "pkgs", "interfaces")
	if err := os.MkdirAll(pkgsIfaceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgsIfaceDir, "main.go"), []byte(`// Code generated by dagnabit; DO NOT EDIT.
package interfaces

import internal "example.com/mod/internal/0/interfaces"

type (
	Value = internal.Value
)
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// internal/alfa/domain uses interfaces.Value as a generic constraint.
	domainDir := filepath.Join(tmpDir, "internal", "alfa", "domain")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(domainDir, "main.go"), []byte(`package domain

import "example.com/mod/internal/0/interfaces"

func Lookup[K interfaces.Value](m map[K]string, key K) string {
	return m[key]
}
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

	if err := exporter.ExportPackage("./internal/alfa/domain"); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "pkgs", "domain", "main.go"))
	if err != nil {
		t.Fatalf("pkgs/domain/main.go not generated: %v", err)
	}

	got := string(content)

	// The generic constraint must reference the pkgs/ facade path, not the
	// internal/ source path. (The facade itself still imports its own internal
	// package for delegation — that import is expected.)
	assertContains(tt, got, "example.com/mod/pkgs/interfaces")
	assertNotContains(tt, got, "example.com/mod/internal/0/interfaces")
}

// TestExportPackageSelfReferentialTypes reproduces issue #97: when a package
// defines named types used in its own generic function signatures, the remap
// must NOT redirect those types to the pkgs/ facade path (self-import cycle).
func TestExportPackageSelfReferentialTypes(t *testing.T) {
	tt := test_ui.T{T: t}
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"),
		[]byte("module example.com/mod\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	pkgDir := filepath.Join(tmpDir, "internal", "charlie", "hyphence")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "main.go"), []byte(`package hyphence

type CoderToTypedBlob[BLOB any] func(raw []byte) (BLOB, error)

type TypedBlob[BLOB any] struct {
	Value BLOB
}

func Wrap[BLOB any](coder CoderToTypedBlob[BLOB], raw []byte) (TypedBlob[BLOB], error) {
	v, err := coder(raw)
	return TypedBlob[BLOB]{Value: v}, err
}
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

	if err := exporter.ExportPackage("./internal/charlie/hyphence"); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "pkgs", "hyphence", "main.go"))
	if err != nil {
		t.Fatalf("pkgs/hyphence/main.go not generated: %v", err)
	}

	got := string(content)

	// Same-package named types must reference the internal alias, not the pkgs/ path.
	assertNotContains(tt, got, `"example.com/mod/pkgs/hyphence"`)
	assertContains(tt, got, "internal.CoderToTypedBlob")
	assertContains(tt, got, "internal.TypedBlob")
}

// TestExportPackageGeneratedFacadeCompiles verifies that generated facades
// actually compile. Text matching misses import cycles, missing imports, and
// other errors that only surface at build time.
func TestExportPackageGeneratedFacadeCompiles(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"),
		[]byte("module example.com/mod\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	pkgDir := filepath.Join(tmpDir, "internal", "charlie", "hyphence")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "main.go"), []byte(`package hyphence

type CoderToTypedBlob[BLOB any] func(raw []byte) (BLOB, error)

type TypedBlob[BLOB any] struct {
	Value BLOB
}

func Wrap[BLOB any](coder CoderToTypedBlob[BLOB], raw []byte) (TypedBlob[BLOB], error) {
	v, err := coder(raw)
	return TypedBlob[BLOB]{Value: v}, err
}
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

	if err := exporter.ExportPackage("./internal/charlie/hyphence"); err != nil {
		t.Fatal(err)
	}

	// Actually compile the generated pkgs/ output to catch import cycles and
	// other errors that assertContains/assertNotContains cannot detect.
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedFiles,
		Dir:  tmpDir,
		Env:  append(os.Environ(), "GOWORK=off"),
	}
	loaded, err := packages.Load(cfg, "./pkgs/...")
	if err != nil {
		t.Fatalf("loading generated pkgs: %v", err)
	}
	for _, pkg := range loaded {
		for _, e := range pkg.Errors {
			t.Errorf("package %s: %v", pkg.PkgPath, e)
		}
	}
}

func assertContains(t test_ui.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("output does not contain %q\n\ngot:\n%s", want, got)
	}
}

func assertNotContains(t test_ui.T, got, unwanted string) {
	t.Helper()
	if strings.Contains(got, unwanted) {
		t.Errorf("output should not contain %q\n\ngot:\n%s", unwanted, got)
	}
}
