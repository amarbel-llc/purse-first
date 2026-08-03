package dagnabit

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	toml "github.com/BurntSushi/toml"
	jen "github.com/dave/jennifer/jen"
)

// initSmokeConfigFile is the config filename dagnabit reads init-smoke target
// arches from, at the module root.
const initSmokeConfigFile = "dagnabit.toml"

// defaultInitSmokeDir is where generated per-arch blank-import tests live,
// relative to the module root. It is a module-root sibling of internal/ and
// pkgs/ deliberately: facade export (--library scans internal/) and reposition
// (operates on internal/) both ignore it, so the generated package needs no
// private marker and no NATO-level placement.
const defaultInitSmokeDir = "initsmoke"

// initSmokePackageName is the Go package name for the generated tests. No
// underscore, so it does not trip package-name style linters.
const initSmokePackageName = "initsmoke"

// InitSmokeArch declares one (arch, loader, skiplist) init-smoke target.
type InitSmokeArch struct {
	GOOS   string   `toml:"goos"`
	GOARCH string   `toml:"goarch"`
	Loader string   `toml:"loader"`
	Skip   []string `toml:"skip"`
}

// tag returns the //go:build constraint expression for this arch, e.g.
// "js && wasm".
func (a InitSmokeArch) tag() string {
	return a.GOOS + " && " + a.GOARCH
}

// fileName returns the generated test filename for this arch, e.g.
// "initsmoke_js_wasm_test.go".
func (a InitSmokeArch) fileName() string {
	return fmt.Sprintf("initsmoke_%s_%s_test.go", a.GOOS, a.GOARCH)
}

// InitSmokeConfig is the [init-smoke] table of dagnabit.toml.
type InitSmokeConfig struct {
	Arch []InitSmokeArch `toml:"arch"`
}

// dagnabitConfig is the root of dagnabit.toml. init-smoke is its only table
// today; the file exists so target arches (which both the drift check and the
// run lane must read identically) are committed, code-reviewed data.
type dagnabitConfig struct {
	InitSmoke InitSmokeConfig `toml:"init-smoke"`
}

// LoadInitSmokeConfig reads dir/dagnabit.toml and returns its [init-smoke]
// table. A missing file yields ok=false with no error, so callers can tell
// "no config" apart from "config with zero arches".
func LoadInitSmokeConfig(dir string) (cfg InitSmokeConfig, ok bool, err error) {
	p := filepath.Join(dir, initSmokeConfigFile)

	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return InitSmokeConfig{}, false, nil
		}

		return InitSmokeConfig{}, false, fmt.Errorf("reading %s: %w", p, err)
	}

	var root dagnabitConfig
	if err := toml.Unmarshal(data, &root); err != nil {
		return InitSmokeConfig{}, false, fmt.Errorf("parsing %s: %w", p, err)
	}

	return root.InitSmoke, true, nil
}

// InitSmoke generates and drift-checks per-arch blank-import init tests. Each
// generated file blank-imports every buildable, non-skipped package for its
// arch, so instantiating the test binary runs every package's init() and a
// per-arch init hazard (e.g. a /dev/null open that fails on the js/wasm strict
// filesystem, purse-first#177) surfaces as a load-time panic.
type InitSmoke struct {
	ModulePath string
	Dir        string // module root; dagnabit.toml + `go list` run from here

	// OutputDir is the generated-tests directory relative to the module root.
	// Defaults to "initsmoke".
	OutputDir string

	DryRun bool

	// OutputRoot, when non-empty, is the filesystem root under which generated
	// files are written (and formatted), in place of Dir. Package enumeration
	// and formatter-config lookup still key off Dir; only the output location
	// moves. Check() uses this to render+format into a temp dir for a
	// no-mutation drift comparison against the real Dir/<outputDir>.
	OutputRoot string

	// Env, when non-nil, is the base environment for the `go list` enumeration
	// (the arch GOOS/GOARCH are appended per call). nil uses os.Environ().
	Env []string
}

