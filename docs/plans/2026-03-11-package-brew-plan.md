# Package Brew Command Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Add a `purse-first package brew` command that produces a complete Homebrew tap (formulas, tarballs, marketplace.json, README) from a `brew-config.json` file.

**Architecture:** New `internal/packagebrew/` package with config parsing, tarball creation, formula generation, and marketplace.json generation. Cobra subcommand `package brew` added under a new `package` parent command. Reuses `internal/marketplace` for plugin discovery and marketplace generation. Removes unused Nix-based brew tap infrastructure.

**Tech Stack:** Go (cobra CLI), `archive/tar` + `compress/gzip` for tarballs, `crypto/sha256` for hashes, `text/template` for formula rendering.

**Rollback:** Purely additive command. Removals are of unused code (`mkBrewTap.nix`, shell scripts). Revert the commits.

---

### Task 1: Remove Unused Nix Brew Tap Infrastructure

**Files:**
- Delete: `lib/mkBrewTap.nix`
- Delete: `bin/brew-build-tarball.bash`
- Delete: `bin/brew-update-hashes.bash`
- Delete: `zz-tests_bats/homebrew_tap.bats`
- Modify: `lib/mkMarketplace.nix:38,153-160,199`
- Modify: `flake.nix:234-248`
- Modify: `justfile:36-38,150-153`

**Step 1: Delete the files**

Remove `lib/mkBrewTap.nix`, `bin/brew-build-tarball.bash`, `bin/brew-update-hashes.bash`, and `zz-tests_bats/homebrew_tap.bats`.

**Step 2: Remove brewConfig from mkMarketplace.nix**

In `lib/mkMarketplace.nix`:
- Remove `brewConfig ? null,` parameter (line 38)
- Remove the `brewTap` block (lines 153-160):
  ```nix
    # Homebrew tap derivation (optional).
    brewTap =
      if brewConfig != null && pluginConfig != null then
        import ./mkBrewTap.nix {
          inherit pkgs pluginConfig brewConfig;
        }
      else
        null;
  ```
- Remove `// (if brewTap != null then { homebrew-tap = brewTap; } else { })` from the packages output (line 199)

**Step 3: Remove brewConfig from flake.nix**

Remove the `brewConfig = { ... };` block (lines 234-248).

**Step 4: Remove brew targets from justfile**

Remove `build-brew` (lines 36-38) and `test-brew` (lines 150-153) targets.

**Step 5: Verify nix build still works**

Run: `nix build --show-trace`
Expected: Builds successfully without homebrew-tap output.

**Step 6: Commit**

```
git add -A && git commit -m "chore: remove unused Nix brew tap infrastructure"
```

---

### Task 2: Add brew-config.json Types and Parsing

**Files:**
- Create: `internal/packagebrew/config.go`
- Create: `internal/packagebrew/config_test.go`

**Step 1: Write the failing test**

Create `internal/packagebrew/config_test.go`:

```go
package packagebrew

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "brew-config.json")

	data := []byte(`{
		"name": "test-marketplace",
		"description": "Test marketplace",
		"owner": {"name": "tester", "email": "test@example.com"},
		"releaseRepo": "org/homebrew-tap",
		"tapName": "org/tap",
		"license": "MIT",
		"packages": {
			"my-tool": {
				"description": "A test tool",
				"version": "1.0.0",
				"binary": true,
				"homepage": "https://example.com",
				"category": "development",
				"tags": ["test"],
				"platforms": {
					"darwin-arm64": "/path/to/bin/my-tool",
					"linux-amd64": "/path/to/bin/my-tool"
				},
				"share": "/path/to/share/purse-first/my-tool",
				"brewDeps": ["gh"]
			},
			"my-skills": {
				"description": "Skill-only package",
				"version": "0.1.0",
				"binary": false,
				"share": "/path/to/share/purse-first/my-skills",
				"brewDeps": []
			}
		}
	}`)

	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := ReadConfig(configPath)
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}

	if cfg.Name != "test-marketplace" {
		t.Errorf("Name = %q, want %q", cfg.Name, "test-marketplace")
	}

	if cfg.ReleaseRepo != "org/homebrew-tap" {
		t.Errorf("ReleaseRepo = %q, want %q", cfg.ReleaseRepo, "org/homebrew-tap")
	}

	if len(cfg.Packages) != 2 {
		t.Fatalf("len(Packages) = %d, want 2", len(cfg.Packages))
	}

	tool := cfg.Packages["my-tool"]
	if !tool.Binary {
		t.Error("my-tool.Binary = false, want true")
	}

	if len(tool.Platforms) != 2 {
		t.Errorf("my-tool.Platforms count = %d, want 2", len(tool.Platforms))
	}

	if tool.Platforms["darwin-arm64"] != "/path/to/bin/my-tool" {
		t.Errorf("my-tool.Platforms[darwin-arm64] = %q", tool.Platforms["darwin-arm64"])
	}

	skills := cfg.Packages["my-skills"]
	if skills.Binary {
		t.Error("my-skills.Binary = true, want false")
	}
}

func TestReadConfigMissingFile(t *testing.T) {
	_, err := ReadConfig("/nonexistent/path.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestReadConfigInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte(`{not json`), 0o644)

	_, err := ReadConfig(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test ./internal/packagebrew/... -v`
Expected: FAIL — package does not exist.

**Step 3: Write the config types and parser**

Create `internal/packagebrew/config.go`:

```go
package packagebrew

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	Owner       Owner                 `json:"owner"`
	ReleaseRepo string                `json:"releaseRepo"`
	TapName     string                `json:"tapName"`
	License     string                `json:"license"`
	Packages    map[string]PackageConfig `json:"packages"`
}

type Owner struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

type PackageConfig struct {
	Description string            `json:"description,omitempty"`
	Version     string            `json:"version,omitempty"`
	Binary      bool              `json:"binary"`
	Homepage    string            `json:"homepage,omitempty"`
	Category    string            `json:"category,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Platforms   map[string]string `json:"platforms,omitempty"`
	Share       string            `json:"share"`
	BrewDeps    []string          `json:"brewDeps,omitempty"`
}

func ReadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("reading brew config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing brew config: %w", err)
	}

	return cfg, nil
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test ./internal/packagebrew/... -v`
Expected: PASS

**Step 5: Commit**

```
git add internal/packagebrew/config.go internal/packagebrew/config_test.go
git commit -m "feat(packagebrew): add brew config types and parser"
```

---

### Task 3: Add Tarball Creation

**Files:**
- Create: `internal/packagebrew/tarball.go`
- Create: `internal/packagebrew/tarball_test.go`

**Step 1: Write the failing test**

Create `internal/packagebrew/tarball_test.go`:

```go
package packagebrew

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func setupTestPackage(t *testing.T) (binPath, shareDir string) {
	t.Helper()
	dir := t.TempDir()

	// Create binary.
	binPath = filepath.Join(dir, "my-tool")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\necho hello"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create share directory.
	shareDir = filepath.Join(dir, "share", "purse-first", "my-tool")
	pluginDir := filepath.Join(shareDir, ".claude-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(pluginDir, "plugin.json"),
		[]byte(`{"name":"my-tool"}`),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	return binPath, shareDir
}

func tarEntries(t *testing.T, tarballPath string) []string {
	t.Helper()
	f, err := os.Open(tarballPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()

	var names []string
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, hdr.Name)
	}

	sort.Strings(names)
	return names
}

func TestCreateBinaryTarball(t *testing.T) {
	binPath, shareDir := setupTestPackage(t)
	outputDir := t.TempDir()

	path, err := CreateTarball(TarballOptions{
		Name:      "my-tool",
		Version:   "1.0.0",
		Platform:  "darwin-arm64",
		BinPath:   binPath,
		ShareDir:  shareDir,
		OutputDir: outputDir,
	})
	if err != nil {
		t.Fatalf("CreateTarball: %v", err)
	}

	expectedName := "my-tool-1.0.0-darwin-arm64.tar.gz"
	if filepath.Base(path) != expectedName {
		t.Errorf("tarball name = %q, want %q", filepath.Base(path), expectedName)
	}

	entries := tarEntries(t, path)
	wantBin := false
	wantPlugin := false
	for _, e := range entries {
		if e == "bin/my-tool" {
			wantBin = true
		}
		if e == "share/purse-first/my-tool/.claude-plugin/plugin.json" {
			wantPlugin = true
		}
	}

	if !wantBin {
		t.Errorf("tarball missing bin/my-tool, entries: %v", entries)
	}
	if !wantPlugin {
		t.Errorf("tarball missing share data, entries: %v", entries)
	}
}

func TestCreateSkillOnlyTarball(t *testing.T) {
	_, shareDir := setupTestPackage(t)
	outputDir := t.TempDir()

	path, err := CreateTarball(TarballOptions{
		Name:      "my-skills",
		Version:   "0.1.0",
		ShareDir:  shareDir,
		OutputDir: outputDir,
	})
	if err != nil {
		t.Fatalf("CreateTarball: %v", err)
	}

	expectedName := "my-skills-0.1.0.tar.gz"
	if filepath.Base(path) != expectedName {
		t.Errorf("tarball name = %q, want %q", filepath.Base(path), expectedName)
	}

	entries := tarEntries(t, path)
	for _, e := range entries {
		if e == "bin/my-skills" {
			t.Error("skill-only tarball should not contain binaries")
		}
	}
}

