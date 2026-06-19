package dagnabit_rust

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amarbel-llc/purse-first/libs/dewey/internal/alfa/test_ui"
)

// readFixtureFile reads a file under root, failing the test on error.
func readFixtureFile(t test_ui.T, root, relPath string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relPath)))
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}

func TestExportGeneratesFacadeCrate(t *testing.T) {
	tt := test_ui.T{T: t}
	root := writeFixtureWorkspaceFiles(tt)

	e := &Exporter{WorkspaceRoot: root}

	if err := e.ExportPackage("internal/0/blob_store_id"); err != nil {
		t.Fatal(err)
	}

	libRS := readFixtureFile(tt, root, "pkgs/blob_store_id/src/lib.rs")

	wantLibRS := generatedHeader() + "\n\n" +
		"pub use blob_store_id_internal::*;\n"
	if libRS != wantLibRS {
		t.Errorf("lib.rs mismatch:\ngot:\n%s\nwant:\n%s", libRS, wantLibRS)
	}

	manifest := readFixtureFile(tt, root, "pkgs/blob_store_id/Cargo.toml")

	for _, want := range []string{
		`name = "blob_store_id"`,
		`version = "0.1.0"`,
		`edition = "2021"`,
		`blob_store_id_internal = { path = "../../internal/0/blob_store_id" }`,
	} {
		if !strings.Contains(manifest, want) {
			t.Errorf("facade Cargo.toml missing %q:\n%s", want, manifest)
		}
	}

	ws := readFixtureFile(tt, root, "Cargo.toml")
	if !strings.Contains(ws, `"pkgs/blob_store_id"`) {
		t.Errorf("workspace members did not gain pkgs/blob_store_id:\n%s", ws)
	}
}

func TestExportOmitsEditionWhenInternalCrateOmitsIt(t *testing.T) {
	tt := test_ui.T{T: t}
	root := writeFixtureWorkspaceFiles(tt)

	writeFixture(tt, root, "internal/0/oldstyle/Cargo.toml", `[package]
name = "oldstyle_internal"
version = "0.1.0"
`)
	writeFixture(tt, root, "internal/0/oldstyle/src/lib.rs", "pub fn f() {}\n")

	e := &Exporter{WorkspaceRoot: root}

	if err := e.ExportPackage("internal/0/oldstyle"); err != nil {
		t.Fatal(err)
	}

	manifest := readFixtureFile(tt, root, "pkgs/oldstyle/Cargo.toml")

	// A facade for an edition-less internal crate must contain no
	// edition line at all (covers the invalid `edition = ""` cargo
	// rejects); both crates then inherit cargo's 2015 default.
	if strings.Contains(manifest, "edition") {
		t.Errorf(
			"facade Cargo.toml must omit edition when the internal crate omits it:\n%s",
			manifest,
		)
	}
}

