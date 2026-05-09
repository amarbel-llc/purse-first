package xdg

import (
	"os"
	"path/filepath"
	"testing"
)

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func TestGetCwdXDGOverridePathChecksCeilingDir(t *testing.T) {
	tmp := t.TempDir()

	root := filepath.Join(tmp, "x")
	leaf := filepath.Join(root, "leaf")
	marker := filepath.Join(root, ".myutil")

	mustMkdirAll(t, leaf)
	mustMkdirAll(t, marker)

	t.Setenv(CeilingEnvVarName("myutil"), root)

	initArgs := InitArgs{
		Cwd:         leaf,
		UtilityName: "myutil",
	}

	got, ok := initArgs.getCwdXDGOverridePath()
	if !ok {
		t.Fatalf("expected to find override at %s, got (%s, false)", root, got)
	}
	if got != root {
		t.Fatalf("expected override path %s, got %s", root, got)
	}
}

func TestGetCwdXDGOverridePathStopsAboveCeiling(t *testing.T) {
	tmp := t.TempDir()

	above := filepath.Join(tmp, "above")
	ceiling := filepath.Join(above, "x")
	leaf := filepath.Join(ceiling, "leaf")
	stray := filepath.Join(above, ".myutil")

	mustMkdirAll(t, leaf)
	mustMkdirAll(t, stray)

	t.Setenv(CeilingEnvVarName("myutil"), ceiling)

	initArgs := InitArgs{
		Cwd:         leaf,
		UtilityName: "myutil",
	}

	if got, ok := initArgs.getCwdXDGOverridePath(); ok {
		t.Fatalf("expected no override (marker is above ceiling), got (%s, true)", got)
	}
}

func TestGetCwdXDGOverridePathFindsMarkerAtCwd(t *testing.T) {
	tmp := t.TempDir()

	cwd := filepath.Join(tmp, "x")
	marker := filepath.Join(cwd, ".myutil")

	mustMkdirAll(t, marker)

	t.Setenv(CeilingEnvVarName("myutil"), cwd)

	initArgs := InitArgs{
		Cwd:         cwd,
		UtilityName: "myutil",
	}

	got, ok := initArgs.getCwdXDGOverridePath()
	if !ok {
		t.Fatalf("expected to find override at %s, got (%s, false)", cwd, got)
	}
	if got != cwd {
		t.Fatalf("expected override path %s, got %s", cwd, got)
	}
}

// Use a long random-ish utility name so the unbounded walk to /
// can't match any real directory on the dev machine.
const uniqueUtility = "dewey-xdg-test-no-ceiling-7f3a9b"

func TestGetCwdXDGOverridePathWithoutCeilingWalksUp(t *testing.T) {
	tmp := t.TempDir()

	root := filepath.Join(tmp, "x")
	leaf := filepath.Join(root, "y", "z")
	marker := filepath.Join(root, "."+uniqueUtility)

	mustMkdirAll(t, leaf)
	mustMkdirAll(t, marker)

	t.Setenv(CeilingEnvVarName(uniqueUtility), "")

	initArgs := InitArgs{
		Cwd:         leaf,
		UtilityName: uniqueUtility,
	}

	got, ok := initArgs.getCwdXDGOverridePath()
	if !ok {
		t.Fatalf("expected to find override at %s, got (%s, false)", root, got)
	}
	if got != root {
		t.Fatalf("expected override path %s, got %s", root, got)
	}
}

func TestGetCwdXDGOverridePathHonorsMultipleCeilings(t *testing.T) {
	tmp := t.TempDir()

	otherCeiling := filepath.Join(tmp, "other")
	relevantCeiling := filepath.Join(tmp, "relevant")
	leaf := filepath.Join(relevantCeiling, "leaf")
	marker := filepath.Join(relevantCeiling, ".myutil")

	mustMkdirAll(t, otherCeiling)
	mustMkdirAll(t, leaf)
	mustMkdirAll(t, marker)

	t.Setenv(
		CeilingEnvVarName("myutil"),
		otherCeiling+string(filepath.ListSeparator)+relevantCeiling,
	)

	initArgs := InitArgs{
		Cwd:         leaf,
		UtilityName: "myutil",
	}

	got, ok := initArgs.getCwdXDGOverridePath()
	if !ok {
		t.Fatalf("expected to find override at %s, got (%s, false)", relevantCeiling, got)
	}
	if got != relevantCeiling {
		t.Fatalf("expected override path %s, got %s", relevantCeiling, got)
	}
}
