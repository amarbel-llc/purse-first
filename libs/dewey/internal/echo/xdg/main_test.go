package xdg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/amarbel-llc/purse-first/libs/dewey/internal/alfa/test_ui"
)

func mustMkdirAll(t test_ui.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func TestGetCwdXDGOverridePathChecksCeilingDir(t *testing.T) {
	tt := test_ui.T{T: t}
	tmp := tt.TempDir()

	root := filepath.Join(tmp, "x")
	leaf := filepath.Join(root, "leaf")
	marker := filepath.Join(root, ".myutil")

	mustMkdirAll(tt, leaf)
	mustMkdirAll(tt, marker)

	tt.Setenv(CeilingEnvVarName("myutil"), root)

	initArgs := InitArgs{
		Cwd:         leaf,
		UtilityName: "myutil",
	}

	got, ok := initArgs.getCwdXDGOverridePath()
	if !ok {
		tt.Fatalf("expected to find override at %s, got (%s, false)", root, got)
	}
	if got != root {
		tt.Fatalf("expected override path %s, got %s", root, got)
	}
}

func TestGetCwdXDGOverridePathStopsAboveCeiling(t *testing.T) {
	tt := test_ui.T{T: t}
	tmp := tt.TempDir()

	above := filepath.Join(tmp, "above")
	ceiling := filepath.Join(above, "x")
	leaf := filepath.Join(ceiling, "leaf")
	stray := filepath.Join(above, ".myutil")

	mustMkdirAll(tt, leaf)
	mustMkdirAll(tt, stray)

	tt.Setenv(CeilingEnvVarName("myutil"), ceiling)

	initArgs := InitArgs{
		Cwd:         leaf,
		UtilityName: "myutil",
	}

	if got, ok := initArgs.getCwdXDGOverridePath(); ok {
		tt.Fatalf("expected no override (marker is above ceiling), got (%s, true)", got)
	}
}

func TestGetCwdXDGOverridePathFindsMarkerAtCwd(t *testing.T) {
	tt := test_ui.T{T: t}
	tmp := tt.TempDir()

	cwd := filepath.Join(tmp, "x")
	marker := filepath.Join(cwd, ".myutil")

	mustMkdirAll(tt, marker)

	tt.Setenv(CeilingEnvVarName("myutil"), cwd)

	initArgs := InitArgs{
		Cwd:         cwd,
		UtilityName: "myutil",
	}

	got, ok := initArgs.getCwdXDGOverridePath()
	if !ok {
		tt.Fatalf("expected to find override at %s, got (%s, false)", cwd, got)
	}
	if got != cwd {
		tt.Fatalf("expected override path %s, got %s", cwd, got)
	}
}

// Use a long random-ish utility name so the unbounded walk to /
// can't match any real directory on the dev machine.
const uniqueUtility = "dewey-xdg-test-no-ceiling-7f3a9b"

func TestGetCwdXDGOverridePathWithoutCeilingWalksUp(t *testing.T) {
	tt := test_ui.T{T: t}
	tmp := tt.TempDir()

	root := filepath.Join(tmp, "x")
	leaf := filepath.Join(root, "y", "z")
	marker := filepath.Join(root, "."+uniqueUtility)

	mustMkdirAll(tt, leaf)
	mustMkdirAll(tt, marker)

	tt.Setenv(CeilingEnvVarName(uniqueUtility), "")

	initArgs := InitArgs{
		Cwd:         leaf,
		UtilityName: uniqueUtility,
	}

	got, ok := initArgs.getCwdXDGOverridePath()
	if !ok {
		tt.Fatalf("expected to find override at %s, got (%s, false)", root, got)
	}
	if got != root {
		tt.Fatalf("expected override path %s, got %s", root, got)
	}
}

// Regression for #80: when the walked `dir` and the ceiling reach the same
// canonical directory through different symlink chains, IsAboveCeiling must
// match git's GIT_CEILING_DIRECTORIES contract and resolve symlinks on both
// sides before comparing.
//
// IsAboveCeiling answers "is dir strictly an ancestor of any ceiling entry"
// — i.e. the walk has gone past the ceiling and should stop. The bug shape
// from the issue is that dir and ceiling share a canonical prefix but are
// expressed through different symlink chains, so the lexical comparison
// misses the relationship entirely.
func TestIsAboveCeilingResolvesSymlinks(t *testing.T) {
	tt := test_ui.T{T: t}
	tmp := tt.TempDir()

	realCeiling := filepath.Join(tmp, "real-ceiling")
	mustMkdirAll(tt, filepath.Join(realCeiling, "leaf"))

	linkToCeiling := filepath.Join(tmp, "link-ceiling")
	if err := os.Symlink(realCeiling, linkToCeiling); err != nil {
		tt.Fatalf("symlink %s -> %s: %v", linkToCeiling, realCeiling, err)
	}

	// dir == ceiling via different symlink chains.
	if !IsAtOrAboveCeiling(realCeiling, []string{linkToCeiling}) {
		tt.Fatalf(
			"expected %s to be at-or-above ceiling %s (same canonical path via symlink)",
			realCeiling, linkToCeiling,
		)
	}

	// dir is a strict ancestor of ceiling via different symlink chains.
	above := filepath.Dir(realCeiling)
	if !IsAboveCeiling(above, []string{linkToCeiling}) {
		tt.Fatalf(
			"expected %s to be above ceiling %s (canonical parent via symlink)",
			above, linkToCeiling,
		)
	}
}

// A ceiling entry that doesn't exist on disk should still bound the walk by
// its cleaned string form rather than being silently dropped.
func TestIsAboveCeilingFallsBackToCleanForNonExistentEntry(t *testing.T) {
	tmp := t.TempDir()

	missing := filepath.Join(tmp, "does-not-exist")
	leaf := filepath.Join(missing, "leaf")

	if IsAboveCeiling(leaf, []string{missing}) {
		t.Fatalf("did not expect %s to be above %s", leaf, missing)
	}
	if !IsAtOrAboveCeiling(missing, []string{missing}) {
		t.Fatalf("expected %s to be at-or-above itself", missing)
	}
}

func TestGetCwdXDGOverridePathHonorsMultipleCeilings(t *testing.T) {
	tt := test_ui.T{T: t}
	tmp := tt.TempDir()

	otherCeiling := filepath.Join(tmp, "other")
	relevantCeiling := filepath.Join(tmp, "relevant")
	leaf := filepath.Join(relevantCeiling, "leaf")
	marker := filepath.Join(relevantCeiling, ".myutil")

	mustMkdirAll(tt, otherCeiling)
	mustMkdirAll(tt, leaf)
	mustMkdirAll(tt, marker)

	tt.Setenv(
		CeilingEnvVarName("myutil"),
		otherCeiling+string(filepath.ListSeparator)+relevantCeiling,
	)

	initArgs := InitArgs{
		Cwd:         leaf,
		UtilityName: "myutil",
	}

	got, ok := initArgs.getCwdXDGOverridePath()
	if !ok {
		tt.Fatalf("expected to find override at %s, got (%s, false)", relevantCeiling, got)
	}
	if got != relevantCeiling {
		tt.Fatalf("expected override path %s, got %s", relevantCeiling, got)
	}
}
