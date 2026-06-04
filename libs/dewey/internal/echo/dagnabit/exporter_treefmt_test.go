package dagnabit

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFindTreefmtConfig_TomlAtRoot(t *testing.T) {
	tmpDir := t.TempDir()
	tomlPath := filepath.Join(tmpDir, "treefmt.toml")
	if err := os.WriteFile(tomlPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	dir, name, ok := findTreefmtConfig(tmpDir)
	if !ok {
		t.Fatal("expected config to be found")
	}
	if name != "treefmt.toml" {
		t.Errorf("expected name=treefmt.toml, got %q", name)
	}
	if dir != absForTest(t, tmpDir) {
		t.Errorf("expected dir=%s, got %s", tmpDir, dir)
	}
}

func TestFindTreefmtConfig_NixAtRoot(t *testing.T) {
	tmpDir := t.TempDir()
	nixPath := filepath.Join(tmpDir, "treefmt.nix")
	if err := os.WriteFile(nixPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	_, name, ok := findTreefmtConfig(tmpDir)
	if !ok {
		t.Fatal("expected config to be found")
	}
	if name != "treefmt.nix" {
		t.Errorf("expected name=treefmt.nix, got %q", name)
	}
}

func TestFindTreefmtConfig_WalksUp(t *testing.T) {
	tmpDir := t.TempDir()
	rootConfig := filepath.Join(tmpDir, "treefmt.toml")
	if err := os.WriteFile(rootConfig, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	deep := filepath.Join(tmpDir, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	dir, _, ok := findTreefmtConfig(deep)
	if !ok {
		t.Fatal("expected config to be found by walking up")
	}
	if dir != absForTest(t, tmpDir) {
		t.Errorf("expected dir=%s, got %s", tmpDir, dir)
	}
}

func TestFindTreefmtConfig_PrefersFirstInList(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "treefmt.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "treefmt.nix"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	_, name, ok := findTreefmtConfig(tmpDir)
	if !ok {
		t.Fatal("expected config to be found")
	}
	if name != "treefmt.toml" {
		t.Errorf("expected treefmt.toml to win over treefmt.nix, got %q", name)
	}
}

func TestFindTreefmtConfig_NotFound(t *testing.T) {
	// Use a path under tmpDir to guarantee nothing above happens to have
	// a treefmt config (avoid a flake where the test machine has /treefmt.toml).
	tmpDir := t.TempDir()
	deep := filepath.Join(tmpDir, "deep")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	// findTreefmtConfig walks all the way to /, so we need to ensure
	// the ancestor chain has no treefmt configs. Since we just created
	// these directories, that's true for everything under tmpDir, but
	// the parents above tmpDir are outside our control. Skip if any
	// ancestor happens to have a config — vanishingly rare in CI.
	if dir, name, ok := findTreefmtConfig(deep); ok {
		t.Skipf("test environment has %s at %s (ancestor of %s)", name, dir, deep)
	}
}

func TestFormatOutput_NoConfigIsNoop(t *testing.T) {
	tmpDir := t.TempDir()
	mustMkdirAll(t, filepath.Join(tmpDir, "pkgs"))

	if _, _, ok := findTreefmtConfig(tmpDir); ok {
		t.Skip("test environment has a treefmt config in an ancestor directory")
	}

	exporter := &Exporter{Dir: tmpDir, OutputDir: "pkgs"}
	if err := exporter.FormatOutput(); err != nil {
		t.Fatalf("FormatOutput with no config should be no-op, got: %v", err)
	}
}

func TestFormatOutput_DryRunIsNoop(t *testing.T) {
	tmpDir := t.TempDir()
	mustMkdirAll(t, filepath.Join(tmpDir, "pkgs"))

	if err := os.WriteFile(filepath.Join(tmpDir, "treefmt.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	sentinel := filepath.Join(tmpDir, "sentinel")
	withFakeTreefmt(t, sentinel)

	exporter := &Exporter{Dir: tmpDir, OutputDir: "pkgs", DryRun: true}
	if err := exporter.FormatOutput(); err != nil {
		t.Fatalf("FormatOutput dry-run should be no-op, got: %v", err)
	}

	if _, err := os.Stat(sentinel); err == nil {
		t.Error("expected sentinel not to exist in dry-run mode; treefmt should not have been invoked")
	}
}

func TestFormatOutput_MissingOutputDirIsNoop(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "treefmt.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	sentinel := filepath.Join(tmpDir, "sentinel")
	withFakeTreefmt(t, sentinel)

	exporter := &Exporter{Dir: tmpDir, OutputDir: "pkgs"}
	if err := exporter.FormatOutput(); err != nil {
		t.Fatalf("FormatOutput with missing pkgs/ should be no-op, got: %v", err)
	}

	if _, err := os.Stat(sentinel); err == nil {
		t.Error("expected sentinel not to exist when output dir is missing")
	}
}

func TestFormatOutput_InvokesTreefmt(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "pkgs")
	mustMkdirAll(t, outDir)

	if err := os.WriteFile(filepath.Join(tmpDir, "treefmt.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	sentinel := filepath.Join(tmpDir, "sentinel")
	withFakeTreefmt(t, sentinel)

	exporter := &Exporter{Dir: tmpDir, OutputDir: "pkgs"}
	if err := exporter.FormatOutput(); err != nil {
		t.Fatalf("FormatOutput: %v", err)
	}

	body, err := os.ReadFile(sentinel)
	if err != nil {
		t.Fatalf("expected sentinel to be written by fake treefmt: %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(args) == 0 || !strings.HasSuffix(args[len(args)-1], filepath.Join("pkgs")) {
		t.Errorf("expected fake treefmt to be invoked with output dir as last arg, got args=%v", args)
	}
}

func TestFormatOutput_PropagatesTreefmtFailure(t *testing.T) {
	tmpDir := t.TempDir()
	mustMkdirAll(t, filepath.Join(tmpDir, "pkgs"))

	if err := os.WriteFile(filepath.Join(tmpDir, "treefmt.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	withFailingFakeTreefmt(t)

	exporter := &Exporter{Dir: tmpDir, OutputDir: "pkgs"}
	err := exporter.FormatOutput()
	if err == nil {
		t.Fatal("expected FormatOutput to surface treefmt failure")
	}
	if !strings.Contains(err.Error(), "treefmt") {
		t.Errorf("expected error to mention treefmt, got: %v", err)
	}
}

// absForTest returns filepath.Abs(path) or fails the test.
func absForTest(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

// withFakeTreefmt writes a stub shell script named `treefmt` into a
// fresh directory, prepends that directory to PATH, and registers a
// cleanup that restores PATH at the end of the test. The fake
// records its argv into sentinelPath.
func withFakeTreefmt(t *testing.T, sentinelPath string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("PATH-injection fake binary not portable to Windows")
	}

	binDir := t.TempDir()
	fake := filepath.Join(binDir, "treefmt")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$@\" > %q\n", sentinelPath)
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	prependPath(t, binDir)
}

// withTreeRootAwareFakeTreefmt installs a fake `treefmt` that models real
// treefmt's tree-root anchoring: it only "formats" .go files located within its
// working directory (FormatOutput sets that to the config/module root) and is a
// no-op for any path outside it. Formatting appends a sentinel line, so a file
// the fake skipped is detectably different from one it processed. Used to
// reproduce #125, where the comparison copy lived outside the tree root and was
// silently left unformatted.
func withTreeRootAwareFakeTreefmt(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("PATH-injection fake binary not portable to Windows")
	}

	binDir := t.TempDir()
	fake := filepath.Join(binDir, "treefmt")
	script := `#!/bin/sh
root=$PWD
for arg in "$@"; do
  case "$arg" in
    --*) continue ;;
  esac
  case "$arg" in
    "$root"/*) ;;
    *) continue ;;
  esac
  find "$arg" -name '*.go' -type f | while IFS= read -r f; do
    grep -q '//treefmt-formatted' "$f" || printf '//treefmt-formatted\n' >>"$f"
  done
done
exit 0
`
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	prependPath(t, binDir)
}

// withFailingFakeTreefmt installs a fake treefmt that exits non-zero.
func withFailingFakeTreefmt(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("PATH-injection fake binary not portable to Windows")
	}

	binDir := t.TempDir()
	fake := filepath.Join(binDir, "treefmt")
	script := "#!/bin/sh\nexit 7\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	prependPath(t, binDir)
}

// prependPath puts dir at the front of PATH for the duration of the
// test, restoring the original PATH on cleanup.
func prependPath(t *testing.T, dir string) {
	t.Helper()
	orig := os.Getenv("PATH")
	t.Cleanup(func() {
		os.Setenv("PATH", orig)
	})
	os.Setenv("PATH", dir+string(os.PathListSeparator)+orig)
}

// withFakeConformist writes a stub shell script named `conformist` into a fresh
// directory and prepends it to PATH, mirroring withFakeTreefmt. The fake
// records its argv into sentinelPath.
func withFakeConformist(t *testing.T, sentinelPath string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("PATH-injection fake binary not portable to Windows")
	}

	binDir := t.TempDir()
	fake := filepath.Join(binDir, "conformist")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$@\" > %q\n", sentinelPath)
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	prependPath(t, binDir)
}

// TestFindTreefmtConfig_PrefersConformist confirms a conformist.toml is chosen
// over a treefmt.toml in the same directory (conformist is the successor).
func TestFindTreefmtConfig_PrefersConformist(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "conformist.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "treefmt.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	_, name, ok := findTreefmtConfig(tmpDir)
	if !ok {
		t.Fatal("expected to find a config")
	}
	if name != "conformist.toml" {
		t.Errorf("expected conformist.toml to win over treefmt.toml, got %q", name)
	}
}

// TestFormatOutput_InvokesConformist confirms a conformist.toml config drives the
// `conformist` binary rather than `treefmt`.
func TestFormatOutput_InvokesConformist(t *testing.T) {
	tmpDir := t.TempDir()
	mustMkdirAll(t, filepath.Join(tmpDir, "pkgs"))

	if err := os.WriteFile(filepath.Join(tmpDir, "conformist.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	sentinel := filepath.Join(tmpDir, "sentinel")
	withFakeConformist(t, sentinel)

	exporter := &Exporter{Dir: tmpDir, OutputDir: "pkgs"}
	if err := exporter.FormatOutput(); err != nil {
		t.Fatalf("FormatOutput: %v", err)
	}

	body, err := os.ReadFile(sentinel)
	if err != nil {
		t.Fatalf("expected sentinel to be written by fake conformist: %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(args) == 0 || !strings.HasSuffix(args[len(args)-1], "pkgs") {
		t.Errorf("expected fake conformist to be invoked with output dir as last arg, got args=%v", args)
	}
}
