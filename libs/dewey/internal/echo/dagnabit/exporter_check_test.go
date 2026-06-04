package dagnabit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeCheckFixture creates a temp module with the given internal packages
// (relPath under internal/ → main.go source) and returns an Exporter rooted
// there. Mirrors the fixtures in exporter_test.go.
//
// It plants its own treefmt.toml at the module root and installs a no-op fake
// `treefmt` on PATH so the FormatOutput pass inside CheckAll/CheckPackage runs
// deterministically (and as a no-op on the facade bytes) regardless of where
// $TMPDIR lives. This helper previously skipped whenever an ancestor carried a
// treefmt/conformist config — always the case when $TMPDIR sits inside a repo
// that has one (e.g. this worktree's .tmp/ under the root conformist.toml),
// which left these tests unexecuted in-repo (#127). The fixture's own config is
// the nearest ancestor, so findTreefmtConfig resolves to it (formatter
// "treefmt") rather than any real config further up.
func writeCheckFixture(t *testing.T, pkgs map[string]string) *Exporter {
	t.Helper()

	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "treefmt.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	withFakeTreefmt(t, filepath.Join(t.TempDir(), "sentinel"))

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"),
		[]byte("module example.com/mod\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for rel, src := range pkgs {
		dir := filepath.Join(tmpDir, "internal", filepath.FromSlash(rel))
		mustMkdirAll(t, dir)
		if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return &Exporter{
		ModulePath:          "example.com/mod",
		Dir:                 tmpDir,
		OutputDir:           "pkgs",
		SkipConsumerRewrite: true,
		Env:                 append(os.Environ(), "GOWORK=off"),
	}
}

func TestCheckAllPassesWhenInSync(t *testing.T) {
	e := writeCheckFixture(t, map[string]string{
		"alfa/widget": "package widget\n\ntype Widget struct{}\n",
	})
	if err := e.ExportAll(); err != nil {
		t.Fatalf("ExportAll: %v", err)
	}
	if err := e.CheckAll(); err != nil {
		t.Fatalf("CheckAll should pass right after a clean export, got: %v", err)
	}
}

func TestCheckAllDetectsDrift(t *testing.T) {
	e := writeCheckFixture(t, map[string]string{
		"alfa/widget": "package widget\n\ntype Widget struct{}\n",
	})
	if err := e.ExportAll(); err != nil {
		t.Fatalf("ExportAll: %v", err)
	}

	facade := filepath.Join(e.Dir, "pkgs", "widget", "main.go")
	before, err := os.ReadFile(facade)
	if err != nil {
		t.Fatal(err)
	}
	mutated := string(before) + "\n// drift\n"
	if err := os.WriteFile(facade, []byte(mutated), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := e.CheckAll(); err == nil {
		t.Fatal("CheckAll should report drift after the facade was mutated")
	} else if !strings.Contains(err.Error(), "widget") {
		t.Errorf("drift error should name the package, got: %v", err)
	}

	// Check is side-effect-free: the on-disk facade must be untouched.
	after, err := os.ReadFile(facade)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != mutated {
		t.Error("CheckAll must not modify on-disk facades")
	}
}

func TestCheckAllDetectsMissingFacade(t *testing.T) {
	e := writeCheckFixture(t, map[string]string{
		"alfa/widget": "package widget\n\ntype Widget struct{}\n",
	})
	if err := e.ExportAll(); err != nil {
		t.Fatalf("ExportAll: %v", err)
	}
	if err := os.Remove(filepath.Join(e.Dir, "pkgs", "widget", "main.go")); err != nil {
		t.Fatal(err)
	}
	if err := e.CheckAll(); err == nil {
		t.Fatal("CheckAll should report a missing committed facade")
	} else if !strings.Contains(err.Error(), "widget") {
		t.Errorf("error should name the missing package, got: %v", err)
	}
}

// TestCheckAllIgnoresHandWrittenFacadeTests confirms a hand-written *_test.go
// living alongside generated facades (e.g. pkgs/<x>/<x>_test.go) is not flagged
// as stale drift — the exporter only produces main.go + build-tag files.
func TestCheckAllIgnoresHandWrittenFacadeTests(t *testing.T) {
	e := writeCheckFixture(t, map[string]string{
		"alfa/widget": "package widget\n\ntype Widget struct{}\n",
	})
	if err := e.ExportAll(); err != nil {
		t.Fatalf("ExportAll: %v", err)
	}

	testFile := filepath.Join(e.Dir, "pkgs", "widget", "widget_test.go")
	if err := os.WriteFile(testFile,
		[]byte("package widget_test\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := e.CheckAll(); err != nil {
		t.Fatalf("CheckAll should ignore hand-written *_test.go files, got: %v", err)
	}
}

// TestCheckPackageScopedToOnePackage confirms single-package check only
// validates the named package and does not flag unrelated on-disk facades
// (reportStale=false for partial regeneration).
func TestCheckPackageScopedToOnePackage(t *testing.T) {
	e := writeCheckFixture(t, map[string]string{
		"alfa/widget": "package widget\n\ntype Widget struct{}\n",
		"alfa/gadget": "package gadget\n\ntype Gadget struct{}\n",
	})
	if err := e.ExportAll(); err != nil {
		t.Fatalf("ExportAll: %v", err)
	}

	gadget := filepath.Join(e.Dir, "pkgs", "gadget", "main.go")
	g, err := os.ReadFile(gadget)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gadget, []byte(string(g)+"\n// drift\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Checking widget must ignore the unrelated gadget drift.
	if err := e.CheckPackage("./internal/alfa/widget"); err != nil {
		t.Fatalf("CheckPackage(widget) should ignore unrelated gadget drift, got: %v", err)
	}

	// Checking the drifted package itself must fail.
	if err := e.CheckPackage("./internal/alfa/gadget"); err == nil {
		t.Fatal("CheckPackage(gadget) should report gadget's drift")
	}
}

// TestFormatOutputFailsLoudWhenFormatterMissing locks in the fail-loud
// behavior: a config-present tree with no formatter on PATH must error rather
// than silently skip formatting (which would emit unformatted facades).
func TestFormatOutputFailsLoudWhenFormatterMissing(t *testing.T) {
	tmpDir := t.TempDir()
	mustMkdirAll(t, filepath.Join(tmpDir, "pkgs"))
	if err := os.WriteFile(filepath.Join(tmpDir, "conformist.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	// Scrub PATH so neither `conformist` nor `nix` is resolvable.
	t.Setenv("PATH", t.TempDir())

	exporter := &Exporter{Dir: tmpDir, OutputDir: "pkgs"}
	err := exporter.FormatOutput()
	if err == nil {
		t.Fatal("FormatOutput should fail loud when the configured formatter is missing")
	}
	if !strings.Contains(err.Error(), "conformist") {
		t.Errorf("error should name the missing formatter, got: %v", err)
	}
}

// TestCheckAllReproducesFormatterAcrossTempDir guards against #125: the
// comparison copy `export --check` renders must be formatted identically to a
// real export before being diffed. The project formatter anchors its tree root
// at the config/module root and formats nothing outside it, so when the
// comparison copy lived in the system temp dir (typically outside the repo) it
// was compared unformatted against the committed (formatted) facades and
// reported phantom drift. The fix renders the copy in-tree (under exporter.Dir).
//
// Unlike the other check tests, this one provides its own treefmt.toml so it
// does NOT skip on an ancestor config — it must actually run FormatOutput. The
// fake treefmt models real treefmt's tree-root behavior (see
// withTreeRootAwareFakeTreefmt): only files under its working directory are
// formatted.
func TestCheckAllReproducesFormatterAcrossTempDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Force the treefmt branch with our own config. findTreefmtConfig finds this
	// before any ancestor conformist/treefmt config.
	if err := os.WriteFile(filepath.Join(tmpDir, "treefmt.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"),
		[]byte("module example.com/mod\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgDir := filepath.Join(tmpDir, "internal", "alfa", "widget")
	mustMkdirAll(t, pkgDir)
	if err := os.WriteFile(filepath.Join(pkgDir, "main.go"),
		[]byte("package widget\n\ntype Widget struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	withTreeRootAwareFakeTreefmt(t)

	e := &Exporter{
		ModulePath:          "example.com/mod",
		Dir:                 tmpDir,
		OutputDir:           "pkgs",
		SkipConsumerRewrite: true,
		Env:                 append(os.Environ(), "GOWORK=off"),
	}

	// Real export: generate, then format the committed facades in-tree.
	if err := e.ExportAll(); err != nil {
		t.Fatalf("ExportAll: %v", err)
	}
	if err := e.FormatOutput(); err != nil {
		t.Fatalf("FormatOutput: %v", err)
	}

	// A faithful --check reproduces that same formatting on its comparison copy
	// and finds no drift. Before the fix the copy lived out-of-tree and was left
	// unformatted, so this reported phantom drift.
	if err := e.CheckAll(); err != nil {
		t.Fatalf("CheckAll false-positived on a clean formatted export: %v", err)
	}
}
