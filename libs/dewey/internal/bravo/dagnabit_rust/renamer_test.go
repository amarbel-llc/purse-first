package dagnabit_rust

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/amarbel-llc/purse-first/libs/dewey/internal/alfa/test_ui"
)

// captureStdout runs fn with os.Stdout redirected to a pipe and returns
// the captured bytes. Copied from dagnabit's events_test.go (echo→echo
// imports are forbidden by the tier convention, test helpers included).
func captureStdout(t test_ui.T, fn func()) []byte {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	orig := os.Stdout
	os.Stdout = w

	defer func() { os.Stdout = orig }()

	done := make(chan []byte)

	go func() {
		data, _ := io.ReadAll(r)
		done <- data
	}()

	fn()

	w.Close()

	return <-done
}

// sliceLevelMapper is a deterministic LevelMapper stub, copied from
// dagnabit's repositioner_test.go (same echo→echo import restriction as
// captureStdout above).
type sliceLevelMapper struct {
	levels []string
}

func (m sliceLevelMapper) LevelName(height int) (string, error) {
	if height < 0 || height >= len(m.levels) {
		return "", fmt.Errorf("height %d out of range", height)
	}

	return m.levels[height], nil
}

func (m sliceLevelMapper) LevelIndex(name string) (int, error) {
	for i, n := range m.levels {
		if n == name {
			return i, nil
		}
	}

	return 0, fmt.Errorf("unknown level %q", name)
}

// requireCargoCheck fails the test when `cargo check --workspace` fails
// at root.
func requireCargoCheck(t test_ui.T, root string) {
	t.Helper()

	cmd := exec.Command("cargo", "check", "--workspace")
	cmd.Dir = root

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cargo check --workspace failed: %v\n%s", err, out)
	}
}

// allContentHashes maps every file's workspace-relative path to its
// sha256, excluding .git/ (whose mtime-sensitive internals are not part
// of the tree under test). Unlike rsContentHashes, paths are included:
// a dry run must change neither contents nor layout.
func allContentHashes(t test_ui.T, root string) map[string]string {
	t.Helper()

	hashes := make(map[string]string)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}

			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		sum := sha256.Sum256(data)
		hashes[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	return hashes
}

func TestMoveRenameRewritesDependentSources(t *testing.T) {
	tt := test_ui.T{T: t}
	requireCargo(tt)
	requireAstGrep(tt)
	root := writeFixtureWorkspace(tt)

	r := &Renamer{WorkspaceRoot: root}

	err := r.MoveRename("internal/0/blob_store_id", "internal/0/blob_id", Options{})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(root, "internal/0/blob_id/Cargo.toml")); err != nil {
		t.Fatal("crate dir not moved:", err)
	}

	if _, err := os.Stat(filepath.Join(root, "internal/0/blob_store_id")); !os.IsNotExist(err) {
		t.Errorf("old crate dir still present (err: %v)", err)
	}

	moved := readFixtureFile(tt, root, "internal/0/blob_id/Cargo.toml")
	if !strings.Contains(moved, `name = "blob_id_internal"`) {
		t.Errorf("moved crate [package] name not renamed:\n%s", moved)
	}

	dep := readFixtureFile(tt, root, "internal/alfa/store/Cargo.toml")

	if !strings.Contains(dep, "blob_id_internal") {
		t.Errorf("dependent dep key not renamed:\n%s", dep)
	}

	if !strings.Contains(dep, `path = "../../0/blob_id"`) {
		t.Errorf("dependent path-dep not updated:\n%s", dep)
	}

	lib := readFixtureFile(tt, root, "internal/alfa/store/src/lib.rs")

	if strings.Contains(lib, "blob_store_id_internal") {
		t.Errorf("dependent sources still reference the old crate name:\n%s", lib)
	}

	for _, want := range []string{
		"use blob_id_internal::make_id as mk;",
		"blob_id_internal::make_id()",
	} {
		if !strings.Contains(lib, want) {
			t.Errorf("dependent sources missing %q:\n%s", want, lib)
		}
	}

	requireCargoCheck(tt, root)
}

