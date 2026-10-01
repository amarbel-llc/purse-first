package dagnabit

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"code.linenisgreat.com/purse-first/libs/dewey/internal/alfa/test_ui"
)

func TestFindConformistConfig_TomlAtRoot(t *testing.T) {
	tt := test_ui.T{T: t}
	tmpDir := t.TempDir()
	tomlPath := filepath.Join(tmpDir, "conformist.toml")
	if err := os.WriteFile(tomlPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	dir, name, ok := findConformistConfig(tmpDir)
	if !ok {
		t.Fatal("expected config to be found")
	}
	if name != "conformist.toml" {
		t.Errorf("expected name=conformist.toml, got %q", name)
	}
	if dir != absForTest(tt, tmpDir) {
		t.Errorf("expected dir=%s, got %s", tmpDir, dir)
	}
}

func TestFindConformistConfig_HiddenTomlAtRoot(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, ".conformist.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	_, name, ok := findConformistConfig(tmpDir)
	if !ok {
		t.Fatal("expected config to be found")
	}
	if name != ".conformist.toml" {
		t.Errorf("expected name=.conformist.toml, got %q", name)
	}
}

func TestFindConformistConfig_WalksUp(t *testing.T) {
	tt := test_ui.T{T: t}
	tmpDir := t.TempDir()
	rootConfig := filepath.Join(tmpDir, "conformist.toml")
	if err := os.WriteFile(rootConfig, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	deep := filepath.Join(tmpDir, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	dir, _, ok := findConformistConfig(deep)
	if !ok {
		t.Fatal("expected config to be found by walking up")
	}
	if dir != absForTest(tt, tmpDir) {
		t.Errorf("expected dir=%s, got %s", tmpDir, dir)
	}
}

func TestFindConformistConfig_PrefersFirstInList(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "conformist.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".conformist.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	_, name, ok := findConformistConfig(tmpDir)
	if !ok {
		t.Fatal("expected config to be found")
	}
	if name != "conformist.toml" {
		t.Errorf("expected conformist.toml to win over .conformist.toml, got %q", name)
	}
}

func TestFindConformistConfig_NotFound(t *testing.T) {
	// Use a path under tmpDir to guarantee nothing above happens to have
	// a conformist config (avoid a flake where the test machine has
	// /conformist.toml).
	tmpDir := t.TempDir()
	deep := filepath.Join(tmpDir, "deep")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	// findConformistConfig walks all the way to /, so we need to ensure
	// the ancestor chain has no conformist configs. Since we just created
	// these directories, that's true for everything under tmpDir, but
	// the parents above tmpDir are outside our control. Skip if any
	// ancestor happens to have a config — vanishingly rare in CI.
	if dir, name, ok := findConformistConfig(deep); ok {
		t.Skipf("test environment has %s at %s (ancestor of %s)", name, dir, deep)
	}
}

func TestFormatOutput_NoConfigIsNoop(t *testing.T) {
	tt := test_ui.T{T: t}
	tmpDir := t.TempDir()
	mustMkdirAll(tt, filepath.Join(tmpDir, "pkgs"))

	if _, _, ok := findConformistConfig(tmpDir); ok {
		t.Skip("test environment has a conformist config in an ancestor directory")
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

	if err := os.WriteFile(filepath.Join(tmpDir, "conformist.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	sentinel := filepath.Join(tmpDir, "sentinel")
	withFakeConformist(tt, sentinel)

	exporter := &Exporter{Dir: tmpDir, OutputDir: "pkgs", DryRun: true}
	if err := exporter.FormatOutput(); err != nil {
		t.Fatalf("FormatOutput dry-run should be no-op, got: %v", err)
	}

	if _, err := os.Stat(sentinel); err == nil {
		t.Error("expected sentinel not to exist in dry-run mode; conformist should not have been invoked")
	}
}

func TestFormatOutput_MissingOutputDirIsNoop(t *testing.T) {
	tt := test_ui.T{T: t}
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "conformist.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	sentinel := filepath.Join(tmpDir, "sentinel")
	withFakeConformist(tt, sentinel)

	exporter := &Exporter{Dir: tmpDir, OutputDir: "pkgs"}
	if err := exporter.FormatOutput(); err != nil {
		t.Fatalf("FormatOutput with missing pkgs/ should be no-op, got: %v", err)
	}

	if _, err := os.Stat(sentinel); err == nil {
		t.Error("expected sentinel not to exist when output dir is missing")
	}
}

func TestFormatOutput_PropagatesConformistFailure(t *testing.T) {
	tt := test_ui.T{T: t}
	tmpDir := t.TempDir()
	mustMkdirAll(tt, filepath.Join(tmpDir, "pkgs"))

	if err := os.WriteFile(filepath.Join(tmpDir, "conformist.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	withFailingFakeConformist(tt)

	exporter := &Exporter{Dir: tmpDir, OutputDir: "pkgs"}
	err := exporter.FormatOutput()
	if err == nil {
		t.Fatal("expected FormatOutput to surface conformist failure")
	}
	if !strings.Contains(err.Error(), "conformist") {
		t.Errorf("expected error to mention conformist, got: %v", err)
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

// withFailingFakeConformist installs a fake conformist that exits non-zero.
func withFailingFakeConformist(t test_ui.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.T.Skip("PATH-injection fake binary not portable to Windows")
	}

	binDir := t.TempDir()
	fake := filepath.Join(binDir, "conformist")
	script := "#!/bin/sh\nexit 7\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	prependPath(t, binDir)
}

// withTreeRootAwareFakeConformist installs a fake `conformist` that models
// tree-root anchoring: it only "formats" .go files located within its working
// directory (FormatOutput sets that to the config/module root) and is a no-op
// for any path outside it. Formatting appends a sentinel line, so a file the
// fake skipped is detectably different from one it processed. Used to
// reproduce #125, where the comparison copy lived outside the tree root and
// was silently left unformatted.
//
// The script body deliberately avoids the literal tree-root flag names:
// conformistBakesTreeRoot scans a `#!` script for them and would otherwise
// misclassify this (script) fake as the Nix wrapper (purse-first#162).
func withTreeRootAwareFakeConformist(t test_ui.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.T.Skip("PATH-injection fake binary not portable to Windows")
	}

	binDir := t.TempDir()
	fake := filepath.Join(binDir, "conformist")
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
    grep -q '//conformist-formatted' "$f" || printf '//conformist-formatted\n' >>"$f"
  done
done
exit 0
`
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
// directory and prepends it to PATH. The fake records its argv into
// sentinelPath.
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

// TestFindConformistConfig_CeilingStopsEscalation is the purse-first#159
// regression: a config only in an ANCESTOR is NOT found when
// DAGNABIT_CEILING_DIRECTORIES bounds the walk below that ancestor. Models the
// real failure — a repo with a Nix-generated conformist config (none on disk)
// must not escalate to a stray ancestor conformist.toml.
func TestFindConformistConfig_CeilingStopsEscalation(t *testing.T) {
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

	if dir, name, ok := findConformistConfig(start); ok {
		t.Errorf("expected ceiling to stop escalation to ancestor config, but found %s at %s", name, dir)
	}
}

// TestFindConformistConfig_CeilingAllowsInTreeConfig confirms the ceiling does
// not block finding a config at or below the start: a config at the repo root
// is still found with the ceiling set at the repo's parent.
func TestFindConformistConfig_CeilingAllowsInTreeConfig(t *testing.T) {
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

	dir, name, ok := findConformistConfig(start)
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

	args := readSentinelArgs(tt, sentinel)
	if !slices.Contains(args, "--config-file") {
		t.Errorf("expected conformist to be invoked with --config-file, got args=%v", args)
	}
	if !slices.Contains(args, configFile) {
		t.Errorf("expected conformist args to include the explicit config %q, got args=%v", configFile, args)
	}
}

// TestFormatOutput_InvokesConformist confirms a conformist.toml config drives
// the `conformist` binary with the output dir as the positional argument.
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

	args := readSentinelArgs(tt, sentinel)
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

// argAfter returns the argv element following flag, or "" when absent.
func argAfter(args []string, flag string) string {
	index := slices.Index(args, flag)
	if index < 0 || index+1 >= len(args) {
		return ""
	}
	return args[index+1]
}

// withFacadeConfig writes body as a facade config and points
// DAGNABIT_CONFORMIST_CONFIG at it, with a ceiling at dir so discovery could
// never find anything else.
func withFacadeConfig(t test_ui.T, dir, body string) string {
	t.Helper()
	configFile := filepath.Join(dir, "facade-conformist.toml")
	if err := os.WriteFile(configFile, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DAGNABIT_CEILING_DIRECTORIES", absForTest(t, dir))
	t.Setenv("DAGNABIT_CONFORMIST_CONFIG", configFile)
	return configFile
}

// writeGeneratedFacade plants a generated file at <root>/pkgs/widget/main.go.
func writeGeneratedFacade(t test_ui.T, root string) {
	t.Helper()
	dir := filepath.Join(root, "pkgs", "widget")
	mustMkdirAll(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package widget\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestFormatOutput_TreeRootIsModuleRoot: the tree root is the module root, not
// the pkgs/ output dir, so a config sees generated files as `pkgs/...`. With
// the root at pkgs/ a formatter working-dir resolved to pkgs/<dir> and facade
// excludes stopped matching (the chrest regression after purse-first#195).
func TestFormatOutput_TreeRootIsModuleRoot(t *testing.T) {
	tt := test_ui.T{T: t}
	tmpDir := absForTest(tt, t.TempDir())
	writeGeneratedFacade(tt, tmpDir)
	withFacadeConfig(tt, tmpDir, "")

	sentinel := filepath.Join(tmpDir, "sentinel")
	withFakeConformist(tt, sentinel)

	exporter := &Exporter{Dir: tmpDir, OutputDir: "pkgs"}
	if err := exporter.FormatOutput(); err != nil {
		t.Fatalf("FormatOutput: %v", err)
	}

	args := readSentinelArgs(tt, sentinel)
	if got := argAfter(args, "--tree-root"); got != tmpDir {
		t.Errorf("expected --tree-root %s (module root), got %q; args=%v", tmpDir, got, args)
	}
	if last := args[len(args)-1]; last != filepath.Join(tmpDir, "pkgs") {
		t.Errorf("expected the pkgs/ output dir as the positional path, got %q", last)
	}
}

// TestFormatOutput_TreeRootFollowsOutputRoot: under `export --check` the output
// is rendered into an in-tree temp root, and the tree root moves with it so the
// comparison copy is also seen as `pkgs/...`.
func TestFormatOutput_TreeRootFollowsOutputRoot(t *testing.T) {
	tt := test_ui.T{T: t}
	tmpDir := absForTest(tt, t.TempDir())
	checkRoot := filepath.Join(tmpDir, ".dagnabit-check-test")
	writeGeneratedFacade(tt, checkRoot)
	withFacadeConfig(tt, tmpDir, "")

	sentinel := filepath.Join(tmpDir, "sentinel")
	withFakeConformist(tt, sentinel)

	exporter := &Exporter{Dir: tmpDir, OutputDir: "pkgs", OutputRoot: checkRoot}
	if err := exporter.FormatOutput(); err != nil {
		t.Fatalf("FormatOutput: %v", err)
	}

	args := readSentinelArgs(tt, sentinel)
	if got := argAfter(args, "--tree-root"); got != checkRoot {
		t.Errorf("expected --tree-root %s (check temp root), got %q; args=%v", checkRoot, got, args)
	}
}

// TestFormatOutput_StripsAmbientTreeRootEnv: an ambient CONFORMIST_TREE_ROOT_FILE
// alongside dagnabit's explicit --tree-root is a hard error in conformist, so
// the child env must not carry any tree-root variable.
func TestFormatOutput_StripsAmbientTreeRootEnv(t *testing.T) {
	tt := test_ui.T{T: t}
	if runtime.GOOS == "windows" {
		t.Skip("PATH-injection fake binary not portable to Windows")
	}
	tmpDir := t.TempDir()
	writeGeneratedFacade(tt, tmpDir)
	withFacadeConfig(tt, tmpDir, "")

	for _, name := range conformistTreeRootEnvVars {
		t.Setenv(name, "flake.nix")
	}

	envDump := filepath.Join(tmpDir, "env")
	binDir := t.TempDir()
	script := fmt.Sprintf("#!/bin/sh\nenv > %q\n", envDump)
	if err := os.WriteFile(filepath.Join(binDir, "conformist"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	prependPath(tt, binDir)

	exporter := &Exporter{Dir: tmpDir, OutputDir: "pkgs"}
	if err := exporter.FormatOutput(); err != nil {
		t.Fatalf("FormatOutput: %v", err)
	}

	env, err := os.ReadFile(envDump)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(env), "\n") {
		name, _, _ := strings.Cut(line, "=")
		if slices.Contains(conformistTreeRootEnvVars, name) {
			t.Errorf("conformist child env still carries %s", line)
		}
	}
}

// TestFormatOutput_FacadeConfigDefaultsPass: the excludes every generated
// conformist config carries (global defaults, per-formatter vendor/*) do not
// match generated files, so a config built from the dagnabit-facade module is
// accepted.
func TestFormatOutput_FacadeConfigDefaultsPass(t *testing.T) {
	tt := test_ui.T{T: t}
	tmpDir := t.TempDir()
	writeGeneratedFacade(tt, tmpDir)
	withFacadeConfig(tt, tmpDir, `excludes = ["*.lock", "*.patch", "go.mod", "go.sum", "LICENSE"]

[formatter]
[formatter.goimports]
command = "goimports"
excludes = ["vendor/*"]
includes = ["*.go"]
priority = 1
`)
	withFakeConformist(tt, filepath.Join(tmpDir, "sentinel"))

	exporter := &Exporter{Dir: tmpDir, OutputDir: "pkgs"}
	if err := exporter.FormatOutput(); err != nil {
		t.Fatalf("expected a formatters-only facade config to pass, got: %v", err)
	}
}

// TestFormatOutput_RejectsNonFacadeConfig: a repo's own config passed as the
// facade config is refused before conformist runs, naming each offending key.
// Each case is one of the breakages this contract exists to prevent.
func TestFormatOutput_RejectsNonFacadeConfig(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
	}{
		{
			name: "linter",
			body: "[linter.codegen-repair]\ncommand = \"x\"\n",
			want: `declares linter "codegen-repair"`,
		},
		{
			name: "working-dir",
			body: "[formatter.goimports]\ncommand = \"goimports\"\nworking-dir = \"go\"\n",
			want: `formatter "goimports" sets working-dir "go"`,
		},
		{
			name: "global exclude",
			body: "excludes = [\"pkgs/**\"]\n",
			want: `global exclude "pkgs/**" matches generated file pkgs/widget/main.go`,
		},
		{
			name: "formatter exclude",
			body: "[formatter.gofumpt]\ncommand = \"gofumpt\"\nexcludes = [\"**/main.go\"]\n",
			want: `formatter "gofumpt" exclude "**/main.go" matches generated file pkgs/widget/main.go`,
		},
		{
			name: "skip-generated",
			body: "skip-generated = true\n",
			want: "sets skip-generated",
		},
		{
			name: "tree-root-file",
			body: "tree-root-file = \"flake.nix\"\n",
			want: "sets tree-root-file",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tt := test_ui.T{T: t}
			tmpDir := t.TempDir()
			writeGeneratedFacade(tt, tmpDir)
			withFacadeConfig(tt, tmpDir, tc.body)

			sentinel := filepath.Join(tmpDir, "sentinel")
			withFakeConformist(tt, sentinel)

			exporter := &Exporter{Dir: tmpDir, OutputDir: "pkgs"}
			err := exporter.FormatOutput()
			if err == nil {
				t.Fatal("expected the non-facade config to be rejected")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("expected error to contain %q, got: %v", tc.want, err)
			}
			if _, statErr := os.Stat(sentinel); statErr == nil {
				t.Error("conformist ran despite the rejected config")
			}
		})
	}
}

// TestFormatOutput_FacadeConfigRefusesWrapper: the Nix wrapper bakes its own
// tree root, so the facade config's tree-root-relative validation would not
// describe what it matches. With DAGNABIT_CONFORMIST_CONFIG set, dagnabit
// requires the raw binary instead of running the wrapper.
func TestFormatOutput_FacadeConfigRefusesWrapper(t *testing.T) {
	tt := test_ui.T{T: t}
	tmpDir := t.TempDir()
	writeGeneratedFacade(tt, tmpDir)
	withFacadeConfig(tt, tmpDir, "")

	sentinel := filepath.Join(tmpDir, "sentinel")
	withFakeWrapperConformist(tt, sentinel)

	exporter := &Exporter{Dir: tmpDir, OutputDir: "pkgs"}
	err := exporter.FormatOutput()
	if err == nil || !strings.Contains(err.Error(), "Nix wrapper") {
		t.Fatalf("expected the wrapper to be refused with a facade config, got: %v", err)
	}
	if _, statErr := os.Stat(sentinel); statErr == nil {
		t.Error("the wrapper ran despite being refused")
	}
}

// TestFormatOutput_DiscoveryAnchorsAtConfigDir: the deprecated discovery path
// formats with the repo's own config, whose excludes and working-dir are
// written relative to the config's directory, so that is its tree root — not
// the module root (chrest-style: config at the repo root, module in go/).
func TestFormatOutput_DiscoveryAnchorsAtConfigDir(t *testing.T) {
	tt := test_ui.T{T: t}
	repo := absForTest(tt, t.TempDir())
	module := filepath.Join(repo, "go")
	writeGeneratedFacade(tt, module)
	if err := os.WriteFile(filepath.Join(repo, "conformist.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DAGNABIT_CONFORMIST_CONFIG", "")

	sentinel := filepath.Join(repo, "sentinel")
	withFakeConformist(tt, sentinel)

	exporter := &Exporter{Dir: module, OutputDir: "pkgs"}
	if err := exporter.FormatOutput(); err != nil {
		t.Fatalf("FormatOutput: %v", err)
	}

	args := readSentinelArgs(tt, sentinel)
	if got := argAfter(args, "--tree-root"); got != repo {
		t.Errorf("expected --tree-root %s (the discovered config's dir), got %q; args=%v", repo, got, args)
	}
}

// TestConformistBakesTreeRoot_RawBinaryWithFlagLiteral is the purse-first#195
// regression: the raw conformist binary carries the literal tree-root flag
// names in its own help/warning strings ("pass --tree-root to override"), so a
// non-script whose bytes contain them must NOT be classified as the wrapper.
// A `#!` script with the same bytes still is (purse-first#162).
func TestConformistBakesTreeRoot_RawBinaryWithFlagLiteral(t *testing.T) {
	dir := t.TempDir()
	flagText := "\x00no tree root found; pass --tree-root to override\x00--tree-root-file\x00"

	raw := filepath.Join(dir, "raw-conformist")
	if err := os.WriteFile(raw, []byte("\x7fELF\x02\x01\x01"+flagText), 0o755); err != nil {
		t.Fatal(err)
	}
	if conformistBakesTreeRoot(raw) {
		t.Errorf("raw (non-script) conformist containing %q was misclassified as the wrapper", "--tree-root")
	}

	wrapper := filepath.Join(dir, "wrapper-conformist")
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\n"+flagText), 0o755); err != nil {
		t.Fatal(err)
	}
	if !conformistBakesTreeRoot(wrapper) {
		t.Error("script conformist baking a tree-root flag was not classified as the wrapper")
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