func (is *InitSmoke) outputDir() string {
	if is.OutputDir != "" {
		return is.OutputDir
	}

	return defaultInitSmokeDir
}

func (is *InitSmoke) outputRoot() string {
	if is.OutputRoot != "" {
		return is.OutputRoot
	}

	return is.Dir
}

// Generate regenerates the per-arch blank-import test files for every arch in
// cfg under outputRoot/outputDir, removes stale per-arch files (from a removed
// arch), and formats the result with conformist so the output matches what the
// repo's formatter produces. A hand-written doc.go in the output dir (not
// touched here) keeps the package building on non-target hosts.
func (is *InitSmoke) Generate(cfg InitSmokeConfig) error {
	outDir := filepath.Join(is.outputRoot(), is.outputDir())

	if !is.DryRun {
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", outDir, err)
		}
	}

	generated := make(map[string]struct{}, len(cfg.Arch))

	for _, arch := range cfg.Arch {
		imports, err := is.enumerate(arch)
		if err != nil {
			return fmt.Errorf("enumerating %s/%s: %w", arch.GOOS, arch.GOARCH, err)
		}

		body, err := renderInitSmokeFile(arch, imports)
		if err != nil {
			return fmt.Errorf("rendering %s/%s: %w", arch.GOOS, arch.GOARCH, err)
		}

		name := arch.fileName()
		generated[name] = struct{}{}

		if is.DryRun {
			continue
		}

		if err := os.WriteFile(filepath.Join(outDir, name), body, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
	}

	if !is.DryRun {
		if err := removeStaleArchFiles(outDir, generated); err != nil {
			return err
		}
	}

	return is.formatOutput()
}

// Check regenerates the per-arch tests into a temp dir, formats them, and
// byte-compares against the committed files without mutating the tree. It
// returns an error naming the out-of-sync arch files on drift (content drift, a
// missing committed file, or a stale committed file no arch produces), else
// nil. This is the side-effect-free equivalent of `dagnabit init-smoke`
// followed by `git diff --exit-code`.
func (is *InitSmoke) Check(cfg InitSmokeConfig) error {
	// Temp dir UNDER the module root (not the system temp dir), for the same
	// reason as the exporter's check: the conformist format pass anchors its
	// tree root inside the repo, so an out-of-tree output dir would be left
	// unformatted and report phantom drift. The dot prefix hides it from
	// `go list`; defer removes it.
	tmp, err := os.MkdirTemp(is.Dir, ".dagnabit-initsmoke-check-")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmp) //defer:err-checked temp-dir cleanup; failure must not clobber the real return

	clone := *is
	clone.OutputRoot = tmp
	clone.DryRun = false
	if err := clone.Generate(cfg); err != nil {
		return err
	}

	committedDir := filepath.Join(is.Dir, is.outputDir())
	freshDir := filepath.Join(tmp, is.outputDir())

	expected := make(map[string]struct{}, len(cfg.Arch))
	var drift []string
	checked := 0

	for _, arch := range cfg.Arch {
		name := arch.fileName()
		expected[name] = struct{}{}

		got, err := os.ReadFile(filepath.Join(freshDir, name))
		if err != nil {
			return fmt.Errorf("reading generated %s: %w", name, err)
		}
		checked++

		want, err := os.ReadFile(filepath.Join(committedDir, name))
		if os.IsNotExist(err) {
			drift = append(drift, name+" (missing — not committed)")
			continue
		}
		if err != nil {
			return fmt.Errorf("reading committed %s: %w", name, err)
		}

		if !bytes.Equal(want, got) {
			drift = append(drift, name+" (out of date)")
		}
	}

	stale, err := staleArchFiles(committedDir, expected)
	if err != nil {
		return err
	}
	drift = append(drift, stale...)

	sort.Strings(drift)
	if len(drift) > 0 {
		return fmt.Errorf(
			"init-smoke tests are out of sync with the package graph (run `dagnabit init-smoke` and commit):\n  %s",
			strings.Join(drift, "\n  "),
		)
	}

	fmt.Printf("init-smoke tests in sync with the package graph (%d arch file(s) checked)\n", checked)

	return nil
}

