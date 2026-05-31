package dagnabit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeCheckFixture creates a temp module with the given internal packages
// (relPath under internal/ → main.go source) and returns an Exporter rooted
// there. Mirrors the fixtures in exporter_test.go. It skips the test if an
// ancestor of the temp dir happens to carry a treefmt/treelint config, so the
// FormatOutput pass inside CheckAll/CheckPackage is a deterministic no-op and
// the tests don't depend on a formatter being on PATH.
func writeCheckFixture(t *testing.T, pkgs map[string]string) *Exporter {
	t.Helper()

	tmpDir := t.TempDir()
	if _, _, ok := findTreefmtConfig(tmpDir); ok {
		t.Skip("ancestor treefmt/treelint config present; check tests need a config-free tree")
	}

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
	if err := os.WriteFile(filepath.Join(tmpDir, "treelint.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	// Scrub PATH so neither `treelint` nor `nix` is resolvable.
	t.Setenv("PATH", t.TempDir())

	exporter := &Exporter{Dir: tmpDir, OutputDir: "pkgs"}
	err := exporter.FormatOutput()
	if err == nil {
		t.Fatal("FormatOutput should fail loud when the configured formatter is missing")
	}
	if !strings.Contains(err.Error(), "treelint") {
		t.Errorf("error should name the missing formatter, got: %v", err)
	}
}
