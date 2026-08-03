package dagnabit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitSmokeArchTagAndFileName(t *testing.T) {
	a := InitSmokeArch{GOOS: "js", GOARCH: "wasm"}

	if got, want := a.tag(), "js && wasm"; got != want {
		t.Errorf("tag() = %q, want %q", got, want)
	}

	if got, want := a.fileName(), "initsmoke_js_wasm_test.go"; got != want {
		t.Errorf("fileName() = %q, want %q", got, want)
	}
}

func TestLoadInitSmokeConfig(t *testing.T) {
	dir := t.TempDir()

	body := `
[[init-smoke.arch]]
goos = "js"
goarch = "wasm"
loader = "strict"
skip = ["internal/foo", "internal/bar"]

[[init-smoke.arch]]
goos = "wasip1"
goarch = "wasm"
loader = "strict"
`
	if err := os.WriteFile(filepath.Join(dir, initSmokeConfigFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, ok, err := LoadInitSmokeConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("ok = false for a present config")
	}
	if len(cfg.Arch) != 2 {
		t.Fatalf("got %d arches, want 2", len(cfg.Arch))
	}

	js := cfg.Arch[0]
	if js.GOOS != "js" || js.GOARCH != "wasm" || js.Loader != "strict" {
		t.Errorf("arch[0] = %+v, want js/wasm/strict", js)
	}
	if len(js.Skip) != 2 || js.Skip[0] != "internal/foo" || js.Skip[1] != "internal/bar" {
		t.Errorf("arch[0].Skip = %v, want [internal/foo internal/bar]", js.Skip)
	}

	if wasip1 := cfg.Arch[1]; wasip1.GOOS != "wasip1" || len(wasip1.Skip) != 0 {
		t.Errorf("arch[1] = %+v, want wasip1/wasm with empty skip", wasip1)
	}
}

func TestLoadInitSmokeConfigMissing(t *testing.T) {
	_, ok, err := LoadInitSmokeConfig(t.TempDir())
	if err != nil {
		t.Fatalf("a missing config must not error, got %v", err)
	}
	if ok {
		t.Error("ok = true for a missing config")
	}
}

func TestRenderInitSmokeFile(t *testing.T) {
	arch := InitSmokeArch{GOOS: "js", GOARCH: "wasm"}
	imports := []string{
		"example.com/m/internal/b",
		"example.com/m/internal/a",
	}

	body, err := renderInitSmokeFile(arch, imports)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)

	for _, want := range []string{
		"//go:build js && wasm",
		"DO NOT EDIT",
		"package initsmoke",
		`_ "example.com/m/internal/a"`,
		`_ "example.com/m/internal/b"`,
		"func TestInitSmoke(t *testing.T)",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("rendered file missing %q\n---\n%s", want, s)
		}
	}

	// Deterministic: the same inputs must render byte-identically, or the
	// drift check would flap.
	body2, err := renderInitSmokeFile(arch, imports)
	if err != nil {
		t.Fatal(err)
	}
	if string(body2) != s {
		t.Error("renderInitSmokeFile is not deterministic for identical inputs")
	}
}

func TestRenderInitSmokeFileNoImports(t *testing.T) {
	// An arch with no eligible packages still produces a valid, buildable file
	// (just the test function), so the drift check and run lane have something
	// to operate on rather than a missing file.
	body, err := renderInitSmokeFile(InitSmokeArch{GOOS: "wasip1", GOARCH: "wasm"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "//go:build wasip1 && wasm") {
		t.Errorf("missing build tag:\n%s", s)
	}
	if !strings.Contains(s, "func TestInitSmoke(t *testing.T)") {
		t.Errorf("missing test function:\n%s", s)
	}
}

func TestIsInitSmokeArchFile(t *testing.T) {
	cases := map[string]bool{
		"initsmoke_js_wasm_test.go":     true,
		"initsmoke_wasip1_wasm_test.go": true,
		"doc.go":                        false,
		"initsmoke_js_wasm.go":          false, // not a _test.go file
		"foo_test.go":                   false, // not an initsmoke_ file
		"initsmoke.go":                  false,
	}

	for name, want := range cases {
		if got := isInitSmokeArchFile(name); got != want {
			t.Errorf("isInitSmokeArchFile(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestStaleArchFiles(t *testing.T) {
	dir := t.TempDir()

	for _, n := range []string{
		"initsmoke_js_wasm_test.go",
		"initsmoke_wasip1_wasm_test.go",
		"doc.go",
	} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("package initsmoke\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Only js is still produced; wasip1 is stale, doc.go is never an arch file.
	expected := map[string]struct{}{"initsmoke_js_wasm_test.go": {}}

	stale, err := staleArchFiles(dir, expected)
	if err != nil {
		t.Fatal(err)
	}
	if len(stale) != 1 || !strings.Contains(stale[0], "initsmoke_wasip1_wasm_test.go") {
		t.Errorf("staleArchFiles = %v, want only the wasip1 file", stale)
	}
}

func TestStaleArchFilesMissingDir(t *testing.T) {
	stale, err := staleArchFiles(filepath.Join(t.TempDir(), "does-not-exist"), nil)
	if err != nil {
		t.Fatalf("a missing dir must not error, got %v", err)
	}
	if len(stale) != 0 {
		t.Errorf("stale = %v, want none", stale)
	}
}