// renderInitSmokeFile produces the blank-import test file for one arch: a
// //go:build-constrained file that blank-imports every enumerated package and
// declares a trivial test so `go test` builds and instantiates the binary,
// running every imported package's init(). Output is canonicalized by the
// caller's conformist pass, so jennifer's exact spacing does not matter.
func renderInitSmokeFile(arch InitSmokeArch, imports []string) ([]byte, error) {
	f := jen.NewFile(initSmokePackageName)
	f.HeaderComment(generatedHeaderText())
	f.HeaderComment("//go:build " + arch.tag())

	for _, imp := range imports {
		f.Anon(imp)
	}

	// The blank imports do the work (their init() runs at binary
	// instantiation); this test exists so `go test` produces a binary at all.
	f.Func().Id("TestInitSmoke").Params(
		jen.Id("t").Op("*").Qual("testing", "T"),
	).Block()

	var buf bytes.Buffer
	if err := f.Render(&buf); err != nil {
		return nil, fmt.Errorf("rendering: %w", err)
	}

	return buf.Bytes(), nil
}

// isInitSmokeArchFile reports whether name is a generated per-arch test file
// (initsmoke_<goos>_<goarch>_test.go), the only files Generate writes and the
// only ones the drift check and stale-removal consider. The hand-written
// doc.go is deliberately excluded.
func isInitSmokeArchFile(name string) bool {
	return strings.HasPrefix(name, "initsmoke_") && strings.HasSuffix(name, "_test.go")
}

// staleArchFiles returns drift entries for committed per-arch files under dir
// that no configured arch produces (e.g. a removed arch). A missing dir is not
// an error (nothing committed yet).
func staleArchFiles(dir string, expected map[string]struct{}) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}

	var stale []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		name := e.Name()
		if !isInitSmokeArchFile(name) {
			continue
		}

		if _, ok := expected[name]; !ok {
			stale = append(stale, name+" (stale — no longer generated)")
		}
	}

	sort.Strings(stale)

	return stale, nil
}

// removeStaleArchFiles deletes committed per-arch files under dir that the
// current run did not generate, so a removed arch's file does not linger.
func removeStaleArchFiles(dir string, generated map[string]struct{}) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		name := e.Name()
		if !isInitSmokeArchFile(name) {
			continue
		}

		if _, ok := generated[name]; !ok {
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				return fmt.Errorf("removing stale %s: %w", name, err)
			}
		}
	}

	return nil
}

// formatOutput runs conformist on the generated output directory, mirroring the
// exporter's FormatOutput (same DAGNABIT_CONFORMIST_CONFIG / ceiling handling)
// so generated init-smoke files are byte-identical to what the repo formatter
// produces — keeping both the pure lint gate and the drift check green.
func (is *InitSmoke) formatOutput() error {
	if is.DryRun {
		return nil
	}

	outputPath := filepath.Join(is.outputRoot(), is.outputDir())

	if ok, err := outputDirExists(outputPath); err != nil {
		return err
	} else if !ok {
		return nil
	}

	// Explicit Nix-generated config short-circuits discovery (purse-first#159).
	if configFile := os.Getenv(conformistConfigEnvVar); configFile != "" {
		return runConformist(is.Dir, outputPath, configFile)
	}

	configDir, configName, ok := findConformistConfig(is.Dir)
	if !ok {
		return nil
	}

	if _, err := exec.LookPath("conformist"); err != nil {
		return fmt.Errorf(
			"formatter config %s found at %s, but `conformist` is not on PATH;"+
				" refusing to skip formatting — run inside the dev shell so"+
				" `conformist` is available",
			configName, configDir,
		)
	}

	return runConformist(configDir, outputPath, "")
}

// initSmokeSelfImport is the import path of the generated package itself, which
// must never appear in its own blank-import list.
func (is *InitSmoke) initSmokeSelfImport() string {
	return path.Join(is.ModulePath, is.outputDir())
}
