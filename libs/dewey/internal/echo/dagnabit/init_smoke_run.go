package dagnabit

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// wasmExecStrictHarness is the JS entry that instantiates a GOOS=js test binary
// under Go's generic wasm_exec.js (stub filesystem), written to a temp file at
// run time and passed to bun. See wasm_exec_strict.js for why the strict loader
// is the sound default (purse-first#177 / FDR 0014).
//
//go:embed wasm_exec_strict.js
var wasmExecStrictHarness []byte

const (
	// loaderStrict is the default: for js, bun + generic wasm_exec.js (stub FS,
	// ENOSYS); for wasip1, wasmtime with no preopens. Both reproduce the
	// arch's real filesystem strictness so an init hazard surfaces.
	loaderStrict = "strict"
	// loaderNode runs js/wasm under Go's stock go_js_wasm_exec (wasm_exec_node.js,
	// real filesystem). The false-confidence path; opt-in only.
	loaderNode = "node"
)

// Run builds and instantiates the generated per-arch test under each arch's
// declared loader, so a package init() that fails on the arch surfaces as a
// load-time failure naming the offender (via the panic stack `go test` prints).
// Returns an error listing the arch(es) whose run failed.
func (is *InitSmoke) Run(cfg InitSmokeConfig) error {
	goroot, err := goRoot(is.baseEnv())
	if err != nil {
		return err
	}

	// coreutils dir for the env -i scrub: GOROOT's wasm wrapper scripts shell
	// out to dirname/readlink, and `env`/coreutils must stay reachable.
	coreDir, err := binDir("env")
	if err != nil {
		return fmt.Errorf("locating coreutils on PATH: %w", err)
	}

	var failed []string
	for _, arch := range cfg.Arch {
		if err := is.runArch(arch, goroot, coreDir); err != nil {
			fmt.Fprintf(os.Stderr, "init-smoke run %s/%s: %v\n", arch.GOOS, arch.GOARCH, err)
			failed = append(failed, arch.GOOS+"/"+arch.GOARCH)
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("init-smoke run failed for: %s", strings.Join(failed, ", "))
	}

	return nil
}

// runArch runs `GOOS/GOARCH go test -exec=<loader> ./<outputDir>` for one arch.
func (is *InitSmoke) runArch(arch InitSmokeArch, goroot, coreDir string) error {
	harnessPath, cleanup, err := writeStrictHarness()
	if err != nil {
		return err
	}
	defer cleanup()

	wrapper, err := is.execWrapper(arch, goroot, coreDir, harnessPath)
	if err != nil {
		return err
	}

	// go test -exec takes a single string it splits on spaces into program +
	// args, then appends the compiled test binary and test flags.
	execStr := strings.Join(wrapper, " ")
	pkg := "./" + is.outputDir()

	env := append(is.baseEnv(), "GOOS="+arch.GOOS, "GOARCH="+arch.GOARCH)

	if is.DryRun {
		fmt.Printf("GOOS=%s GOARCH=%s go test -exec=%q %s\n", arch.GOOS, arch.GOARCH, execStr, pkg)
		return nil
	}

	cmd := exec.Command("go", "test", "-exec="+execStr, pkg)
	cmd.Dir = is.Dir
	cmd.Env = env
	// Stream output so an init panic (and the offending package in its stack)
	// is visible directly.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// execWrapper builds the `go test -exec` program+args for arch's loader. The
// env -i scrub bounds the environment handed to the wasm host: wasm_exec_node.js
// (and any real-FS loader) forwards the whole ambient environment to the module,
// and Go.run rejects an argv+env payload over its size cap — which a nix devshell
// environment alone exceeds. The strict bun harness sets the module env to {},
// so the scrub is belt-and-suspenders there but essential for the node loader.
func (is *InitSmoke) execWrapper(arch InitSmokeArch, goroot, coreDir, harnessPath string) ([]string, error) {
	loader := arch.Loader
	if loader == "" {
		loader = loaderStrict
	}

	switch arch.GOOS {
	case "js":
		switch loader {
		case loaderStrict:
			bunDir, err := binDir("bun")
			if err != nil {
				return nil, fmt.Errorf("strict js loader needs `bun` on PATH: %w", err)
			}
			wasmExec := findGorootWasm(goroot, "wasm_exec.js")
			if wasmExec == "" {
				return nil, fmt.Errorf("no wasm_exec.js under %s (lib/wasm or misc/wasm)", goroot)
			}
			return []string{
				"env", "-i",
				"PATH=" + bunDir + ":" + coreDir,
				"DAGNABIT_WASM_EXEC=" + wasmExec,
				"bun", harnessPath,
			}, nil
		case loaderNode:
			nodeDir, err := binDir("node")
			if err != nil {
				return nil, fmt.Errorf("node js loader needs `node` on PATH: %w", err)
			}
			wrapper := findGorootWasm(goroot, "go_js_wasm_exec")
			if wrapper == "" {
				return nil, fmt.Errorf("no go_js_wasm_exec under %s (lib/wasm or misc/wasm)", goroot)
			}
			return []string{"env", "-i", "PATH=" + nodeDir + ":" + coreDir, wrapper}, nil
		default:
			return is.repoLoaderWrapper(loader, coreDir)
		}

	case "wasip1":
		switch loader {
		case loaderStrict:
			wtDir, err := binDir("wasmtime")
			if err != nil {
				return nil, fmt.Errorf("strict wasip1 loader needs `wasmtime` on PATH: %w", err)
			}
			// No --dir preopens: the guest gets no ambient filesystem, so a
			// filesystem-touching init fails — the strict behavior we want.
			return []string{"env", "-i", "PATH=" + wtDir + ":" + coreDir, "wasmtime", "run"}, nil
		default:
			return is.repoLoaderWrapper(loader, coreDir)
		}

	default:
		return nil, fmt.Errorf(
			"unsupported goos %q for init-smoke run (supported: js, wasip1)", arch.GOOS,
		)
	}
}

// repoLoaderWrapper handles a loader that names a repo-provided executable
// (relative to the module root, or absolute): the exact shim the repo ships. The
// contract is that it is an executable taking the test binary as its first
// argument (followed by test flags). It is invoked under the same env -i scrub;
// a shim needing a runtime beyond coreutils must be a self-contained wrapper.
func (is *InitSmoke) repoLoaderWrapper(loader, coreDir string) ([]string, error) {
	p := loader
	if !filepath.IsAbs(p) {
		p = filepath.Join(is.Dir, loader)
	}

	info, err := os.Stat(p)
	if err != nil {
		return nil, fmt.Errorf("loader %q: %w", loader, err)
	}
	if info.IsDir() || info.Mode()&0o111 == 0 {
		return nil, fmt.Errorf("loader %q is not an executable file", loader)
	}

	return []string{"env", "-i", "PATH=" + coreDir, p}, nil
}

// writeStrictHarness writes the embedded strict harness to a temp file and
// returns its path plus a cleanup func. Written every run (cheap) so the path
// is always valid regardless of which loader an arch uses.
func writeStrictHarness() (string, func(), error) {
	f, err := os.CreateTemp("", "dagnabit-initsmoke-*.js")
	if err != nil {
		return "", func() {}, fmt.Errorf("creating harness temp file: %w", err)
	}

	if _, err := f.Write(wasmExecStrictHarness); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", func() {}, fmt.Errorf("writing harness: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", func() {}, fmt.Errorf("closing harness: %w", err)
	}

	path := f.Name()
	return path, func() { os.Remove(path) }, nil
}

// baseEnv is the environment for the `go` invocations (nil Env ⇒ process env).
func (is *InitSmoke) baseEnv() []string {
	if is.Env != nil {
		return is.Env
	}
	return os.Environ()
}

// goRoot returns `go env GOROOT` under env.
func goRoot(env []string) (string, error) {
	cmd := exec.Command("go", "env", "GOROOT")
	cmd.Env = env

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go env GOROOT: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}

// findGorootWasm locates a wasm helper (wasm_exec.js, go_js_wasm_exec, …) under
// GOROOT, checking lib/wasm (Go ≥ 1.21) then the older misc/wasm. Returns "" if
// absent.
func findGorootWasm(goroot, name string) string {
	for _, dir := range []string{"lib/wasm", "misc/wasm"} {
		p := filepath.Join(goroot, dir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// binDir returns the directory of the named binary as resolved on PATH.
func binDir(name string) (string, error) {
	p, err := exec.LookPath(name)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.Dir(abs), nil
}