func TestMoveRenamePreflightGateAbortsOnBrokenWorkspace(t *testing.T) {
	tt := test_ui.T{T: t}
	requireCargo(tt)
	requireAstGrep(tt)
	root := writeFixtureWorkspace(tt)

	libPath := filepath.Join(root, "internal/alfa/store/src/lib.rs")
	if err := os.WriteFile(libPath, []byte("pub fn broken( {\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifestBefore := readFixtureFile(tt, root, "internal/alfa/store/Cargo.toml")

	r := &Renamer{WorkspaceRoot: root}

	err := r.MoveRename("internal/0/blob_store_id", "internal/0/blob_id", Options{})
	if err == nil {
		t.Fatal("expected pre-flight error on broken workspace, got nil")
	}

	if !strings.Contains(err.Error(), "cargo check") {
		t.Errorf("error does not mention cargo check: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(root, "internal/0/blob_store_id/Cargo.toml")); statErr != nil {
		t.Error("src crate dir mutated despite failed pre-flight:", statErr)
	}

	if _, statErr := os.Stat(filepath.Join(root, "internal/0/blob_id")); !os.IsNotExist(statErr) {
		t.Errorf("dst crate dir created despite failed pre-flight (err: %v)", statErr)
	}

	if got := readFixtureFile(tt, root, "internal/alfa/store/Cargo.toml"); got != manifestBefore {
		t.Errorf("dependent manifest mutated despite failed pre-flight:\n%s", got)
	}
}

func TestMoveRenameForceSkipsPreflight(t *testing.T) {
	tt := test_ui.T{T: t}
	requireCargo(tt)
	requireAstGrep(tt)
	root := writeFixtureWorkspace(tt)

	// Break a dependent source. Without Force the pre-flight rejects
	// this workspace (test above); with Force the rename must proceed
	// past the pre-flight and fail only at the post-flight gate. The
	// moved directory is the proof the pre-flight was skipped.
	libPath := filepath.Join(root, "internal/alfa/store/src/lib.rs")
	if err := os.WriteFile(libPath, []byte("pub fn broken( {\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &Renamer{WorkspaceRoot: root}

	err := r.MoveRename(
		"internal/0/blob_store_id",
		"internal/0/blob_id",
		Options{Force: true},
	)
	if err == nil {
		t.Fatal("expected post-flight error on broken workspace, got nil")
	}

	if strings.Contains(err.Error(), "pre-flight") {
		t.Errorf("Force did not skip the pre-flight gate: %v", err)
	}

	if !strings.Contains(err.Error(), "post-flight") {
		t.Errorf("error is not from the post-flight gate: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(root, "internal/0/blob_id/Cargo.toml")); statErr != nil {
		t.Error("crate dir not moved; rename never got past the pre-flight:", statErr)
	}
}

func TestMoveWithoutLeafRenameTouchesNoRs(t *testing.T) {
	tt := test_ui.T{T: t}
	requireCargo(tt)
	requireAstGrep(tt)
	root := writeFixtureWorkspace(tt)

	before := rsContentHashes(tt, root)

	r := &Renamer{WorkspaceRoot: root}

	err := r.MoveRename("internal/0/blob_store_id", "internal/alfa/blob_store_id", Options{})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(root, "internal/alfa/blob_store_id/Cargo.toml")); err != nil {
		t.Fatal("crate dir not moved:", err)
	}

	after := rsContentHashes(tt, root)

	if len(before) != len(after) {
		t.Fatalf(".rs file count changed: %d before, %d after", len(before), len(after))
	}

	for i := range before {
		if before[i] != after[i] {
			t.Fatalf(".rs contents changed: hash %d differs (%s != %s)", i, before[i], after[i])
		}
	}
}

func TestRenameComputesRequiredLevel(t *testing.T) {
	tt := test_ui.T{T: t}
	requireCargo(tt)
	requireAstGrep(tt)

	// store sits at the WRONG level (0) while depending on
	// blob_store_id at level 0; its dep height is 1, so Rename must
	// land it at the mapper's level-1 name ("alfa").
	root := t.TempDir()

	gitInTestRepo(tt, root, "init")
	gitInTestRepo(tt, root, "config", "user.email", "test@test.com")
	gitInTestRepo(tt, root, "config", "user.name", "Test")
	gitInTestRepo(tt, root, "config", "commit.gpgSign", "false")

	writeFixture(tt, root, "Cargo.toml", `[workspace]
members = [
  "internal/0/blob_store_id",
  "internal/0/store",
]
resolver = "2"
`)

	writeFixture(tt, root, "internal/0/blob_store_id/Cargo.toml", `[package]
name = "blob_store_id_internal"
version = "0.1.0"
edition = "2021"
`)

	writeFixture(
		tt,
		root,
		"internal/0/blob_store_id/src/lib.rs",
		"pub fn make_id() -> u32 { 7 }\n",
	)

	writeFixture(tt, root, "internal/0/store/Cargo.toml", `[package]
name = "store_internal"
version = "0.1.0"
edition = "2021"

[dependencies]
blob_store_id_internal = { path = "../blob_store_id" }
`)

	writeFixture(
		tt,
		root,
		"internal/0/store/src/lib.rs",
		"pub fn make() -> u32 { blob_store_id_internal::make_id() }\n",
	)

	gitInTestRepo(tt, root, "add", "-A")
	gitInTestRepo(tt, root, "commit", "-m", "fixture")

	r := &Renamer{WorkspaceRoot: root}
	mapper := sliceLevelMapper{levels: []string{"0", "alfa"}}

	if err := r.Rename("internal/0/store", "", mapper, Options{}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(root, "internal/alfa/store/Cargo.toml")); err != nil {
		t.Fatal("crate not repositioned to internal/alfa/store:", err)
	}

	if _, err := os.Stat(filepath.Join(root, "internal/0/store")); !os.IsNotExist(err) {
		t.Errorf("old crate dir still present (err: %v)", err)
	}

	requireCargoCheck(tt, root)
}

func TestDryRunPrintsPlanAndChangesNothing(t *testing.T) {
	tt := test_ui.T{T: t}
	requireCargo(tt)
	requireAstGrep(tt)
	root := writeFixtureWorkspace(tt)

	before := allContentHashes(tt, root)

	r := &Renamer{WorkspaceRoot: root}

	var renameErr error

	out := captureStdout(tt, func() {
		renameErr = r.MoveRename(
			"internal/0/blob_store_id",
			"internal/0/blob_id",
			Options{DryRun: true},
		)
	})
	if renameErr != nil {
		t.Fatal(renameErr)
	}

	plan := string(out)

	for _, want := range []string{"internal/0/blob_store_id", "internal/0/blob_id"} {
		if !strings.Contains(plan, want) {
			t.Errorf("dry-run plan does not mention %q:\n%s", want, plan)
		}
	}

	if !regexp.MustCompile(`[1-9][0-9]* match`).MatchString(plan) {
		t.Errorf("dry-run plan reports no ast-grep match counts:\n%s", plan)
	}

	after := allContentHashes(tt, root)

	if len(before) != len(after) {
		t.Fatalf("file count changed in dry run: %d before, %d after", len(before), len(after))
	}

	for rel, hash := range before {
		if after[rel] != hash {
			t.Errorf("file %s changed in dry run", rel)
		}
	}
}
