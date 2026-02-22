package flake

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasDevShell_Present(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "flake.nix"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !HasDevShell(dir) {
		t.Error("expected HasDevShell to return true when flake.nix exists")
	}
}

func TestHasDevShell_Absent(t *testing.T) {
	dir := t.TempDir()

	if HasDevShell(dir) {
		t.Error("expected HasDevShell to return false when flake.nix is absent")
	}
}