func TestExportRejectsNamingCollision(t *testing.T) {
	tt := test_ui.T{T: t}
	root := writeFixtureWorkspaceFiles(tt)

	writeFixture(tt, root, "internal/0/badcrate/Cargo.toml", `[package]
name = "badcrate"
version = "0.1.0"
edition = "2021"
`)
	writeFixture(tt, root, "internal/0/badcrate/src/lib.rs", "pub fn f() {}\n")

	e := &Exporter{WorkspaceRoot: root}

	err := e.ExportPackage("internal/0/badcrate")
	if err == nil {
		t.Fatal("expected naming-collision error, got nil")
	}

	for _, want := range []string{"badcrate_internal", "convention"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestExportAllExportsEverythingUnderInternal(t *testing.T) {
	tt := test_ui.T{T: t}
	root := writeFixtureWorkspaceFiles(tt)

	e := &Exporter{WorkspaceRoot: root}

	if err := e.ExportAll(); err != nil {
		t.Fatal(err)
	}

	for _, facade := range []string{"pkgs/blob_store_id", "pkgs/store"} {
		if _, err := os.Stat(
			filepath.Join(root, filepath.FromSlash(facade), "src", "lib.rs"),
		); err != nil {
			t.Errorf("facade %s not generated: %v", facade, err)
		}
	}

	storeLibRS := readFixtureFile(tt, root, "pkgs/store/src/lib.rs")
	if !strings.Contains(storeLibRS, "pub use store_internal::*;") {
		t.Errorf("store facade lib.rs wrong:\n%s", storeLibRS)
	}

	ws := readFixtureFile(tt, root, "Cargo.toml")

	for _, member := range []string{`"pkgs/blob_store_id"`, `"pkgs/store"`} {
		if !strings.Contains(ws, member) {
			t.Errorf("workspace members missing %s:\n%s", member, ws)
		}
	}
}

func TestExportScanHonorsMetadataDirective(t *testing.T) {
	tt := test_ui.T{T: t}
	root := writeFixtureWorkspaceFiles(tt)

	writeFixture(tt, root, "internal/0/blob_store_id/Cargo.toml", `[package]
name = "blob_store_id_internal"
version = "0.1.0"
edition = "2021"

[package.metadata.dagnabit]
export = true
`)

	e := &Exporter{WorkspaceRoot: root}

	if err := e.ScanAndExport(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(
		filepath.Join(root, "pkgs", "blob_store_id", "src", "lib.rs"),
	); err != nil {
		t.Errorf("directive-marked crate not exported: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "pkgs", "store")); !os.IsNotExist(err) {
		t.Errorf("unmarked crate must not be exported (err: %v)", err)
	}

	if err := e.CheckScan(); err != nil {
		t.Errorf("CheckScan right after ScanAndExport should pass: %v", err)
	}
}

func TestExportCheckDetectsDrift(t *testing.T) {
	tt := test_ui.T{T: t}
	root := writeFixtureWorkspaceFiles(tt)

	e := &Exporter{WorkspaceRoot: root}

	if err := e.ExportAll(); err != nil {
		t.Fatal(err)
	}

	if err := e.CheckAll(); err != nil {
		t.Fatalf("CheckAll right after ExportAll should pass: %v", err)
	}

	corrupted := filepath.Join(root, "pkgs", "blob_store_id", "src", "lib.rs")
	if err := os.WriteFile(corrupted, []byte("// corrupted\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := e.CheckAll()
	if err == nil {
		t.Fatal("expected drift error, got nil")
	}

	if !strings.Contains(err.Error(), "pkgs/blob_store_id/src/lib.rs") {
		t.Errorf("drift error does not name the drifted file: %v", err)
	}
}

func TestExportCheckDetectsMissingFacade(t *testing.T) {
	tt := test_ui.T{T: t}
	root := writeFixtureWorkspaceFiles(tt)

	e := &Exporter{WorkspaceRoot: root}

	err := e.CheckAll()
	if err == nil {
		t.Fatal("expected missing-facade error, got nil")
	}

	if !strings.Contains(err.Error(), "blob_store_id") {
		t.Errorf("missing-facade error does not name the facade: %v", err)
	}

	// Check must not have written the real tree nor touched the manifest.
	if _, statErr := os.Stat(filepath.Join(root, "pkgs")); !os.IsNotExist(statErr) {
		t.Errorf("CheckAll must not create pkgs/ (err: %v)", statErr)
	}

	ws := readFixtureFile(tt, root, "Cargo.toml")
	if strings.Contains(ws, "pkgs/") {
		t.Errorf("CheckAll must not mutate workspace members:\n%s", ws)
	}
}

func TestExportDryRunWritesNothing(t *testing.T) {
	tt := test_ui.T{T: t}
	root := writeFixtureWorkspaceFiles(tt)

	wsBefore := readFixtureFile(tt, root, "Cargo.toml")

	e := &Exporter{WorkspaceRoot: root, DryRun: true}

	if err := e.ExportPackage("internal/0/blob_store_id"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(root, "pkgs")); !os.IsNotExist(err) {
		t.Errorf("dry-run must not create pkgs/ (err: %v)", err)
	}

	if wsAfter := readFixtureFile(tt, root, "Cargo.toml"); wsAfter != wsBefore {
		t.Errorf("dry-run must not mutate root manifest:\n%s", wsAfter)
	}
}

func TestGeneratedFacadeCompiles(t *testing.T) {
	tt := test_ui.T{T: t}
	requireCargo(tt)
	root := writeFixtureWorkspace(tt)

	e := &Exporter{WorkspaceRoot: root}

	if err := e.ExportAll(); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("cargo", "check", "--workspace")
	cmd.Dir = root

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cargo check --workspace failed: %v\n%s", err, out)
	}
}
