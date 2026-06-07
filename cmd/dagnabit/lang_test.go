package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeLangFixture creates a file with parents under dir.
func writeLangFixture(t *testing.T, dir, relPath, content string) {
	t.Helper()

	abs := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// hermeticTempDir returns a temp dir whose ancestors contain neither a
// go.mod nor a workspace Cargo.toml. t.TempDir() cannot be used for
// walk-to-filesystem-root tests here: $TMPDIR points inside this repo's
// worktree (.tmp/), so the repo's own go.mod is always found in an
// ancestor. The literal "/tmp" base is deliberate for the same reason —
// os.TempDir() would honor that $TMPDIR and recreate the problem.
func hermeticTempDir(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("/tmp", "dagnabit-lang-test-")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { os.RemoveAll(dir) })

	return dir
}

const workspaceManifest = `[workspace]
members = ["internal/0/blob_store_id"]
resolver = "2"
`

const crateManifest = `[package]
name = "blob_store_id_internal"
version = "0.1.0"
edition = "2021"
`

func TestDetectGo(t *testing.T) {
	root := t.TempDir()
	writeLangFixture(t, root, "go.mod", "module example.com/m\n\ngo 1.26\n")

	lang, rootDir, err := detectLanguage(root, "")
	if err != nil {
		t.Fatal(err)
	}

	if lang != langGo {
		t.Errorf("lang = %v, want langGo", lang)
	}

	if rootDir != root {
		t.Errorf("rootDir = %q, want %q", rootDir, root)
	}
}

func TestDetectRust(t *testing.T) {
	root := t.TempDir()
	writeLangFixture(t, root, "Cargo.toml", workspaceManifest)

	nested := filepath.Join(root, "internal", "0", "blob_store_id")
	writeLangFixture(t, root, "internal/0/blob_store_id/Cargo.toml", crateManifest)

	lang, rootDir, err := detectLanguage(nested, "")
	if err != nil {
		t.Fatal(err)
	}

	if lang != langRust {
		t.Errorf("lang = %v, want langRust", lang)
	}

	if rootDir != root {
		t.Errorf("rootDir = %q, want %q", rootDir, root)
	}
}

func TestDetectBothErrors(t *testing.T) {
	root := t.TempDir()
	writeLangFixture(t, root, "go.mod", "module example.com/m\n\ngo 1.26\n")
	writeLangFixture(t, root, "Cargo.toml", workspaceManifest)

	_, _, err := detectLanguage(root, "")
	if err == nil {
		t.Fatal("expected error for go.mod + workspace Cargo.toml at same level")
	}

	if !strings.Contains(err.Error(), "-lang") {
		t.Errorf("error %q does not name the -lang flag", err)
	}
}

func TestDetectNeitherErrors(t *testing.T) {
	root := hermeticTempDir(t)

	_, _, err := detectLanguage(root, "")
	if err == nil {
		t.Fatal("expected error when neither marker exists")
	}

	if !strings.Contains(err.Error(), "-lang") {
		t.Errorf("error %q does not name the -lang flag", err)
	}
}

func TestFlagOverridesDetection(t *testing.T) {
	root := t.TempDir()
	writeLangFixture(t, root, "go.mod", "module example.com/m\n\ngo 1.26\n")
	writeLangFixture(t, root, "Cargo.toml", workspaceManifest)

	lang, rootDir, err := detectLanguage(root, "rust")
	if err != nil {
		t.Fatal(err)
	}

	if lang != langRust {
		t.Errorf("lang = %v, want langRust", lang)
	}

	if rootDir != root {
		t.Errorf("rootDir = %q, want %q", rootDir, root)
	}
}

func TestFlagRustWithoutWorkspaceErrors(t *testing.T) {
	root := hermeticTempDir(t)
	writeLangFixture(t, root, "go.mod", "module example.com/m\n\ngo 1.26\n")

	_, _, err := detectLanguage(root, "rust")
	if err == nil {
		t.Fatal("expected error: -lang rust with no workspace Cargo.toml anywhere up the tree")
	}
}

func TestInvalidFlagErrors(t *testing.T) {
	root := t.TempDir()

	_, _, err := detectLanguage(root, "fortran")
	if err == nil {
		t.Fatal("expected error for invalid -lang value")
	}

	if !strings.Contains(err.Error(), "-lang") {
		t.Errorf("error %q does not name the -lang flag", err)
	}
}

func TestCrateManifestWithoutWorkspaceIsNotRust(t *testing.T) {
	root := t.TempDir()
	writeLangFixture(t, root, "go.mod", "module example.com/m\n\ngo 1.26\n")

	crateDir := filepath.Join(root, "vendor", "blob_store_id")
	writeLangFixture(t, root, "vendor/blob_store_id/Cargo.toml", crateManifest)

	lang, rootDir, err := detectLanguage(crateDir, "")
	if err != nil {
		t.Fatal(err)
	}

	if lang != langGo {
		t.Errorf("lang = %v, want langGo (crate-only Cargo.toml must keep walking)", lang)
	}

	if rootDir != root {
		t.Errorf("rootDir = %q, want %q", rootDir, root)
	}
}
