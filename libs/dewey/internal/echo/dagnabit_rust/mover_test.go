package dagnabit_rust

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// requireCargoMetadata fails the test when the workspace at root no
// longer resolves under `cargo metadata`.
func requireCargoMetadata(t *testing.T, root string) {
	t.Helper()

	cmd := exec.Command("cargo", "metadata", "--format-version", "1", "--no-deps")
	cmd.Dir = root

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("workspace broken after move: %v\n%s", err, out)
	}
}

// rsContentHashes returns the sorted sha256 hex digests of every .rs
// file's contents under root. Paths are deliberately excluded — a move
// changes paths but must never change .rs contents.
func rsContentHashes(t *testing.T, root string) []string {
	t.Helper()

	var hashes []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".rs") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		sum := sha256.Sum256(data)
		hashes = append(hashes, hex.EncodeToString(sum[:]))

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	sort.Strings(hashes)

	return hashes
}

func TestMoverMovesCrateAndRewritesPathDeps(t *testing.T) {
	requireCargo(t)
	root := writeFixtureWorkspace(t) // git-initialized + committed

	m := &Mover{WorkspaceRoot: root}

	if err := m.MovePackage("internal/0/blob_store_id", "internal/alfa/blob_store_id"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(root, "internal/alfa/blob_store_id/Cargo.toml")); err != nil {
		t.Fatal("crate dir not moved")
	}

	dep, _ := os.ReadFile(filepath.Join(root, "internal/alfa/store/Cargo.toml"))
	if !strings.Contains(string(dep), `path = "../blob_store_id"`) {
		t.Errorf("dependent path-dep not rewritten:\n%s", dep)
	}

	ws, _ := os.ReadFile(filepath.Join(root, "Cargo.toml"))
	if !strings.Contains(string(ws), "internal/alfa/blob_store_id") {
		t.Errorf("workspace members not updated:\n%s", ws)
	}

	requireCargoMetadata(t, root)
}

func TestMoverNoRsFilesTouched(t *testing.T) {
	requireCargo(t)
	root := writeFixtureWorkspace(t)

	before := rsContentHashes(t, root)

	m := &Mover{WorkspaceRoot: root}

	if err := m.MovePackage("internal/0/blob_store_id", "internal/alfa/blob_store_id"); err != nil {
		t.Fatal(err)
	}

	after := rsContentHashes(t, root)

	if len(before) != len(after) {
		t.Fatalf(".rs file count changed: %d before, %d after", len(before), len(after))
	}

	for i := range before {
		if before[i] != after[i] {
			t.Fatalf(".rs contents changed: hash %d differs (%s != %s)", i, before[i], after[i])
		}
	}
}

func TestMoverRewritesMovedCratesOwnDeps(t *testing.T) {
	requireCargo(t)
	root := writeFixtureWorkspace(t)

	m := &Mover{WorkspaceRoot: root}

	// Move the DEPENDENT crate; its own path-dep on blob_store_id must
	// be recomputed from the new location so the workspace still
	// resolves.
	if err := m.MovePackage("internal/alfa/store", "internal/bravo/store"); err != nil {
		t.Fatal(err)
	}

	dep, err := os.ReadFile(filepath.Join(root, "internal/bravo/store/Cargo.toml"))
	if err != nil {
		t.Fatal("moved crate manifest missing:", err)
	}

	if !strings.Contains(string(dep), `path = "../../0/blob_store_id"`) {
		t.Errorf("moved crate's own path-dep not recomputed:\n%s", dep)
	}

	ws, _ := os.ReadFile(filepath.Join(root, "Cargo.toml"))
	if !strings.Contains(string(ws), "internal/bravo/store") {
		t.Errorf("workspace members not updated:\n%s", ws)
	}

	requireCargoMetadata(t, root)
}

func TestMoverRewritesMovedCratesOwnDepsAcrossDepthChange(t *testing.T) {
	requireCargo(t)
	root := writeFixtureWorkspace(t)

	m := &Mover{WorkspaceRoot: root}

	// Move the dependent crate to a SHALLOWER directory: the relative
	// base genuinely changes, so an unmodified `../../0/blob_store_id`
	// would no longer resolve. This catches movers that skip rewriting
	// the moved crate's own deps (the same-depth move above rewrites to
	// an identical string).
	if err := m.MovePackage("internal/alfa/store", "crates/store"); err != nil {
		t.Fatal(err)
	}

	dep, err := os.ReadFile(filepath.Join(root, "crates/store/Cargo.toml"))
	if err != nil {
		t.Fatal("moved crate manifest missing:", err)
	}

	if !strings.Contains(string(dep), `path = "../../internal/0/blob_store_id"`) {
		t.Errorf("moved crate's own path-dep not recomputed:\n%s", dep)
	}

	requireCargoMetadata(t, root)
}
