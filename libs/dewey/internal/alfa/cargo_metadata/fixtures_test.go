package cargo_metadata

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// requireCargo skips the calling test when cargo is not on PATH.
func requireCargo(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("cargo"); err != nil {
		t.Skip("cargo not on PATH")
	}
}

// writeFixture creates a file with parents under dir.
func writeFixture(t *testing.T, dir, relPath, content string) {
	t.Helper()

	abs := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeFixtureWorkspace creates a real cargo workspace in a temp dir with a
// tiered internal/<level>/<crate> layout:
//
//	<tmp>/Cargo.toml                              (virtual [workspace])
//	<tmp>/internal/0/blob_store_id/               (blob_store_id_internal)
//	<tmp>/internal/alfa/store/                    (store_internal; path-deps on blob_store_id_internal)
//
// The fixture is only useful when cargo can read it, so it requires cargo
// on PATH (skipping the calling test otherwise).
func writeFixtureWorkspace(t *testing.T) string {
	t.Helper()

	requireCargo(t)

	root := t.TempDir()

	writeFixture(t, root, "Cargo.toml", `[workspace]
members = ["internal/0/blob_store_id", "internal/alfa/store"]
resolver = "2"
`)

	writeFixture(t, root, "internal/0/blob_store_id/Cargo.toml", `[package]
name = "blob_store_id_internal"
version = "0.1.0"
edition = "2021"
`)

	writeFixture(
		t,
		root,
		"internal/0/blob_store_id/src/lib.rs",
		"pub fn make_id() -> u32 { 7 }\n",
	)

	writeFixture(t, root, "internal/alfa/store/Cargo.toml", `[package]
name = "store_internal"
version = "0.1.0"
edition = "2021"

[dependencies]
blob_store_id_internal = { path = "../../0/blob_store_id" }
`)

	writeFixture(
		t,
		root,
		"internal/alfa/store/src/lib.rs",
		"pub fn make() -> u32 { blob_store_id_internal::make_id() }\n",
	)

	return root
}
