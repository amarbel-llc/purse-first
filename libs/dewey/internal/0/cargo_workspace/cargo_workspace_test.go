package cargo_workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFindRootAtWorkspaceDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Cargo.toml"), `
[workspace]
members = ["internal/0/blob_store_id", "pkgs/blob_store_id"]
resolver = "2"
`)

	ws, err := FindRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ws.RootDir != dir {
		t.Errorf("RootDir = %q, want %q", ws.RootDir, dir)
	}
	if len(ws.Members) != 2 || ws.Members[0] != "internal/0/blob_store_id" {
		t.Errorf("Members = %v", ws.Members)
	}
}

func TestFindRootWalksUpPastMemberManifest(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Cargo.toml"), "[workspace]\nmembers = [\"a\"]\n")
	// member crate has its own Cargo.toml WITHOUT [workspace]
	writeFile(t, filepath.Join(dir, "a", "Cargo.toml"), "[package]\nname = \"a\"\nversion = \"0.1.0\"\n")

	ws, err := FindRoot(filepath.Join(dir, "a"))
	if err != nil {
		t.Fatal(err)
	}
	if ws.RootDir != dir {
		t.Errorf("RootDir = %q, want %q", ws.RootDir, dir)
	}
}

func TestFindRootErrorsWithoutWorkspace(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Cargo.toml"), "[package]\nname = \"solo\"\nversion = \"0.1.0\"\n")

	if _, err := FindRoot(dir); err == nil {
		t.Fatal("expected error for crate without workspace root")
	}
}

func TestFindRootErrorsWithoutAnyManifest(t *testing.T) {
	if _, err := FindRoot(t.TempDir()); err == nil {
		t.Fatal("expected error when no Cargo.toml exists")
	}
}
