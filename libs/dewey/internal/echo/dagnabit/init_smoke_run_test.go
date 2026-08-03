package dagnabit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecWrapperUnsupportedGOOS(t *testing.T) {
	is := &InitSmoke{Dir: t.TempDir(), ModulePath: "example.com/m"}

	// An unsupported goos hits the error path before any runtime lookup, so this
	// needs no bun/wasmtime on PATH.
	_, err := is.execWrapper(
		InitSmokeArch{GOOS: "linux", GOARCH: "amd64", Loader: "strict"},
		"/fake/goroot", "/usr/bin", "/tmp/harness.js",
	)
	if err == nil {
		t.Fatal("expected an error for an unsupported goos, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported goos") {
		t.Errorf("error = %v, want it to mention 'unsupported goos'", err)
	}
}

func TestRepoLoaderWrapper(t *testing.T) {
	dir := t.TempDir()
	is := &InitSmoke{Dir: dir, ModulePath: "example.com/m"}
	coreDir := "/usr/bin"

	// A repo-relative executable loader resolves against the module root and is
	// invoked under the env -i scrub.
	loaderRel := "loaders/strict.sh"
	loaderAbs := filepath.Join(dir, loaderRel)
	if err := os.MkdirAll(filepath.Dir(loaderAbs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(loaderAbs, []byte("#!/bin/sh\nexec \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	wrapper, err := is.repoLoaderWrapper(loaderRel, coreDir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"env", "-i", "PATH=" + coreDir, loaderAbs}
	if strings.Join(wrapper, " ") != strings.Join(want, " ") {
		t.Errorf("wrapper = %v, want %v", wrapper, want)
	}
}

func TestRepoLoaderWrapperNonExecutable(t *testing.T) {
	dir := t.TempDir()
	is := &InitSmoke{Dir: dir, ModulePath: "example.com/m"}

	nonExec := filepath.Join(dir, "not-exec.txt")
	if err := os.WriteFile(nonExec, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := is.repoLoaderWrapper("not-exec.txt", "/usr/bin"); err == nil {
		t.Error("expected an error for a non-executable loader, got nil")
	}
}

func TestRepoLoaderWrapperMissing(t *testing.T) {
	is := &InitSmoke{Dir: t.TempDir(), ModulePath: "example.com/m"}

	if _, err := is.repoLoaderWrapper("does/not/exist.sh", "/usr/bin"); err == nil {
		t.Error("expected an error for a missing loader, got nil")
	}
}

func TestFindGorootWasm(t *testing.T) {
	goroot := t.TempDir()

	// Absent everywhere.
	if got := findGorootWasm(goroot, "wasm_exec.js"); got != "" {
		t.Errorf("findGorootWasm on empty goroot = %q, want \"\"", got)
	}

	// Present under lib/wasm (Go >= 1.21).
	libWasm := filepath.Join(goroot, "lib", "wasm")
	if err := os.MkdirAll(libWasm, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(libWasm, "wasm_exec.js")
	if err := os.WriteFile(want, []byte("// shim"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := findGorootWasm(goroot, "wasm_exec.js"); got != want {
		t.Errorf("findGorootWasm = %q, want %q", got, want)
	}
}

func TestFindGorootWasmMiscFallback(t *testing.T) {
	goroot := t.TempDir()

	// Only the older misc/wasm location exists.
	miscWasm := filepath.Join(goroot, "misc", "wasm")
	if err := os.MkdirAll(miscWasm, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(miscWasm, "go_js_wasm_exec")
	if err := os.WriteFile(want, []byte("#!/bin/sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findGorootWasm(goroot, "go_js_wasm_exec"); got != want {
		t.Errorf("findGorootWasm = %q, want %q", got, want)
	}
}

func TestWriteStrictHarness(t *testing.T) {
	path, cleanup, err := writeStrictHarness()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// The embedded harness must be the strict one (generic wasm_exec.js, no real
	// fs) — a canary against it being swapped for a permissive loader. The
	// meaningful signal is that it never assigns a real filesystem onto
	// globalThis.fs (which is exactly what wasm_exec_node.js does); mentioning
	// that name in a comment is fine, assigning globalThis.fs is not.
	s := string(body)
	if !strings.Contains(s, "DAGNABIT_WASM_EXEC") {
		t.Error("harness does not reference DAGNABIT_WASM_EXEC")
	}
	if strings.Contains(s, "globalThis.fs =") {
		t.Error("harness assigns globalThis.fs — the strict loader must NOT wire a real filesystem (that would mask init-time FS hazards)")
	}

	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("cleanup did not remove the harness temp file (stat err = %v)", err)
	}
}
