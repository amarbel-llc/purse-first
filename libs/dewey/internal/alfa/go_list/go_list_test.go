package go_list

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	topological_sort "code.linenisgreat.com/purse-first/libs/dewey/internal/0/topological_sort"
)

// writeFile creates a file with parents under dir.
func writeFile(t *testing.T, dir, relPath, content string) {
	t.Helper()

	abs := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeFlatModule creates a minimal module with a flat internal/<pkg> layout:
//
//	<tmp>/go.mod                 (module example.com/m)
//	<tmp>/internal/foo/foo.go    (package foo)
//	<tmp>/internal/bar/bar.go    (package bar; imports foo)
func writeFlatModule(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	writeFile(t, dir, "go.mod", "module example.com/m\n\ngo 1.21\n")
	// Shadow any outer go.work (nix-shell's TMPDIR is inside the worktree).
	writeFile(t, dir, "go.work", "go 1.21\n\nuse .\n")

	writeFile(
		t,
		dir,
		"internal/foo/foo.go",
		"package foo\n\nfunc X() int { return 1 }\n",
	)

	writeFile(
		t,
		dir,
		"internal/bar/bar.go",
		`package bar

import "example.com/m/internal/foo"

func Y() int { return foo.X() }
`,
	)

	return dir
}

// writeTieredModule creates a minimal module with a prefix/level/package layout:
//
//	<tmp>/internal/alfa/foo/foo.go
//	<tmp>/internal/bravo/bar/bar.go (imports foo)
func writeTieredModule(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	writeFile(t, dir, "go.mod", "module example.com/m\n\ngo 1.21\n")
	writeFile(t, dir, "go.work", "go 1.21\n\nuse .\n")

	writeFile(
		t,
		dir,
		"internal/alfa/foo/foo.go",
		"package foo\n\nfunc X() int { return 1 }\n",
	)

	writeFile(
		t,
		dir,
		"internal/bravo/bar/bar.go",
		`package bar

import "example.com/m/internal/alfa/foo"

func Y() int { return foo.X() }
`,
	)

	return dir
}

func TestGoListReaderErrorsOnFlatLayoutWithDefaultDepth(t *testing.T) {
	dir := writeFlatModule(t)

	reader := Reader{
		Dir:             dir,
		ModulePath:      "example.com/m",
		PackagePrefixes: []string{"internal"},
		// ComponentDepth defaults to 3 (via componentDepth()).
	}

	_, err := reader.ReadDependencies()
	if err == nil {
		t.Fatal("expected error for flat layout at default depth=3, got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "no edges computed") {
		t.Errorf("expected error to mention %q, got %q", "no edges computed", msg)
	}

	if !strings.Contains(msg, "--initial") {
		t.Errorf("expected error to hint at --initial, got %q", msg)
	}
}

func TestGoListReaderFlatLayoutDepth2Succeeds(t *testing.T) {
	dir := writeFlatModule(t)

	reader := Reader{
		Dir:             dir,
		ModulePath:      "example.com/m",
		PackagePrefixes: []string{"internal"},
		ComponentDepth:  2,
	}

	edgesByPrefix, err := reader.ReadDependencies()
	if err != nil {
		t.Fatalf("unexpected error at depth=2: %v", err)
	}

	assertEdges(t, edgesByPrefix["internal"],
		topological_sort.Edge{Source: "internal/bar", Target: "internal/foo"},
		topological_sort.Edge{Source: "internal/bar"},
		topological_sort.Edge{Source: "internal/foo"},
	)
}

// Packages with no in-prefix edges (imported only from outside the prefix,
// only from _test.go files, or not at all) must still be reported, as node
// declarations, so the repositioner plans them (purse-first#57, #220).
func TestGoListReaderDeclaresPackagesWithoutInPrefixEdges(t *testing.T) {
	dir := writeFlatModule(t)

	writeFile(t, dir, "internal/servelog/servelog.go", "package servelog\n\nfunc Log() {}\n")
	writeFile(t, dir, "internal/testfs/testfs.go", "package testfs\n\nfunc Dir() string { return \"\" }\n")
	writeFile(t, dir, "internal/resources/resources.go", "package resources\n\nconst Name = \"r\"\n")
	writeFile(t, dir, "internal/bar/bar_test.go", `package bar

import (
	"testing"

	"example.com/m/internal/testfs"
)

func TestY(t *testing.T) { _ = testfs.Dir() }
`)
	writeFile(t, dir, "cmd/app/main.go", `package main

import "example.com/m/internal/servelog"

func main() { servelog.Log() }
`)

	reader := Reader{
		Dir:             dir,
		ModulePath:      "example.com/m",
		PackagePrefixes: []string{"internal"},
		ComponentDepth:  2,
	}

	edgesByPrefix, err := reader.ReadDependencies()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEdges(t, edgesByPrefix["internal"],
		topological_sort.Edge{Source: "internal/bar", Target: "internal/foo"},
		topological_sort.Edge{Source: "internal/bar"},
		topological_sort.Edge{Source: "internal/foo"},
		topological_sort.Edge{Source: "internal/resources"},
		topological_sort.Edge{Source: "internal/servelog"},
		topological_sort.Edge{Source: "internal/testfs"},
	)
}

// assertEdges compares edges against want as sets (go list order is not
// part of the contract).
func assertEdges(t *testing.T, edges []topological_sort.Edge, want ...topological_sort.Edge) {
	t.Helper()

	got := make(map[topological_sort.Edge]bool, len(edges))
	for _, e := range edges {
		got[e] = true
	}

	if len(got) != len(edges) {
		t.Errorf("duplicate edges in %v", edges)
	}

	for _, w := range want {
		if !got[w] {
			t.Errorf("missing edge %+v in %v", w, edges)
		}
	}

	if len(edges) != len(want) {
		t.Errorf("expected %d edges, got %d: %v", len(want), len(edges), edges)
	}
}

func TestGoListReaderTieredLayoutDepth3Succeeds(t *testing.T) {
	dir := writeTieredModule(t)

	reader := Reader{
		Dir:             dir,
		ModulePath:      "example.com/m",
		PackagePrefixes: []string{"internal"},
		// ComponentDepth defaults to 3.
	}

	edgesByPrefix, err := reader.ReadDependencies()
	if err != nil {
		t.Fatalf("unexpected error at depth=3 with tiered layout: %v", err)
	}

	assertEdges(t, edgesByPrefix["internal"],
		topological_sort.Edge{Source: "internal/bravo/bar", Target: "internal/alfa/foo"},
		topological_sort.Edge{Source: "internal/bravo/bar"},
		topological_sort.Edge{Source: "internal/alfa/foo"},
	)
}

func TestGoListReaderVerboseLogsSkippedSources(t *testing.T) {
	dir := writeFlatModule(t)

	reader := Reader{
		Dir:             dir,
		ModulePath:      "example.com/m",
		PackagePrefixes: []string{"internal"},
		Verbose:         true,
	}

	stderr := captureStderr(t, func() {
		_, _ = reader.ReadDependencies()
	})

	if !strings.Contains(stderr, "dagnabit: skipping") {
		t.Errorf("expected verbose stderr to contain %q, got:\n%s", "dagnabit: skipping", stderr)
	}

	if !strings.Contains(stderr, "internal/foo") {
		t.Errorf("expected stderr to mention internal/foo, got:\n%s", stderr)
	}

	if !strings.Contains(stderr, "internal/bar") {
		t.Errorf("expected stderr to mention internal/bar, got:\n%s", stderr)
	}
}

func TestCountComponents(t *testing.T) {
	cases := []struct {
		path string
		want int
	}{
		{"", 0},
		{"foo", 1},
		{"foo/bar", 2},
		{"foo/bar/baz", 3},
	}

	for _, c := range cases {
		got := countComponents(c.path)
		if got != c.want {
			t.Errorf("countComponents(%q) = %d, want %d", c.path, got, c.want)
		}
	}
}

// captureStderr redirects os.Stderr for the duration of fn, returning the
// captured output.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	originalStderr := os.Stderr

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	os.Stderr = w

	done := make(chan struct{})
	buf := &bytes.Buffer{}

	go func() {
		_, _ = io.Copy(buf, r)
		close(done)
	}()

	fn()

	_ = w.Close()
	os.Stderr = originalStderr

	<-done
	_ = r.Close()

	return buf.String()
}

// ensure writeFile compiles with filepath on every platform (keeps import
// discoverable if the test file is read in isolation).
var _ = filepath.Join
