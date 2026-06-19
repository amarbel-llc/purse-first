package dagnabit

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/amarbel-llc/purse-first/libs/dewey/internal/alfa/test_ui"
)

func TestFindTreefmtConfig_TomlAtRoot(t *testing.T) {
	tt := test_ui.T{T: t}
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
	if dir != absForTest(tt, tmpDir) {
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
	tt := test_ui.T{T: t}
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
	if dir != absForTest(tt, tmpDir) {
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
	tt := test_ui.T{T: t}
	tmpDir := t.TempDir()
	mustMkdirAll(tt, filepath.Join(tmpDir, "pkgs"))

	if _, _, ok := findTreefmtConfig(tmpDir); ok {
		t.Skip("test environment has a treefmt config in an ancestor directory")
	}

	exporter := &Exporter{Dir: tmpDir, OutputDir: "pkgs"}
	if err := exporter.FormatOutput(); err != nil {
		t.Fatalf("FormatOutput with no config should be no-op, got: %v", err)
	}
}

func TestFormatOutput_DryRunIsNoop(t *testing.T) {
	tt := test_ui.T{T: t}
	tmpDir := t.TempDir()
	mustMkdirAll(tt, filepath.Join(tmpDir, "pkgs"))

	if err := os.WriteFile(filepath.Join(tmpDir, "treefmt.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	sentinel := filepath.Join(tmpDir, "sentinel")
	withFakeTreefmt(tt, sentinel)

	exporter := &Exporter{Dir: tmpDir, OutputDir: "pkgs", DryRun: true}
	if err := exporter.FormatOutput(); err != nil {
		t.Fatalf("FormatOutput dry-run should be no-op, got: %v", err)
	}

	if _, err := os.Stat(sentinel); err == nil {
		t.Error("expected sentinel not to exist in dry-run mode; treefmt should not have been invoked")
	}
}

func TestFormatOutput_MissingOutputDirIsNoop(t *testing.T) {
	tt := test_ui.T{T: t}
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "treefmt.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	sentinel := filepath.Join(tmpDir, "sentinel")
	withFakeTreefmt(tt, sentinel)

	exporter := &Exporter{Dir: tmpDir, OutputDir: "pkgs"}
	if err := exporter.FormatOutput(); err != nil {
		t.Fatalf("FormatOutput with missing pkgs/ should be no-op, got: %v", err)
	}

	if _, err := os.Stat(sentinel); err == nil {
		t.Error("expected sentinel not to exist when output dir is missing")
	}
}

func TestFormatOutput_InvokesTreefmt(t *testing.T) {
	tt := test_ui.T{T: t}
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "pkgs")
	mustMkdirAll(tt, outDir)

	if err := os.WriteFile(filepath.Join(tmpDir, "treefmt.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	sentinel := filepath.Join(tmpDir, "sentinel")
	withFakeTreefmt(tt, sentinel)

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
	tt := test_ui.T{T: t}
	tmpDir := t.TempDir()
	mustMkdirAll(tt, filepath.Join(tmpDir, "pkgs"))

	if err := os.WriteFile(filepath.Join(tmpDir, "treefmt.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	withFailingFakeTreefmt(tt)

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
func absForTest(t test_ui.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func mustMkdirAll(t test_ui.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

// withFakeTreefmt writes a stub shell script named `treefmt` into a
// fresh directory, prepends that directory to PATH, and registers a
// cleanup that restores PATH at the end of the test. The fake
// records its argv into sentinelPath.
func withFakeTreefmt(t test_ui.T, sentinelPath string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.T.Skip("PATH-injection fake binary not portable to Windows")
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
func withTreeRootAwareFakeTreefmt(t test_ui.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.T.Skip("PATH-injection fake binary not portable to Windows")
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
func withFailingFakeTreefmt(t test_ui.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.T.Skip("PATH-injection fake binary not portable to Windows")
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
func prependPath(t test_ui.T, dir string) {
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
func withFakeConformist(t test_ui.T, sentinelPath string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.T.Skip("PATH-injection fake binary not portable to Windows")
	}

	binDir := t.TempDir()
	fake := filepath.Join(binDir, "conformist")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$@\" > %q\n", sentinelPath)
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	prependPath(t, binDir)
}

// withFakeWrapperConformist installs a fake `conformist` that models the
// Nix-generated wrapper: its script body bakes a --tree-root-file flag (as
// conformist's build.wrapper does), so conformistBakesTreeRoot detects it and
// FormatOutput omits dagnabit's own --tree-root (purse-first#162). Like the
// plain fake it records the argv it is actually invoked with into sentinelPath.
func withFakeWrapperConformist(t test_ui.T, sentinelPath string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.T.Skip("PATH-injection fake binary not portable to Windows")
	}

	binDir := t.TempDir()
	fake := filepath.Join(binDir, "conformist")
	script := fmt.Sprintf(
		"#!/bin/sh\n# --tree-root-file=/baked/flake.nix (wrapper-baked tree root)\n"+
			"printf '%%s\\n' \"$@\" > %q\n",
		sentinelPath,
	)
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	prependPath(t, binDir)
}

// readSentinelArgs reads the newline-separated argv a fake formatter recorded.
func readSentinelArgs(t test_ui.T, sentinelPath string) []string {
	t.Helper()
	body, err := os.ReadFile(sentinelPath)
	if err != nil {
		t.Fatalf("expected sentinel to be written by fake conformist: %v", err)
	}
	return strings.Split(strings.TrimSpace(string(body)), "\n")
}

// TestFindTreefmtConfig_CeilingStopsEscalation is the purse-first#159
// regression: a config only in an ANCESTOR is NOT found when
// DAGNABIT_CEILING_DIRECTORIES bounds the walk below that ancestor. Models the
// real failure — a repo with a Nix-generated conformist config (none on disk)
// must not escalate to a stray ancestor conformist.toml.
func TestFindTreefmtConfig_CeilingStopsEscalation(t *testing.T) {
	tt := test_ui.T{T: t}
	root := t.TempDir()

	// Config lives only at the ancestor root.
	if err := os.WriteFile(filepath.Join(root, "conformist.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	// The "repo" is root/repo; the walk starts in a subdir of it.
	repo := filepath.Join(root, "repo")
	start := filepath.Join(repo, "libs", "dewey")
	mustMkdirAll(tt, start)

	// Ceiling at the repo so the walk checks repo and below but never ascends
	// to root (where the stray config is).
	t.Setenv("DAGNABIT_CEILING_DIRECTORIES", absForTest(tt, repo))

	if dir, name, ok := findTreefmtConfig(start); ok {
		t.Errorf("expected ceiling to stop escalation to ancestor config, but found %s at %s", name, dir)
	}
}

// TestFindTreefmtConfig_CeilingAllowsInTreeConfig confirms the ceiling does not
// block finding a config at or below the start: a config at the repo root is
// still found with the ceiling set at the repo's parent.
func TestFindTreefmtConfig_CeilingAllowsInTreeConfig(t *testing.T) {
	tt := test_ui.T{T: t}
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	start := filepath.Join(repo, "libs", "dewey")
	mustMkdirAll(tt, start)

	// In-tree config at the repo root.
	if err := os.WriteFile(filepath.Join(repo, "conformist.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	// Ceiling at root (repo's parent): the walk may still reach repo itself.
	t.Setenv("DAGNABIT_CEILING_DIRECTORIES", absForTest(tt, root))

	dir, name, ok := findTreefmtConfig(start)
	if !ok {
		t.Fatal("expected in-tree config to be found with ceiling at repo parent")
	}
	if name != "conformist.toml" {
		t.Errorf("expected conformist.toml, got %q", name)
	}
	if dir != absForTest(tt, repo) {
		t.Errorf("expected dir=%s, got %s", repo, dir)
	}
}

// TestFormatOutput_ExplicitConfigPassesConfigFile confirms that
// DAGNABIT_CONFORMIST_CONFIG short-circuits discovery and invokes conformist
// with --config-file pointing at the explicit (e.g. Nix-generated) config —
// the purse-first#159 escape hatch for a repo with no conformist.toml on disk.
func TestFormatOutput_ExplicitConfigPassesConfigFile(t *testing.T) {
	tt := test_ui.T{T: t}
	tmpDir := t.TempDir()
	mustMkdirAll(tt, filepath.Join(tmpDir, "pkgs"))

	// No conformist.toml anywhere in-tree; a ceiling guarantees discovery would
	// otherwise find nothing.
	t.Setenv("DAGNABIT_CEILING_DIRECTORIES", absForTest(tt, tmpDir))

	configFile := filepath.Join(tmpDir, "generated-conformist.toml")
	if err := os.WriteFile(configFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DAGNABIT_CONFORMIST_CONFIG", configFile)

	sentinel := filepath.Join(tmpDir, "sentinel")
	withFakeConformist(tt, sentinel)

	exporter := &Exporter{Dir: tmpDir, OutputDir: "pkgs"}
	if err := exporter.FormatOutput(); err != nil {
		t.Fatalf("FormatOutput with explicit config: %v", err)
	}

	body, err := os.ReadFile(sentinel)
	if err != nil {
		t.Fatalf("expected sentinel to be written by fake conformist: %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(body)), "\n")
	if !slices.Contains(args, "--config-file") {
		t.Errorf("expected conformist to be invoked with --config-file, got args=%v", args)
	}
	if !slices.Contains(args, configFile) {
		t.Errorf("expected conformist args to include the explicit config %q, got args=%v", configFile, args)
	}
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
	tt := test_ui.T{T: t}
	tmpDir := t.TempDir()
	mustMkdirAll(tt, filepath.Join(tmpDir, "pkgs"))

	if err := os.WriteFile(filepath.Join(tmpDir, "conformist.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	sentinel := filepath.Join(tmpDir, "sentinel")
	withFakeConformist(tt, sentinel)

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

// TestFormatOutput_PlainConformistGetsTreeRoot confirms the raw conformist
// binary (no baked tree root) is still invoked with dagnabit's own
// --tree-root, the pre-purse-first#162 behavior. Pairs with
// TestFormatOutput_WrapperConformistOmitsTreeRoot below.
func TestFormatOutput_PlainConformistGetsTreeRoot(t *testing.T) {
	tt := test_ui.T{T: t}
	tmpDir := t.TempDir()
	mustMkdirAll(tt, filepath.Join(tmpDir, "pkgs"))

	if err := os.WriteFile(filepath.Join(tmpDir, "conformist.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	sentinel := filepath.Join(tmpDir, "sentinel")
	withFakeConformist(tt, sentinel)

	exporter := &Exporter{Dir: tmpDir, OutputDir: "pkgs"}
	if err := exporter.FormatOutput(); err != nil {
		t.Fatalf("FormatOutput: %v", err)
	}

	args := readSentinelArgs(tt, sentinel)
	if !slices.Contains(args, "--tree-root") {
		t.Errorf("expected plain conformist to receive --tree-root, got args=%v", args)
	}
}

// TestFormatOutput_WrapperConformistOmitsTreeRoot is the purse-first#162
// regression: when the on-PATH conformist is the Nix-generated wrapper (which
// bakes --tree-root-file), dagnabit must NOT append --tree-root, else conformist
// rejects the mutually-exclusive tree-root flags.
func TestFormatOutput_WrapperConformistOmitsTreeRoot(t *testing.T) {
	tt := test_ui.T{T: t}
	tmpDir := t.TempDir()
	mustMkdirAll(tt, filepath.Join(tmpDir, "pkgs"))

	if err := os.WriteFile(filepath.Join(tmpDir, "conformist.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	sentinel := filepath.Join(tmpDir, "sentinel")
	withFakeWrapperConformist(tt, sentinel)

	exporter := &Exporter{Dir: tmpDir, OutputDir: "pkgs"}
	if err := exporter.FormatOutput(); err != nil {
		t.Fatalf("FormatOutput: %v", err)
	}

	args := readSentinelArgs(tt, sentinel)
	if slices.Contains(args, "--tree-root") {
		t.Errorf("expected wrapper conformist NOT to receive --tree-root (collides with baked --tree-root-file), got args=%v", args)
	}
	if !slices.Contains(args, "--walk") {
		t.Errorf("expected wrapper conformist to still receive --walk filesystem, got args=%v", args)
	}
	if len(args) == 0 || !strings.HasSuffix(args[len(args)-1], "pkgs") {
		t.Errorf("expected wrapper conformist to still get the output dir as last arg, got args=%v", args)
	}
}