func TestCreateMarketplaceTarball(t *testing.T) {
	outputDir := t.TempDir()
	marketplaceJSON := []byte(`{"name":"test"}`)

	path, err := CreateMarketplaceTarball(MarketplaceTarballOptions{
		Name:            "my-marketplace",
		Version:         "1.0.0",
		MarketplaceJSON: marketplaceJSON,
		OutputDir:       outputDir,
	})
	if err != nil {
		t.Fatalf("CreateMarketplaceTarball: %v", err)
	}

	entries := tarEntries(t, path)
	found := false
	for _, e := range entries {
		if e == ".claude-plugin/marketplace.json" {
			found = true
		}
	}
	if !found {
		t.Errorf("marketplace tarball missing .claude-plugin/marketplace.json, entries: %v", entries)
	}

	expectedName := "my-marketplace-1.0.0.tar.gz"
	if filepath.Base(path) != expectedName {
		t.Errorf("tarball name = %q, want %q", filepath.Base(path), expectedName)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test ./internal/packagebrew/... -v`
Expected: FAIL — `CreateTarball` and `CreateMarketplaceTarball` undefined.

**Step 3: Write tarball creation**

Create `internal/packagebrew/tarball.go`:

```go
package packagebrew

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type TarballOptions struct {
	Name      string
	Version   string
	Platform  string // empty for skill-only packages
	BinPath   string // empty for skill-only packages
	ShareDir  string
	OutputDir string
}

type MarketplaceTarballOptions struct {
	Name            string
	Version         string
	MarketplaceJSON []byte
	OutputDir       string
}

func tarballFilename(name, version, platform string) string {
	if platform == "" {
		return fmt.Sprintf("%s-%s.tar.gz", name, version)
	}
	return fmt.Sprintf("%s-%s-%s.tar.gz", name, version, platform)
}

func CreateTarball(opts TarballOptions) (string, error) {
	filename := tarballFilename(opts.Name, opts.Version, opts.Platform)
	outPath := filepath.Join(opts.OutputDir, filename)

	f, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("creating tarball: %w", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Add binary if present.
	if opts.BinPath != "" {
		if err := addFileToTar(tw, opts.BinPath, filepath.Join("bin", opts.Name)); err != nil {
			return "", fmt.Errorf("adding binary: %w", err)
		}
	}

	// Add share directory.
	if opts.ShareDir != "" {
		shareParent := filepath.Dir(filepath.Dir(filepath.Dir(opts.ShareDir)))
		if err := addDirToTar(tw, opts.ShareDir, shareParent); err != nil {
			return "", fmt.Errorf("adding share dir: %w", err)
		}
	}

	return outPath, nil
}

func CreateMarketplaceTarball(opts MarketplaceTarballOptions) (string, error) {
	filename := tarballFilename(opts.Name, opts.Version, "")
	outPath := filepath.Join(opts.OutputDir, filename)

	f, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("creating marketplace tarball: %w", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	hdr := &tar.Header{
		Name: ".claude-plugin/marketplace.json",
		Mode: 0o644,
		Size: int64(len(opts.MarketplaceJSON)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return "", err
	}
	if _, err := tw.Write(opts.MarketplaceJSON); err != nil {
		return "", err
	}

	return outPath, nil
}

func addFileToTar(tw *tar.Writer, srcPath, tarPath string) error {
	info, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	hdr := &tar.Header{
		Name: tarPath,
		Mode: int64(info.Mode()),
		Size: info.Size(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(tw, f)
	return err
}

func addDirToTar(tw *tar.Writer, dirPath, baseDir string) error {
	return filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}

		// Normalize to forward slashes for tar.
		rel = strings.ReplaceAll(rel, string(filepath.Separator), "/")

		info, err := d.Info()
		if err != nil {
			return err
		}

		if d.IsDir() {
			hdr := &tar.Header{
				Name:     rel + "/",
				Mode:     int64(info.Mode()),
				Typeflag: tar.TypeDir,
			}
			return tw.WriteHeader(hdr)
		}

		return addFileToTar(tw, path, rel)
	})
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test ./internal/packagebrew/... -v`
Expected: PASS

**Step 5: Commit**

```
git add internal/packagebrew/tarball.go internal/packagebrew/tarball_test.go
git commit -m "feat(packagebrew): add tarball creation for packages and marketplace"
```

---

### Task 4: Add Formula Generation

**Files:**
- Create: `internal/packagebrew/formula.go`
- Create: `internal/packagebrew/formula_test.go`

**Step 1: Write the failing test**

Create `internal/packagebrew/formula_test.go`:

```go
package packagebrew

import (
	"strings"
	"testing"
)

func TestGenerateBinaryFormula(t *testing.T) {
	formula := GenerateFormula(FormulaOptions{
		Name:        "grit",
		Description: "Git operations via MCP",
		Version:     "1.0.0",
		Homepage:    "https://example.com",
		License:     "MIT",
		ReleaseRepo: "org/homebrew-tap",
		Binary:      true,
		Hashes: map[string]string{
			"darwin-arm64": "abc123",
			"linux-amd64":  "def456",
		},
		BrewDeps: []string{"gh"},
	})

	if !strings.Contains(formula, "class Grit < Formula") {
		t.Error("missing PascalCase class name")
	}
	if !strings.Contains(formula, `desc "Git operations via MCP"`) {
		t.Error("missing description")
	}
	if !strings.Contains(formula, `sha256 "abc123"`) {
		t.Error("missing darwin-arm64 hash")
	}
	if !strings.Contains(formula, `sha256 "def456"`) {
		t.Error("missing linux-amd64 hash")
	}
	if !strings.Contains(formula, `bin.install "grit"`) {
		t.Error("missing bin.install for binary package")
	}
	if !strings.Contains(formula, `depends_on "gh"`) {
		t.Error("missing dependency")
	}
	if !strings.Contains(formula, `system bin/"grit", "--help"`) {
		t.Error("missing binary test block")
	}
}

func TestGenerateSkillOnlyFormula(t *testing.T) {
	formula := GenerateFormula(FormulaOptions{
		Name:        "bob",
		Description: "Skills package",
		Version:     "0.1.0",
		License:     "MIT",
		ReleaseRepo: "org/homebrew-tap",
		Binary:      false,
		Hashes: map[string]string{
			"": "xyz789",
		},
	})

	if !strings.Contains(formula, "class Bob < Formula") {
		t.Error("missing class name")
	}
	if strings.Contains(formula, "bin.install") {
		t.Error("skill-only formula should not have bin.install")
	}
	if strings.Contains(formula, "on_macos") {
		t.Error("skill-only formula should not have platform blocks")
	}
	if !strings.Contains(formula, "assert_predicate") {
		t.Error("missing assert_predicate test for skill-only")
	}
}

func TestGenerateMetaFormula(t *testing.T) {
	formula := GenerateMetaFormula(MetaFormulaOptions{
		Name:        "my-marketplace",
		Description: "All packages",
		Version:     "1.0.0",
		License:     "MIT",
		ReleaseRepo: "org/homebrew-tap",
		Hash:        "meta123",
		Packages:    []string{"grit", "bob"},
		AutoInstall: true,
	})

	if !strings.Contains(formula, "class MyMarketplace < Formula") {
		t.Error("missing PascalCase class name")
	}
	if !strings.Contains(formula, `depends_on "grit"`) {
		t.Error("missing grit dependency")
	}
	if !strings.Contains(formula, `depends_on "bob"`) {
		t.Error("missing bob dependency")
	}
	if !strings.Contains(formula, `system "purse-first", "install"`) {
		t.Error("missing post_install auto-install")
	}
}

func TestGenerateMetaFormulaNoAutoInstall(t *testing.T) {
	formula := GenerateMetaFormula(MetaFormulaOptions{
		Name:        "my-marketplace",
		Description: "All packages",
		Version:     "1.0.0",
		License:     "MIT",
		ReleaseRepo: "org/homebrew-tap",
		Hash:        "meta123",
		Packages:    []string{"grit"},
		AutoInstall: false,
	})

	if strings.Contains(formula, "post_install") {
		t.Error("should not have post_install when AutoInstall is false")
	}
}

func TestToClassName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"grit", "Grit"},
		{"get-hubbed", "GetHubbed"},
		{"purse-first", "PurseFirst"},
		{"tap-dancer", "TapDancer"},
		{"purse-first-all", "PurseFirstAll"},
	}
	for _, tt := range tests {
		got := toClassName(tt.input)
		if got != tt.want {
			t.Errorf("toClassName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test ./internal/packagebrew/... -v`
Expected: FAIL — `GenerateFormula`, `GenerateMetaFormula`, `toClassName` undefined.

**Step 3: Write formula generation**

Create `internal/packagebrew/formula.go`:

```go
package packagebrew

import (
	"fmt"
	"strings"
	"unicode"
)

type FormulaOptions struct {
	Name        string
	Description string
	Version     string
	Homepage    string
	License     string
	ReleaseRepo string
	Binary      bool
	Hashes      map[string]string // platform -> sha256 (empty key for platform-independent)
	BrewDeps    []string
}

type MetaFormulaOptions struct {
	Name        string
	Description string
	Version     string
	License     string
	ReleaseRepo string
	Hash        string
	Packages    []string
	AutoInstall bool
}

// Platform mapping for Homebrew conditionals.
var platformBlocks = []struct {
	key     string
	os      string
	archFn  string
}{
	{"darwin-arm64", "macos", "arm?"},
	{"darwin-amd64", "macos", "intel?"},
	{"linux-arm64", "linux", "arm?"},
	{"linux-amd64", "linux", "intel?"},
}

func toClassName(name string) string {
	var b strings.Builder
	upper := true
	for _, r := range name {
		if r == '-' {
			upper = true
			continue
		}
		if upper {
			b.WriteRune(unicode.ToUpper(r))
			upper = false
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func GenerateFormula(opts FormulaOptions) string {
	var b strings.Builder

	homepage := opts.Homepage
	if homepage == "" {
		homepage = fmt.Sprintf("https://github.com/%s", opts.ReleaseRepo)
	}

	fmt.Fprintf(&b, "class %s < Formula\n", toClassName(opts.Name))
	fmt.Fprintf(&b, "  desc %q\n", opts.Description)
	fmt.Fprintf(&b, "  homepage %q\n", homepage)
	fmt.Fprintf(&b, "  version %q\n", opts.Version)
	fmt.Fprintf(&b, "  license %q\n", opts.License)
	b.WriteString("\n")

	if opts.Binary {
		// Group platforms by OS.
		type archEntry struct {
			archFn string
			hash   string
			url    string
		}
		osGroups := map[string][]archEntry{}
		for _, pb := range platformBlocks {
			hash, ok := opts.Hashes[pb.key]
			if !ok {
				continue
			}
			url := fmt.Sprintf(
				"https://github.com/%s/releases/download/v%s/%s-%s-%s.tar.gz",
				opts.ReleaseRepo, opts.Version, opts.Name, opts.Version, pb.key,
			)
			osGroups[pb.os] = append(osGroups[pb.os], archEntry{pb.archFn, hash, url})
		}

		for _, osName := range []string{"macos", "linux"} {
			entries, ok := osGroups[osName]
			if !ok {
				continue
			}
			fmt.Fprintf(&b, "  on_%s do\n", osName)
			for i, e := range entries {
				if i > 0 {
					b.WriteString("    else\n")
				}
				fmt.Fprintf(&b, "    if Hardware::CPU.%s\n", e.archFn)
				fmt.Fprintf(&b, "      url %q\n", e.url)
				fmt.Fprintf(&b, "      sha256 %q\n", e.hash)
			}
			b.WriteString("    end\n")
			b.WriteString("  end\n\n")
		}
	} else {
		// Skill-only: single URL.
		hash := opts.Hashes[""]
		url := fmt.Sprintf(
			"https://github.com/%s/releases/download/v%s/%s-%s.tar.gz",
			opts.ReleaseRepo, opts.Version, opts.Name, opts.Version,
		)
		fmt.Fprintf(&b, "  url %q\n", url)
		fmt.Fprintf(&b, "  sha256 %q\n", hash)
		b.WriteString("\n")
	}

	for _, dep := range opts.BrewDeps {
		fmt.Fprintf(&b, "  depends_on %q\n", dep)
	}
	if len(opts.BrewDeps) > 0 {
		b.WriteString("\n")
	}

	b.WriteString("  def install\n")
	if opts.Binary {
		fmt.Fprintf(&b, "    bin.install %q\n", opts.Name)
	}
	fmt.Fprintf(&b, "    (share/\"purse-first/%s\").install Dir[\"share/purse-first/%s/*\"]\n", opts.Name, opts.Name)
	b.WriteString("  end\n\n")

	b.WriteString("  test do\n")
	if opts.Binary {
		fmt.Fprintf(&b, "    system bin/%q, \"--help\"\n", opts.Name)
	} else {
		fmt.Fprintf(&b, "    assert_predicate share/\"purse-first/%s/.claude-plugin/plugin.json\", :exist?\n", opts.Name)
	}
	b.WriteString("  end\n")
	b.WriteString("end\n")

	return b.String()
}

func GenerateMetaFormula(opts MetaFormulaOptions) string {
	var b strings.Builder

	fmt.Fprintf(&b, "class %s < Formula\n", toClassName(opts.Name))
	fmt.Fprintf(&b, "  desc %q\n", opts.Description)
	fmt.Fprintf(&b, "  homepage \"https://github.com/%s\"\n", opts.ReleaseRepo)
	fmt.Fprintf(&b, "  version %q\n", opts.Version)
	fmt.Fprintf(&b, "  license %q\n", opts.License)

	url := fmt.Sprintf(
		"https://github.com/%s/releases/download/v%s/%s-%s.tar.gz",
		opts.ReleaseRepo, opts.Version, opts.Name, opts.Version,
	)
	fmt.Fprintf(&b, "  url %q\n", url)
	fmt.Fprintf(&b, "  sha256 %q\n", opts.Hash)
	b.WriteString("\n")

	for _, pkg := range opts.Packages {
		fmt.Fprintf(&b, "  depends_on %q\n", pkg)
	}
	b.WriteString("\n")

	b.WriteString("  def install\n")
	b.WriteString("    (prefix/\".claude-plugin\").install \".claude-plugin/marketplace.json\"\n")
	b.WriteString("  end\n\n")

	if opts.AutoInstall {
		b.WriteString("  def post_install\n")
		b.WriteString("    system \"purse-first\", \"install\"\n")
		b.WriteString("  end\n\n")
	}

	b.WriteString("  test do\n")
	b.WriteString("    assert_predicate prefix/\".claude-plugin/marketplace.json\", :exist?\n")
	b.WriteString("  end\n")
	b.WriteString("end\n")

	return b.String()
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test ./internal/packagebrew/... -v`
Expected: PASS

**Step 5: Commit**

```
git add internal/packagebrew/formula.go internal/packagebrew/formula_test.go
git commit -m "feat(packagebrew): add formula generation for binary, skill-only, and meta packages"
```

---

### Task 5: Add README Generation

**Files:**
- Create: `internal/packagebrew/readme.go`
- Create: `internal/packagebrew/readme_test.go`

**Step 1: Write the failing test**

Create `internal/packagebrew/readme_test.go`:

```go
package packagebrew

import (
	"strings"
	"testing"
)

func TestGenerateReadme(t *testing.T) {
	readme := GenerateReadme(ReadmeOptions{
		TapName:     "org/tap",
		Description: "My marketplace",
		Packages: []ReadmePackage{
			{Name: "grit", Description: "Git MCP"},
			{Name: "bob", Description: "Skills"},
		},
		MetaFormulaName: "my-marketplace",
	})

	if !strings.Contains(readme, "brew tap org/tap") {
		t.Error("missing tap instruction")
	}
	if !strings.Contains(readme, "grit") {
		t.Error("missing grit package")
	}
	if !strings.Contains(readme, "brew install my-marketplace") {
		t.Error("missing meta install instruction")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test ./internal/packagebrew/... -v -run TestGenerateReadme`
Expected: FAIL — `GenerateReadme` undefined.

**Step 3: Write README generation**

Create `internal/packagebrew/readme.go`:

```go
package packagebrew

import (
	"fmt"
	"strings"
)

type ReadmeOptions struct {
	TapName         string
	Description     string
	Packages        []ReadmePackage
	MetaFormulaName string
}

type ReadmePackage struct {
	Name        string
	Description string
}

func GenerateReadme(opts ReadmeOptions) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s Homebrew Tap\n\n", opts.TapName)
	if opts.Description != "" {
		fmt.Fprintf(&b, "%s\n\n", opts.Description)
	}

	b.WriteString("## Installation\n\n")
	fmt.Fprintf(&b, "```bash\nbrew tap %s\n```\n\n", opts.TapName)

	b.WriteString("## Available Packages\n\n")
	b.WriteString("| Package | Description |\n")
	b.WriteString("|---------|-------------|\n")
	for _, pkg := range opts.Packages {
		fmt.Fprintf(&b, "| `%s` | %s |\n", pkg.Name, pkg.Description)
	}
	fmt.Fprintf(&b, "| `%s` | Meta package — installs everything |\n\n", opts.MetaFormulaName)

	b.WriteString("## Install All\n\n")
	fmt.Fprintf(&b, "```bash\nbrew install %s\n```\n\n", opts.MetaFormulaName)

	b.WriteString("## Install Individual Packages\n\n")
	b.WriteString("```bash\n")
	for _, pkg := range opts.Packages {
		fmt.Fprintf(&b, "brew install %s\n", pkg.Name)
	}
	b.WriteString("```\n")

	return b.String()
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test ./internal/packagebrew/... -v -run TestGenerateReadme`
Expected: PASS

**Step 5: Commit**

```
git add internal/packagebrew/readme.go internal/packagebrew/readme_test.go
git commit -m "feat(packagebrew): add README generation"
```

---

### Task 6: Add Run Orchestrator

**Files:**
- Create: `internal/packagebrew/run.go`
- Create: `internal/packagebrew/run_test.go`

**Step 1: Write the failing test**

Create `internal/packagebrew/run_test.go`:

```go
package packagebrew

import (
	"os"
	"path/filepath"
	"testing"
)

func setupIntegrationTest(t *testing.T) (configPath, outputDir string) {
	t.Helper()
	dir := t.TempDir()
	outputDir = filepath.Join(dir, "output")

	// Create fake binary.
	binDir := filepath.Join(dir, "bins")
	os.MkdirAll(binDir, 0o755)
	binPath := filepath.Join(binDir, "my-tool")
	os.WriteFile(binPath, []byte("#!/bin/sh\necho hello"), 0o755)

	// Create share directories.
	shareBase := filepath.Join(dir, "shares")

	toolShare := filepath.Join(shareBase, "my-tool")
	toolPlugin := filepath.Join(toolShare, ".claude-plugin")
	os.MkdirAll(toolPlugin, 0o755)
	os.WriteFile(filepath.Join(toolPlugin, "plugin.json"), []byte(`{
		"name": "my-tool",
		"mcpServers": {
			"my-tool": {"type": "stdio", "command": "my-tool"}
		}
	}`), 0o644)

	skillShare := filepath.Join(shareBase, "my-skills")
	skillPlugin := filepath.Join(skillShare, ".claude-plugin")
	skillDir := filepath.Join(skillShare, "skills", "test-skill")
	os.MkdirAll(skillPlugin, 0o755)
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillPlugin, "plugin.json"), []byte(`{"name":"my-skills"}`), 0o644)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: test\n---\nSkill content"), 0o644)

	// Write config.
	configPath = filepath.Join(dir, "brew-config.json")
	config := []byte(fmt.Sprintf(`{
		"name": "test-marketplace",
		"description": "Test",
		"owner": {"name": "tester"},
		"releaseRepo": "org/tap",
		"tapName": "org/tap",
		"license": "MIT",
		"packages": {
			"my-tool": {
				"description": "A tool",
				"version": "1.0.0",
				"binary": true,
				"platforms": {"darwin-arm64": %q},
				"share": %q,
				"brewDeps": []
			},
			"my-skills": {
				"description": "Skills",
				"version": "0.1.0",
				"binary": false,
				"share": %q,
				"brewDeps": []
			}
		}
	}`, binPath, toolShare, skillShare))

	os.WriteFile(configPath, config, 0o644)
	return configPath, outputDir
}

func TestRun(t *testing.T) {
	configPath, outputDir := setupIntegrationTest(t)

	err := Run(RunOptions{
		ConfigPath:    configPath,
		OutputDir:     outputDir,
		AutoInstall:   true,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Check formulas exist.
	for _, name := range []string{"my-tool.rb", "my-skills.rb", "test-marketplace.rb"} {
		path := filepath.Join(outputDir, "Formula", name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("missing formula: %s", name)
		}
	}

	// Check tarballs exist.
	toolTarball := filepath.Join(outputDir, "tarballs", "my-tool-1.0.0-darwin-arm64.tar.gz")
	if _, err := os.Stat(toolTarball); err != nil {
		t.Error("missing binary tarball")
	}

	skillTarball := filepath.Join(outputDir, "tarballs", "my-skills-0.1.0.tar.gz")
	if _, err := os.Stat(skillTarball); err != nil {
		t.Error("missing skill tarball")
	}

	metaTarball := filepath.Join(outputDir, "tarballs", "test-marketplace-1.0.0.tar.gz")
	if _, err := os.Stat(metaTarball); err != nil {
		t.Error("missing meta tarball")
	}

	// Check marketplace.json exists.
	mpPath := filepath.Join(outputDir, ".claude-plugin", "marketplace.json")
	if _, err := os.Stat(mpPath); err != nil {
		t.Error("missing marketplace.json")
	}

	// Check README exists.
	readmePath := filepath.Join(outputDir, "README.md")
	if _, err := os.Stat(readmePath); err != nil {
		t.Error("missing README.md")
	}
}
```

Note: This test will also need `"fmt"` in the imports.

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test ./internal/packagebrew/... -v -run TestRun`
Expected: FAIL — `Run` and `RunOptions` undefined.

**Step 3: Write the orchestrator**

Create `internal/packagebrew/run.go`:

```go
package packagebrew

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/amarbel-llc/purse-first/internal/marketplace"
)

type RunOptions struct {
	ConfigPath  string
	OutputDir   string
	AutoInstall bool
}

func Run(opts RunOptions) error {
	cfg, err := ReadConfig(opts.ConfigPath)
	if err != nil {
		return err
	}

	formulaDir := filepath.Join(opts.OutputDir, "Formula")
	tarballDir := filepath.Join(opts.OutputDir, "tarballs")
	if err := os.MkdirAll(formulaDir, 0o755); err != nil {
		return fmt.Errorf("creating Formula dir: %w", err)
	}
	if err := os.MkdirAll(tarballDir, 0o755); err != nil {
		return fmt.Errorf("creating tarballs dir: %w", err)
	}

	// Determine a consistent version from the first package (or use "0.0.0").
	metaVersion := "0.0.0"

	// Sort package names for deterministic output.
	pkgNames := make([]string, 0, len(cfg.Packages))
	for name := range cfg.Packages {
		pkgNames = append(pkgNames, name)
	}
	sort.Strings(pkgNames)

	if len(pkgNames) > 0 {
		metaVersion = cfg.Packages[pkgNames[0]].Version
	}

	// 1. Create tarballs and collect hashes.
	type pkgHashes struct {
		hashes map[string]string // platform -> sha256
	}
	allHashes := make(map[string]pkgHashes)

	for _, name := range pkgNames {
		pkg := cfg.Packages[name]
		hashes := make(map[string]string)

		if pkg.Binary {
			for platform, binPath := range pkg.Platforms {
				tarPath, err := CreateTarball(TarballOptions{
					Name:      name,
					Version:   pkg.Version,
					Platform:  platform,
					BinPath:   binPath,
					ShareDir:  pkg.Share,
					OutputDir: tarballDir,
				})
				if err != nil {
					return fmt.Errorf("creating tarball for %s/%s: %w", name, platform, err)
				}

				hash, err := sha256File(tarPath)
				if err != nil {
					return fmt.Errorf("hashing tarball %s: %w", tarPath, err)
				}
				hashes[platform] = hash
			}
		} else {
			tarPath, err := CreateTarball(TarballOptions{
				Name:      name,
				Version:   pkg.Version,
				ShareDir:  pkg.Share,
				OutputDir: tarballDir,
			})
			if err != nil {
				return fmt.Errorf("creating tarball for %s: %w", name, err)
			}

			hash, err := sha256File(tarPath)
			if err != nil {
				return fmt.Errorf("hashing tarball %s: %w", tarPath, err)
			}
			hashes[""] = hash
		}

		allHashes[name] = pkgHashes{hashes: hashes}
	}

	// 2. Generate marketplace.json by discovering plugins from share dirs.
	mpConfig := marketplace.Config{
		Name:        cfg.Name,
		Description: cfg.Description,
		Repo:        cfg.ReleaseRepo,
		Owner: marketplace.Owner{
			Name:  cfg.Owner.Name,
			Email: cfg.Owner.Email,
		},
		Plugins: make(map[string]marketplace.PluginMeta),
	}

	var discovered []marketplace.DiscoveredPlugin
	for _, name := range pkgNames {
		pkg := cfg.Packages[name]

		mpConfig.Plugins[name] = marketplace.PluginMeta{
			Description: pkg.Description,
			Version:     pkg.Version,
			Homepage:    pkg.Homepage,
			Category:    pkg.Category,
			Tags:        pkg.Tags,
		}

		// Discover MCP servers and skills from the share directory.
		sharePkgs, err := marketplace.DiscoverPlugins(filepath.Dir(pkg.Share))
		if err != nil {
			return fmt.Errorf("discovering plugins in %s: %w", pkg.Share, err)
		}

		for _, dp := range sharePkgs {
			if dp.Name == name {
				discovered = append(discovered, dp)
				break
			}
		}
	}

	mp := marketplace.Generate(mpConfig, discovered)
	mpJSON, err := json.MarshalIndent(mp, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling marketplace.json: %w", err)
	}
	mpJSON = append(mpJSON, '\n')

	mpDir := filepath.Join(opts.OutputDir, ".claude-plugin")
	if err := os.MkdirAll(mpDir, 0o755); err != nil {
		return fmt.Errorf("creating .claude-plugin dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(mpDir, "marketplace.json"), mpJSON, 0o644); err != nil {
		return fmt.Errorf("writing marketplace.json: %w", err)
	}

	// 3. Create meta-formula tarball.
	metaTarPath, err := CreateMarketplaceTarball(MarketplaceTarballOptions{
		Name:            cfg.Name,
		Version:         metaVersion,
		MarketplaceJSON: mpJSON,
		OutputDir:       tarballDir,
	})
	if err != nil {
		return fmt.Errorf("creating meta tarball: %w", err)
	}
	metaHash, err := sha256File(metaTarPath)
	if err != nil {
		return fmt.Errorf("hashing meta tarball: %w", err)
	}

	// 4. Generate per-package formulas.
	for _, name := range pkgNames {
		pkg := cfg.Packages[name]

		formula := GenerateFormula(FormulaOptions{
			Name:        name,
			Description: pkg.Description,
			Version:     pkg.Version,
			Homepage:    pkg.Homepage,
			License:     cfg.License,
			ReleaseRepo: cfg.ReleaseRepo,
			Binary:      pkg.Binary,
			Hashes:      allHashes[name].hashes,
			BrewDeps:    pkg.BrewDeps,
		})

		formulaPath := filepath.Join(formulaDir, name+".rb")
		if err := os.WriteFile(formulaPath, []byte(formula), 0o644); err != nil {
			return fmt.Errorf("writing formula %s: %w", name, err)
		}
	}

	// 5. Generate meta-formula.
	metaFormula := GenerateMetaFormula(MetaFormulaOptions{
		Name:        cfg.Name,
		Description: cfg.Description,
		Version:     metaVersion,
		License:     cfg.License,
		ReleaseRepo: cfg.ReleaseRepo,
		Hash:        metaHash,
		Packages:    pkgNames,
		AutoInstall: opts.AutoInstall,
	})
	metaFormulaPath := filepath.Join(formulaDir, cfg.Name+".rb")
	if err := os.WriteFile(metaFormulaPath, []byte(metaFormula), 0o644); err != nil {
		return fmt.Errorf("writing meta formula: %w", err)
	}

	// 6. Generate README.
	var readmePkgs []ReadmePackage
	for _, name := range pkgNames {
		readmePkgs = append(readmePkgs, ReadmePackage{
			Name:        name,
			Description: cfg.Packages[name].Description,
		})
	}

	readme := GenerateReadme(ReadmeOptions{
		TapName:         cfg.TapName,
		Description:     cfg.Description,
		Packages:        readmePkgs,
		MetaFormulaName: cfg.Name,
	})
	if err := os.WriteFile(filepath.Join(opts.OutputDir, "README.md"), []byte(readme), 0o644); err != nil {
		return fmt.Errorf("writing README: %w", err)
	}

	return nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test ./internal/packagebrew/... -v -run TestRun`
Expected: PASS

**Step 5: Commit**

```
git add internal/packagebrew/run.go internal/packagebrew/run_test.go
git commit -m "feat(packagebrew): add run orchestrator"
```

---

### Task 7: Wire Up Cobra Command

**Files:**
- Modify: `cmd/purse-first/main.go`

**Step 1: Add the `package brew` command to main.go**

Add import for `"github.com/amarbel-llc/purse-first/internal/packagebrew"` to the imports block.

Add the following before the `root.AddCommand(...)` line (line 218):

```go
	packageCmd := &cobra.Command{
		Use:   "package",
		Short: "Package commands for distribution",
	}

	var (
		brewConfigPath  string
		brewOutputDir   string
		brewNoAutoInstall bool
	)

	brewCmd := &cobra.Command{
		Use:           "brew",
		Short:         "Generate a Homebrew tap from pre-built packages",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			output := brewOutputDir
			if output == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("getting working directory: %w", err)
				}
				output = cwd
			}

			return packagebrew.Run(packagebrew.RunOptions{
				ConfigPath:  brewConfigPath,
				OutputDir:   output,
				AutoInstall: !brewNoAutoInstall,
			})
		},
	}

	brewCmd.Flags().StringVar(&brewConfigPath, "config", "", "path to brew-config.json")
	brewCmd.Flags().StringVar(&brewOutputDir, "output", "", "output directory (defaults to cwd)")
	brewCmd.Flags().BoolVar(&brewNoAutoInstall, "no-auto-install", false, "omit purse-first install from meta-formula post_install")
	brewCmd.MarkFlagRequired("config")

	packageCmd.AddCommand(brewCmd)
```

Update the `root.AddCommand(...)` to include `packageCmd`:

```go
root.AddCommand(installCmd, genMarketplaceCmd, installLocalCmd, installDevMCPCmd, genPluginCmd, validateCmd, packageCmd)
```

**Step 2: Build and verify**

Run: `nix develop --command go build -o /dev/null ./cmd/purse-first/`
Expected: Builds successfully.

Run: `nix develop --command go run ./cmd/purse-first/ package brew --help`
Expected: Shows help for `package brew` with `--config`, `--output`, `--no-auto-install` flags.

**Step 3: Commit**

```
git add cmd/purse-first/main.go
git commit -m "feat(purse-first): add package brew command"
```

---

### Task 8: Add BATS Integration Test

**Files:**
- Create: `zz-tests_bats/package_brew.bats`
- Modify: `justfile` (add test target)

**Step 1: Create BATS test**

Create `zz-tests_bats/package_brew.bats`:

```bash
#!/usr/bin/env bats

setup() {
  load "common.bash"
  OUTPUT_DIR="${BATS_TEST_TMPDIR}/brew-output"

  # Create fake binary.
  BIN_DIR="${BATS_TEST_TMPDIR}/bins"
  mkdir -p "$BIN_DIR"
  printf '#!/bin/sh\necho hello' > "$BIN_DIR/my-tool"
  chmod +x "$BIN_DIR/my-tool"

  # Create share directories.
  SHARE_BASE="${BATS_TEST_TMPDIR}/shares"

  TOOL_SHARE="$SHARE_BASE/my-tool"
  mkdir -p "$TOOL_SHARE/.claude-plugin"
  cat > "$TOOL_SHARE/.claude-plugin/plugin.json" <<'EOF'
{"name":"my-tool","mcpServers":{"my-tool":{"type":"stdio","command":"my-tool"}}}
EOF

  SKILL_SHARE="$SHARE_BASE/my-skills"
  mkdir -p "$SKILL_SHARE/.claude-plugin"
  mkdir -p "$SKILL_SHARE/skills/test-skill"
  echo '{"name":"my-skills"}' > "$SKILL_SHARE/.claude-plugin/plugin.json"
  printf -- '---\nname: test\n---\nContent' > "$SKILL_SHARE/skills/test-skill/SKILL.md"

  # Write config.
  CONFIG_PATH="${BATS_TEST_TMPDIR}/brew-config.json"
  cat > "$CONFIG_PATH" <<CONF
{
  "name": "test-marketplace",
  "description": "Test marketplace",
  "owner": {"name": "tester"},
  "releaseRepo": "org/tap",
  "tapName": "org/tap",
  "license": "MIT",
  "packages": {
    "my-tool": {
      "description": "A tool",
      "version": "1.0.0",
      "binary": true,
      "platforms": {"darwin-arm64": "$BIN_DIR/my-tool"},
      "share": "$TOOL_SHARE",
      "brewDeps": ["gh"]
    },
    "my-skills": {
      "description": "Skills package",
      "version": "0.1.0",
      "binary": false,
      "share": "$SKILL_SHARE",
      "brewDeps": []
    }
  }
}
CONF
}

@test "package brew generates Formula directory" {
  run purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  [[ "$status" -eq 0 ]]
  [[ -d "$OUTPUT_DIR/Formula" ]]
}

@test "package brew generates per-package formulas" {
  purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  [[ -f "$OUTPUT_DIR/Formula/my-tool.rb" ]]
  [[ -f "$OUTPUT_DIR/Formula/my-skills.rb" ]]
}

@test "package brew generates meta formula" {
  purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  [[ -f "$OUTPUT_DIR/Formula/test-marketplace.rb" ]]
}

@test "package brew generates tarballs" {
  purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  [[ -f "$OUTPUT_DIR/tarballs/my-tool-1.0.0-darwin-arm64.tar.gz" ]]
  [[ -f "$OUTPUT_DIR/tarballs/my-skills-0.1.0.tar.gz" ]]
  [[ -f "$OUTPUT_DIR/tarballs/test-marketplace-1.0.0.tar.gz" ]]
}

@test "package brew generates marketplace.json" {
  purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  [[ -f "$OUTPUT_DIR/.claude-plugin/marketplace.json" ]]
}

@test "package brew generates README" {
  purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  [[ -f "$OUTPUT_DIR/README.md" ]]
}

@test "binary formula has bin.install" {
  purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  grep -q 'bin.install' "$OUTPUT_DIR/Formula/my-tool.rb"
}

@test "skill-only formula lacks bin.install" {
  purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  ! grep -q 'bin.install' "$OUTPUT_DIR/Formula/my-skills.rb"
}

@test "formula has real sha256 hashes" {
  purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  ! grep -q 'SHA256_PLACEHOLDER' "$OUTPUT_DIR/Formula/my-tool.rb"
  grep -qE 'sha256 "[a-f0-9]{64}"' "$OUTPUT_DIR/Formula/my-tool.rb"
}

@test "meta formula has depends_on for all packages" {
  purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  grep -q 'depends_on "my-tool"' "$OUTPUT_DIR/Formula/test-marketplace.rb"
  grep -q 'depends_on "my-skills"' "$OUTPUT_DIR/Formula/test-marketplace.rb"
}

@test "meta formula has post_install by default" {
  purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  grep -q 'system "purse-first", "install"' "$OUTPUT_DIR/Formula/test-marketplace.rb"
}

@test "meta formula omits post_install with --no-auto-install" {
  purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR" --no-auto-install
  ! grep -q 'post_install' "$OUTPUT_DIR/Formula/test-marketplace.rb"
}

@test "binary formula has brew dependency" {
  purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  grep -q 'depends_on "gh"' "$OUTPUT_DIR/Formula/my-tool.rb"
}

@test "formula class names are PascalCase" {
  purse-first package brew --config "$CONFIG_PATH" --output "$OUTPUT_DIR"
  grep -q 'class MyTool < Formula' "$OUTPUT_DIR/Formula/my-tool.rb"
  grep -q 'class MySkills < Formula' "$OUTPUT_DIR/Formula/my-skills.rb"
  grep -q 'class TestMarketplace < Formula' "$OUTPUT_DIR/Formula/test-marketplace.rb"
}
```

**Step 2: Add test target to justfile**

Add after the existing test targets:

```just
test-package-brew: build-batman
    {{cmd_nix_dev}} {{cmd_batman_bats}} --jobs {{num_cpus()}} zz-tests_bats/package_brew.bats
```

**Step 3: Run the BATS test**

Run: `just test-package-brew`
Expected: All tests pass (requires `purse-first` binary to be on PATH via nix develop).

**Step 4: Commit**

```
git add zz-tests_bats/package_brew.bats justfile
git commit -m "test(packagebrew): add BATS integration tests for package brew command"
```
