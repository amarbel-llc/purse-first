# Filetype Config Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Restructure lux config into three layers (tool declarations + filetype routing) with formatter chaining, fallback, and LSP format control.

**Architecture:** New `internal/config/filetype` package owns filetype config parsing, validation, and loading. Routing fields removed from LSP and formatter structs. Both `server.Router` and `formatter.Router` are updated to build matchers from filetype configs. Executor gains chain and fallback modes. CLI gets `init` and updated `add`/`list` commands.

**Tech Stack:** Go, TOML (BurntSushi/toml), gobwas/glob (via pkg/filematch), Cobra CLI

---

### Task 1: Filetype Config Struct and Loading

**Files:**
- Create: `internal/config/filetype/filetype.go`
- Test: `internal/config/filetype/filetype_test.go`

**Step 1: Write the failing test**

```go
package filetype

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFiletype(t *testing.T) {
	dir := t.TempDir()
	content := `
extensions = ["go"]
patterns = ["go.mod", "go.sum"]
language_ids = ["go"]
lsp = "gopls"
formatters = ["golines"]
formatter_mode = "chain"
lsp_format = "fallback"
`
	if err := os.WriteFile(filepath.Join(dir, "go.toml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	configs, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}

	cfg := configs[0]
	if cfg.Name != "go" {
		t.Errorf("name = %q, want %q", cfg.Name, "go")
	}
	if len(cfg.Extensions) != 1 || cfg.Extensions[0] != "go" {
		t.Errorf("extensions = %v, want [go]", cfg.Extensions)
	}
	if cfg.LSP != "gopls" {
		t.Errorf("lsp = %q, want %q", cfg.LSP, "gopls")
	}
	if len(cfg.Formatters) != 1 || cfg.Formatters[0] != "golines" {
		t.Errorf("formatters = %v, want [golines]", cfg.Formatters)
	}
	if cfg.FormatterMode != "chain" {
		t.Errorf("formatter_mode = %q, want %q", cfg.FormatterMode, "chain")
	}
	if cfg.LSPFormat != "fallback" {
		t.Errorf("lsp_format = %q, want %q", cfg.LSPFormat, "fallback")
	}
}

func TestLoadDir_Empty(t *testing.T) {
	dir := t.TempDir()
	configs, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if len(configs) != 0 {
		t.Fatalf("expected 0 configs, got %d", len(configs))
	}
}

func TestLoadDir_NonExistent(t *testing.T) {
	configs, err := LoadDir("/nonexistent/path")
	if err != nil {
		t.Fatalf("LoadDir should not error on missing dir: %v", err)
	}
	if len(configs) != 0 {
		t.Fatalf("expected 0 configs, got %d", len(configs))
	}
}

func TestLoadDir_MultipleFiles(t *testing.T) {
	dir := t.TempDir()

	goContent := `
extensions = ["go"]
lsp = "gopls"
`
	pyContent := `
extensions = ["py"]
lsp = "pyright"
formatters = ["isort", "black"]
`
	os.WriteFile(filepath.Join(dir, "go.toml"), []byte(goContent), 0644)
	os.WriteFile(filepath.Join(dir, "python.toml"), []byte(pyContent), 0644)

	configs, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	if len(configs) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(configs))
	}

	// Should be sorted alphabetically by filename
	if configs[0].Name != "go" {
		t.Errorf("first config name = %q, want %q", configs[0].Name, "go")
	}
	if configs[1].Name != "python" {
		t.Errorf("second config name = %q, want %q", configs[1].Name, "python")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test -v -run TestLoad ./internal/config/filetype/`
Expected: FAIL — package does not exist

**Step 3: Write minimal implementation**

```go
package filetype

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Name          string   `toml:"-"`
	Extensions    []string `toml:"extensions"`
	Patterns      []string `toml:"patterns"`
	LanguageIDs   []string `toml:"language_ids"`
	LSP           string   `toml:"lsp"`
	Formatters    []string `toml:"formatters"`
	FormatterMode string   `toml:"formatter_mode"`
	LSPFormat     string   `toml:"lsp_format"`
}

func LoadDir(dir string) ([]*Config, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading filetype dir %s: %w", dir, err)
	}

	var configs []*Config
	var names []string

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		names = append(names, entry.Name())
	}

	sort.Strings(names)

	for _, name := range names {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}

		var cfg Config
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}

		cfg.Name = strings.TrimSuffix(name, ".toml")
		configs = append(configs, &cfg)
	}

	return configs, nil
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test -v -run TestLoad ./internal/config/filetype/`
Expected: PASS

**Step 5: Commit**

```
git add internal/config/filetype/filetype.go internal/config/filetype/filetype_test.go
git commit -m "feat: add filetype config struct and directory loading"
```

---

### Task 2: Filetype Config Validation

**Files:**
- Modify: `internal/config/filetype/filetype.go`
- Test: `internal/config/filetype/filetype_test.go`

**Step 1: Write the failing test**

```go
func TestValidate_Valid(t *testing.T) {
	lsps := map[string]bool{"gopls": true, "pyright": true}
	fmts := map[string]bool{"golines": true, "isort": true, "black": true}

	configs := []*Config{
		{Name: "go", Extensions: []string{"go"}, LSP: "gopls", Formatters: []string{"golines"}},
		{Name: "python", Extensions: []string{"py"}, LSP: "pyright", Formatters: []string{"isort", "black"}, FormatterMode: "chain"},
	}

	if err := Validate(configs, lsps, fmts); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidate_NoMatchingFields(t *testing.T) {
	configs := []*Config{{Name: "bad", LSP: "gopls"}}
	err := Validate(configs, map[string]bool{"gopls": true}, nil)
	if err == nil {
		t.Fatal("expected error for missing extensions/patterns/language_ids")
	}
}

func TestValidate_UnknownLSP(t *testing.T) {
	configs := []*Config{{Name: "go", Extensions: []string{"go"}, LSP: "unknown"}}
	err := Validate(configs, map[string]bool{"gopls": true}, nil)
	if err == nil {
		t.Fatal("expected error for unknown LSP")
	}
}

func TestValidate_UnknownFormatter(t *testing.T) {
	configs := []*Config{{Name: "go", Extensions: []string{"go"}, LSP: "gopls", Formatters: []string{"unknown"}}}
	err := Validate(configs, map[string]bool{"gopls": true}, map[string]bool{"golines": true})
	if err == nil {
		t.Fatal("expected error for unknown formatter")
	}
}

func TestValidate_DuplicateExtension(t *testing.T) {
	configs := []*Config{
		{Name: "go", Extensions: []string{"go"}, LSP: "gopls"},
		{Name: "golang", Extensions: []string{"go"}, LSP: "gopls"},
	}
	err := Validate(configs, map[string]bool{"gopls": true}, nil)
	if err == nil {
		t.Fatal("expected error for duplicate extension")
	}
}

func TestValidate_InvalidFormatterMode(t *testing.T) {
	configs := []*Config{{Name: "go", Extensions: []string{"go"}, LSP: "gopls", FormatterMode: "invalid"}}
	err := Validate(configs, map[string]bool{"gopls": true}, nil)
	if err == nil {
		t.Fatal("expected error for invalid formatter_mode")
	}
}

func TestValidate_InvalidLSPFormat(t *testing.T) {
	configs := []*Config{{Name: "go", Extensions: []string{"go"}, LSP: "gopls", LSPFormat: "invalid"}}
	err := Validate(configs, map[string]bool{"gopls": true}, nil)
	if err == nil {
		t.Fatal("expected error for invalid lsp_format")
	}
}

func TestValidate_EmptyLSP(t *testing.T) {
	configs := []*Config{{Name: "go", Extensions: []string{"go"}, Formatters: []string{"golines"}}}
	err := Validate(configs, nil, map[string]bool{"golines": true})
	if err != nil {
		t.Errorf("LSP should be optional, got: %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test -v -run TestValidate ./internal/config/filetype/`
Expected: FAIL — `Validate` undefined

**Step 3: Write minimal implementation**

```go
func Validate(configs []*Config, lsps, formatters map[string]bool) error {
	seenExts := make(map[string]string)
	seenLangs := make(map[string]string)

	for _, cfg := range configs {
		if len(cfg.Extensions) == 0 && len(cfg.Patterns) == 0 && len(cfg.LanguageIDs) == 0 {
			return fmt.Errorf("filetype/%s.toml: at least one of extensions, patterns, or language_ids is required", cfg.Name)
		}

		if cfg.LSP != "" && !lsps[cfg.LSP] {
			return fmt.Errorf("filetype/%s.toml: lsp %q not found in lsps.toml", cfg.Name, cfg.LSP)
		}

		for _, f := range cfg.Formatters {
			if !formatters[f] {
				return fmt.Errorf("filetype/%s.toml: formatter %q not found in formatters.toml", cfg.Name, f)
			}
		}

		if cfg.FormatterMode != "" && cfg.FormatterMode != "chain" && cfg.FormatterMode != "fallback" {
			return fmt.Errorf("filetype/%s.toml: invalid formatter_mode %q (must be \"chain\" or \"fallback\")", cfg.Name, cfg.FormatterMode)
		}

		if cfg.LSPFormat != "" && cfg.LSPFormat != "never" && cfg.LSPFormat != "fallback" && cfg.LSPFormat != "prefer" {
			return fmt.Errorf("filetype/%s.toml: invalid lsp_format %q (must be \"never\", \"fallback\", or \"prefer\")", cfg.Name, cfg.LSPFormat)
		}

		for _, ext := range cfg.Extensions {
			if other, ok := seenExts[ext]; ok {
				return fmt.Errorf("filetype/%s.toml: extension %q also claimed by filetype/%s.toml", cfg.Name, ext, other)
			}
			seenExts[ext] = cfg.Name
		}

		for _, lang := range cfg.LanguageIDs {
			if other, ok := seenLangs[lang]; ok {
				return fmt.Errorf("filetype/%s.toml: language_id %q also claimed by filetype/%s.toml", cfg.Name, lang, other)
			}
			seenLangs[lang] = cfg.Name
		}
	}

	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test -v -run TestValidate ./internal/config/filetype/`
Expected: PASS

**Step 5: Commit**

```
git add internal/config/filetype/filetype.go internal/config/filetype/filetype_test.go
git commit -m "feat: add filetype config validation"
```

---

### Task 3: Filetype Config Merging

**Files:**
- Modify: `internal/config/filetype/filetype.go`
- Test: `internal/config/filetype/filetype_test.go`

**Step 1: Write the failing test**

```go
func TestMerge_ProjectReplacesGlobal(t *testing.T) {
	global := []*Config{
		{Name: "go", Extensions: []string{"go"}, LSP: "gopls", Formatters: []string{"golines"}},
		{Name: "python", Extensions: []string{"py"}, LSP: "pyright"},
	}
	project := []*Config{
		{Name: "go", Extensions: []string{"go"}, LSP: "gopls", Formatters: []string{"gofumpt"}, FormatterMode: "chain"},
	}

	merged := Merge(global, project)

	if len(merged) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(merged))
	}

	// go.toml should be fully replaced by project version
	goConfig := findByName(merged, "go")
	if goConfig == nil {
		t.Fatal("missing go config")
	}
	if len(goConfig.Formatters) != 1 || goConfig.Formatters[0] != "gofumpt" {
		t.Errorf("formatters = %v, want [gofumpt]", goConfig.Formatters)
	}
	if goConfig.FormatterMode != "chain" {
		t.Errorf("formatter_mode = %q, want %q", goConfig.FormatterMode, "chain")
	}

	// python.toml should be preserved from global
	pyConfig := findByName(merged, "python")
	if pyConfig == nil {
		t.Fatal("missing python config")
	}
}

func TestMerge_ProjectAddsNew(t *testing.T) {
	global := []*Config{
		{Name: "go", Extensions: []string{"go"}, LSP: "gopls"},
	}
	project := []*Config{
		{Name: "rust", Extensions: []string{"rs"}, LSP: "rust-analyzer"},
	}

	merged := Merge(global, project)

	if len(merged) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(merged))
	}
}

func findByName(configs []*Config, name string) *Config {
	for _, c := range configs {
		if c.Name == name {
			return c
		}
	}
	return nil
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test -v -run TestMerge ./internal/config/filetype/`
Expected: FAIL — `Merge` undefined

**Step 3: Write minimal implementation**

```go
func Merge(global, project []*Config) []*Config {
	projectByName := make(map[string]*Config)
	for _, p := range project {
		projectByName[p.Name] = p
	}

	var merged []*Config
	seen := make(map[string]bool)

	for _, g := range global {
		if p, ok := projectByName[g.Name]; ok {
			merged = append(merged, p)
		} else {
			merged = append(merged, g)
		}
		seen[g.Name] = true
	}

	for _, p := range project {
		if !seen[p.Name] {
			merged = append(merged, p)
		}
	}

	return merged
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test -v -run TestMerge ./internal/config/filetype/`
Expected: PASS

**Step 5: Commit**

```
git add internal/config/filetype/filetype.go internal/config/filetype/filetype_test.go
git commit -m "feat: add filetype config merging (project replaces global by name)"
```

---

### Task 4: Filetype Config Path Helpers and Full Loader

**Files:**
- Modify: `internal/config/filetype/filetype.go`
- Test: `internal/config/filetype/filetype_test.go`

**Step 1: Write the failing test**

```go
func TestEffectiveFormatterMode(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want string
	}{
		{"empty defaults to chain", "", "chain"},
		{"explicit chain", "chain", "chain"},
		{"explicit fallback", "fallback", "fallback"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{FormatterMode: tt.mode}
			if got := cfg.EffectiveFormatterMode(); got != tt.want {
				t.Errorf("EffectiveFormatterMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEffectiveLSPFormat(t *testing.T) {
	tests := []struct {
		name       string
		lspFormat  string
		formatters []string
		want       string
	}{
		{"explicit never", "never", []string{"fmt"}, "never"},
		{"explicit fallback", "fallback", []string{"fmt"}, "fallback"},
		{"explicit prefer", "prefer", nil, "prefer"},
		{"empty with formatters defaults to never", "", []string{"fmt"}, "never"},
		{"empty without formatters defaults to prefer", "", nil, "prefer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{LSPFormat: tt.lspFormat, Formatters: tt.formatters}
			if got := cfg.EffectiveLSPFormat(); got != tt.want {
				t.Errorf("EffectiveLSPFormat() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test -v -run TestEffective ./internal/config/filetype/`
Expected: FAIL — methods undefined

**Step 3: Write minimal implementation**

Add to `internal/config/filetype/filetype.go`:

```go
func GlobalDir() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "lux", "filetype")
}

func LocalDir() string {
	return filepath.Join(".lux", "filetype")
}

func (c *Config) EffectiveFormatterMode() string {
	if c.FormatterMode == "" {
		return "chain"
	}
	return c.FormatterMode
}

func (c *Config) EffectiveLSPFormat() string {
	if c.LSPFormat != "" {
		return c.LSPFormat
	}
	if len(c.Formatters) > 0 {
		return "never"
	}
	return "prefer"
}

func Load() ([]*Config, error) {
	return LoadDir(GlobalDir())
}

func LoadLocal() ([]*Config, error) {
	return LoadDir(LocalDir())
}

func LoadMerged() ([]*Config, error) {
	global, err := Load()
	if err != nil {
		return nil, fmt.Errorf("loading global filetypes: %w", err)
	}

	local, err := LoadLocal()
	if err != nil {
		return nil, fmt.Errorf("loading local filetypes: %w", err)
	}

	return Merge(global, local), nil
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test -v -run TestEffective ./internal/config/filetype/`
Expected: PASS

**Step 5: Commit**

```
git add internal/config/filetype/filetype.go internal/config/filetype/filetype_test.go
git commit -m "feat: add filetype config helpers and full loader"
```

---

### Task 5: Remove Routing Fields from LSP and Formatter Structs

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/config/formatter.go`
- Modify: `internal/config/formatter_test.go`

**Step 1: Remove fields from LSP struct**

In `internal/config/config.go`, remove from the `LSP` struct (lines 23-25):

```go
// Remove these three lines:
Extensions   []string            `toml:"extensions"`
Patterns     []string            `toml:"patterns"`
LanguageIDs  []string            `toml:"language_ids"`
```

Also remove the validation check that requires at least one of these fields
(find the check in `Validate()` and remove it).

**Step 2: Remove fields from Formatter struct**

In `internal/config/formatter.go`, remove from the `Formatter` struct (lines 27-28):

```go
// Remove these two lines:
Extensions []string          `toml:"extensions"`
Patterns   []string          `toml:"patterns"`
```

Also remove the validation check that requires at least one of extensions or
patterns (find in `Validate()`).

**Step 3: Fix compilation errors**

Run: `nix develop --command go build ./...`

This will surface every file that references the removed fields. Fix each:

- `internal/server/router.go`: `NewRouter` reads `l.Extensions`, `l.Patterns`,
  `l.LanguageIDs` — this will be rewritten in Task 6
- `internal/formatter/router.go`: `NewRouter` reads `f.Extensions`,
  `f.Patterns` — this will be rewritten in Task 7
- `internal/config/config_test.go`: Update tests that set these fields
- `internal/config/formatter_test.go`: Update tests that set these fields
- `internal/server/router_test.go`: Update tests — will be rewritten in Task 6
- `internal/formatter/router_test.go`: Update tests — will be rewritten in
  Task 7
- `cmd/lux/main.go`: The `list` command prints extensions/patterns — update
  in Task 10
- `internal/capabilities/bootstrap.go`: If it sets extensions during `lux add`
  — update

For now, stub out the router constructors to accept filetype configs (empty
implementation) so the code compiles. The full rewrite happens in Tasks 6-7.

**Step 4: Run tests**

Run: `nix develop --command go test ./internal/config/...`
Expected: PASS (tests updated to not set removed fields)

**Step 5: Commit**

```
git add internal/config/config.go internal/config/config_test.go \
       internal/config/formatter.go internal/config/formatter_test.go \
       internal/server/router.go internal/formatter/router.go
git commit -m "refactor: remove routing fields from LSP and formatter structs"
```

---

### Task 6: Rewrite Server Router to Use Filetype Configs

**Files:**
- Modify: `internal/server/router.go`
- Modify: `internal/server/router_test.go`

**Step 1: Write the failing test**

```go
package server

import (
	"testing"

	"github.com/amarbel-llc/lux/internal/config/filetype"
)

func TestRouter_RouteByURI_Extension(t *testing.T) {
	filetypes := []*filetype.Config{
		{Name: "go", Extensions: []string{"go"}, LSP: "gopls"},
		{Name: "python", Extensions: []string{"py"}, LSP: "pyright"},
	}

	router, err := NewRouter(filetypes)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		uri     string
		wantLSP string
	}{
		{"go file", "file:///src/main.go", "gopls"},
		{"python file", "file:///src/main.py", "pyright"},
		{"unknown file", "file:///src/readme.md", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := router.RouteByURI(lsp.DocumentURI(tt.uri))
			if got != tt.wantLSP {
				t.Errorf("RouteByURI(%q) = %q, want %q", tt.uri, got, tt.wantLSP)
			}
		})
	}
}

func TestRouter_FiletypeByURI(t *testing.T) {
	filetypes := []*filetype.Config{
		{Name: "go", Extensions: []string{"go"}, LSP: "gopls", Formatters: []string{"golines"}},
	}

	router, err := NewRouter(filetypes)
	if err != nil {
		t.Fatal(err)
	}

	ft := router.FiletypeByURI(lsp.DocumentURI("file:///src/main.go"))
	if ft == nil {
		t.Fatal("expected filetype config for .go file")
	}
	if ft.LSP != "gopls" {
		t.Errorf("lsp = %q, want %q", ft.LSP, "gopls")
	}
	if len(ft.Formatters) != 1 || ft.Formatters[0] != "golines" {
		t.Errorf("formatters = %v, want [golines]", ft.Formatters)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test -v -run TestRouter ./internal/server/`
Expected: FAIL — signature mismatch

**Step 3: Rewrite router**

```go
package server

import (
	"encoding/json"
	"sync"

	"github.com/amarbel-llc/lux/internal/config/filetype"
	"github.com/amarbel-llc/lux/internal/lsp"
	"github.com/amarbel-llc/lux/pkg/filematch"
)

type Router struct {
	matchers    *filematch.MatcherSet
	filetypes   map[string]*filetype.Config
	languageMap map[lsp.DocumentURI]string
	mu          sync.RWMutex
}

func NewRouter(configs []*filetype.Config) (*Router, error) {
	matchers := filematch.NewMatcherSet()
	ftMap := make(map[string]*filetype.Config)

	for _, ft := range configs {
		if err := matchers.Add(ft.Name, ft.Extensions, ft.Patterns, ft.LanguageIDs); err != nil {
			return nil, err
		}
		ftMap[ft.Name] = ft
	}

	return &Router{
		matchers:    matchers,
		filetypes:   ftMap,
		languageMap: make(map[lsp.DocumentURI]string),
	}, nil
}

func (r *Router) Route(method string, params json.RawMessage) string {
	ft := r.routeFiletype(method, params)
	if ft == nil {
		return ""
	}
	return ft.LSP
}

func (r *Router) routeFiletype(method string, params json.RawMessage) *filetype.Config {
	var paramsMap map[string]any
	if err := json.Unmarshal(params, &paramsMap); err != nil {
		return nil
	}

	uri := lsp.ExtractURI(method, paramsMap)
	if uri == "" {
		return nil
	}

	if method == lsp.MethodTextDocumentDidOpen {
		langID := lsp.ExtractLanguageID(paramsMap)
		if langID != "" {
			r.mu.Lock()
			r.languageMap[uri] = langID
			r.mu.Unlock()
		}
	}

	if method == lsp.MethodTextDocumentDidClose {
		r.mu.Lock()
		delete(r.languageMap, uri)
		r.mu.Unlock()
	}

	return r.FiletypeByURI(uri)
}

func (r *Router) FiletypeByURI(uri lsp.DocumentURI) *filetype.Config {
	r.mu.RLock()
	langID := r.languageMap[uri]
	r.mu.RUnlock()

	path := uri.Path()
	ext := uri.Extension()

	name := r.matchers.Match(path, ext, langID)
	if name == "" {
		return nil
	}
	return r.filetypes[name]
}

func (r *Router) RouteByURI(uri lsp.DocumentURI) string {
	ft := r.FiletypeByURI(uri)
	if ft == nil {
		return ""
	}
	return ft.LSP
}

func (r *Router) RouteByExtension(ext string) string {
	name := r.matchers.MatchByExtension(ext)
	if name == "" {
		return ""
	}
	if ft, ok := r.filetypes[name]; ok {
		return ft.LSP
	}
	return ""
}

func (r *Router) RouteByLanguageID(langID string) string {
	name := r.matchers.MatchByLanguageID(langID)
	if name == "" {
		return ""
	}
	if ft, ok := r.filetypes[name]; ok {
		return ft.LSP
	}
	return ""
}

func (r *Router) SetLanguageID(uri lsp.DocumentURI, langID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.languageMap[uri] = langID
}

func (r *Router) GetLanguageID(uri lsp.DocumentURI) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.languageMap[uri]
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test -v -run TestRouter ./internal/server/`
Expected: PASS

**Step 5: Commit**

```
git add internal/server/router.go internal/server/router_test.go
git commit -m "refactor: rewrite server router to use filetype configs"
```

---

### Task 7: Rewrite Formatter Router to Use Filetype Configs

**Files:**
- Modify: `internal/formatter/router.go`
- Modify: `internal/formatter/router_test.go`

**Step 1: Write the failing test**

```go
package formatter

import (
	"testing"

	"github.com/amarbel-llc/lux/internal/config"
	"github.com/amarbel-llc/lux/internal/config/filetype"
)

func TestRouterMatch_Filetype(t *testing.T) {
	filetypes := []*filetype.Config{
		{Name: "go", Extensions: []string{"go"}, Formatters: []string{"golines"}},
		{Name: "python", Extensions: []string{"py"}, Formatters: []string{"isort", "black"}, FormatterMode: "chain"},
	}
	formatters := map[string]*config.Formatter{
		"golines": {Name: "golines", Flake: "nixpkgs#golines"},
		"isort":   {Name: "isort", Flake: "nixpkgs#isort"},
		"black":   {Name: "black", Flake: "nixpkgs#black"},
	}

	router, err := NewRouter(filetypes, formatters)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		filePath string
		wantFmt  int
		wantMode string
	}{
		{"go file", "/src/main.go", 1, "chain"},
		{"python file", "/src/main.py", 2, "chain"},
		{"unknown file", "/src/readme.md", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := router.Match(tt.filePath)
			if tt.wantFmt == 0 {
				if result != nil {
					t.Errorf("expected nil result for %s", tt.filePath)
				}
				return
			}
			if result == nil {
				t.Fatalf("expected result for %s", tt.filePath)
			}
			if len(result.Formatters) != tt.wantFmt {
				t.Errorf("formatters count = %d, want %d", len(result.Formatters), tt.wantFmt)
			}
			if result.Mode != tt.wantMode {
				t.Errorf("mode = %q, want %q", result.Mode, tt.wantMode)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test -v -run TestRouterMatch_Filetype ./internal/formatter/`
Expected: FAIL — signature mismatch

**Step 3: Rewrite formatter router**

```go
package formatter

import (
	"path/filepath"
	"strings"

	"github.com/amarbel-llc/lux/internal/config"
	"github.com/amarbel-llc/lux/internal/config/filetype"
	"github.com/amarbel-llc/lux/pkg/filematch"
)

type MatchResult struct {
	Formatters []*config.Formatter
	Mode       string
	LSPFormat  string
}

type Router struct {
	matchers   *filematch.MatcherSet
	filetypes  map[string]*filetype.Config
	formatters map[string]*config.Formatter
}

func NewRouter(filetypes []*filetype.Config, formatters map[string]*config.Formatter) (*Router, error) {
	matchers := filematch.NewMatcherSet()
	ftMap := make(map[string]*filetype.Config)

	for _, ft := range filetypes {
		if len(ft.Formatters) == 0 {
			continue
		}
		if err := matchers.Add(ft.Name, ft.Extensions, ft.Patterns, ft.LanguageIDs); err != nil {
			return nil, err
		}
		ftMap[ft.Name] = ft
	}

	return &Router{
		matchers:   matchers,
		filetypes:  ftMap,
		formatters: formatters,
	}, nil
}

func (r *Router) Match(filePath string) *MatchResult {
	ext := strings.ToLower(filepath.Ext(filePath))
	name := r.matchers.Match(filePath, ext, "")
	if name == "" {
		return nil
	}

	ft := r.filetypes[name]
	if ft == nil {
		return nil
	}

	var fmts []*config.Formatter
	for _, fmtName := range ft.Formatters {
		if f, ok := r.formatters[fmtName]; ok {
			fmts = append(fmts, f)
		}
	}

	if len(fmts) == 0 {
		return nil
	}

	return &MatchResult{
		Formatters: fmts,
		Mode:       ft.EffectiveFormatterMode(),
		LSPFormat:  ft.EffectiveLSPFormat(),
	}
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test -v -run TestRouterMatch_Filetype ./internal/formatter/`
Expected: PASS

**Step 5: Commit**

```
git add internal/formatter/router.go internal/formatter/router_test.go
git commit -m "refactor: rewrite formatter router to use filetype configs"
```

---

### Task 8: Add Chain and Fallback Execution to Formatter

**Files:**
- Modify: `internal/formatter/executor.go`
- Test: `internal/formatter/executor_test.go`

**Step 1: Write the failing test**

```go
func TestFormatChain(t *testing.T) {
	// Create two formatters that each add a prefix
	dir := t.TempDir()

	script1 := filepath.Join(dir, "fmt1")
	os.WriteFile(script1, []byte("#!/bin/sh\necho PREFIX1\ncat"), 0755)

	script2 := filepath.Join(dir, "fmt2")
	os.WriteFile(script2, []byte("#!/bin/sh\necho PREFIX2\ncat"), 0755)

	f1 := &config.Formatter{Name: "fmt1", Path: script1, Mode: "stdin"}
	f2 := &config.Formatter{Name: "fmt2", Path: script2, Mode: "stdin"}

	result, err := FormatChain(context.Background(), []*config.Formatter{f1, f2}, "/tmp/test.txt", []byte("hello"), nil)
	if err != nil {
		t.Fatalf("FormatChain: %v", err)
	}

	if !strings.Contains(result.Formatted, "PREFIX1") {
		t.Error("expected PREFIX1 in output")
	}
	if !strings.Contains(result.Formatted, "PREFIX2") {
		t.Error("expected PREFIX2 in output")
	}
	if !result.Changed {
		t.Error("expected Changed = true")
	}
}

func TestFormatFallback_FirstSucceeds(t *testing.T) {
	dir := t.TempDir()

	script := filepath.Join(dir, "fmt1")
	os.WriteFile(script, []byte("#!/bin/sh\necho formatted"), 0755)

	f1 := &config.Formatter{Name: "fmt1", Path: script, Mode: "stdin"}
	f2 := &config.Formatter{Name: "fmt2", Path: "/nonexistent/binary", Mode: "stdin"}

	result, err := FormatFallback(context.Background(), []*config.Formatter{f1, f2}, "/tmp/test.txt", []byte("hello"), nil)
	if err != nil {
		t.Fatalf("FormatFallback: %v", err)
	}
	if result.Formatted != "formatted\n" {
		t.Errorf("formatted = %q, want %q", result.Formatted, "formatted\n")
	}
}

func TestFormatFallback_FirstFailsSecondSucceeds(t *testing.T) {
	dir := t.TempDir()

	script := filepath.Join(dir, "fmt2")
	os.WriteFile(script, []byte("#!/bin/sh\necho formatted"), 0755)

	f1 := &config.Formatter{Name: "fmt1", Path: "/nonexistent/binary", Mode: "stdin"}
	f2 := &config.Formatter{Name: "fmt2", Path: script, Mode: "stdin"}

	result, err := FormatFallback(context.Background(), []*config.Formatter{f1, f2}, "/tmp/test.txt", []byte("hello"), nil)
	if err != nil {
		t.Fatalf("FormatFallback: %v", err)
	}
	if result.Formatted != "formatted\n" {
		t.Errorf("formatted = %q, want %q", result.Formatted, "formatted\n")
	}
}

func TestFormatFallback_AllFail(t *testing.T) {
	f1 := &config.Formatter{Name: "fmt1", Path: "/nonexistent/binary1", Mode: "stdin"}
	f2 := &config.Formatter{Name: "fmt2", Path: "/nonexistent/binary2", Mode: "stdin"}

	_, err := FormatFallback(context.Background(), []*config.Formatter{f1, f2}, "/tmp/test.txt", []byte("hello"), nil)
	if err == nil {
		t.Fatal("expected error when all formatters fail")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test -v -run "TestFormat(Chain|Fallback)" ./internal/formatter/`
Expected: FAIL — functions undefined

**Step 3: Write minimal implementation**

Add to `internal/formatter/executor.go`:

```go
func FormatChain(ctx context.Context, formatters []*config.Formatter, filePath string, content []byte, executor subprocess.Executor) (*Result, error) {
	current := content
	changed := false

	for _, f := range formatters {
		result, err := Format(ctx, f, filePath, current, executor)
		if err != nil {
			return nil, fmt.Errorf("chain formatter %s: %w", f.Name, err)
		}
		if result.Changed {
			changed = true
			current = []byte(result.Formatted)
		}
	}

	return &Result{
		Formatted: string(current),
		Changed:   changed,
	}, nil
}

func FormatFallback(ctx context.Context, formatters []*config.Formatter, filePath string, content []byte, executor subprocess.Executor) (*Result, error) {
	var lastErr error

	for _, f := range formatters {
		result, err := Format(ctx, f, filePath, content, executor)
		if err != nil {
			lastErr = err
			continue
		}
		return result, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all formatters failed, last error: %w", lastErr)
	}
	return &Result{Formatted: string(content), Changed: false}, nil
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test -v -run "TestFormat(Chain|Fallback)" ./internal/formatter/`
Expected: PASS

**Step 5: Commit**

```
git add internal/formatter/executor.go internal/formatter/executor_test.go
git commit -m "feat: add FormatChain and FormatFallback execution modes"
```

---

### Task 9: Update Handler and Bridge for New Formatter Router

**Files:**
- Modify: `internal/server/handler.go`
- Modify: `internal/server/server.go`
- Modify: `internal/tools/bridge.go`

**Step 1: Update server.go to load filetype configs**

In `internal/server/server.go`, update `New()` to:

1. Load filetype configs: `filetype.LoadMerged()`
2. Build server router from filetypes: `NewRouter(filetypes)`
3. Build formatter map from `FormatterConfig`
4. Build formatter router from filetypes + formatter map: `formatter.NewRouter(filetypes, fmtMap)`

```go
// In New(), replace the current fmtRouter setup (lines 65-75) with:
fmtCfg, err := config.LoadMergedFormatters()
if err != nil {
    fmt.Fprintf(os.Stderr, "warning: could not load formatter config: %v\n", err)
} else {
    fmtMap := make(map[string]*config.Formatter)
    for i := range fmtCfg.Formatters {
        f := &fmtCfg.Formatters[i]
        if !f.Disabled {
            fmtMap[f.Name] = f
        }
    }

    ftConfigs, err := filetype.LoadMerged()
    if err != nil {
        fmt.Fprintf(os.Stderr, "warning: could not load filetype config: %v\n", err)
    } else {
        fmtRouter, err := formatter.NewRouter(ftConfigs, fmtMap)
        if err != nil {
            fmt.Fprintf(os.Stderr, "warning: could not create formatter router: %v\n", err)
        } else {
            s.fmtRouter = fmtRouter
        }
    }
}
```

Also update `NewRouter(cfg)` call to use filetype configs.

**Step 2: Update handler.go tryExternalFormat**

Replace `handler.go`'s `tryExternalFormat` (lines 192-243) to use
`formatter.MatchResult` with chain/fallback:

```go
func (h *Handler) tryExternalFormat(ctx context.Context, msg *jsonrpc.Message) (*jsonrpc.Message, bool) {
    if h.server.fmtRouter == nil {
        return nil, false
    }

    var params map[string]any
    if err := json.Unmarshal(msg.Params, &params); err != nil {
        return nil, false
    }

    uri := lsp.ExtractURI(msg.Method, params)
    if uri == "" {
        return nil, false
    }

    filePath := uri.Path()
    match := h.server.fmtRouter.Match(filePath)
    if match == nil {
        return nil, false
    }

    if match.LSPFormat == "prefer" {
        return nil, false
    }

    content, err := os.ReadFile(filePath)
    if err != nil {
        resp, _ := jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError,
            fmt.Sprintf("reading file for formatting: %v", err), nil)
        return resp, true
    }

    var result *formatter.Result
    switch match.Mode {
    case "chain":
        result, err = formatter.FormatChain(ctx, match.Formatters, filePath, content, h.server.executor)
    case "fallback":
        result, err = formatter.FormatFallback(ctx, match.Formatters, filePath, content, h.server.executor)
    }

    if err != nil {
        if match.LSPFormat == "fallback" {
            return nil, false // let LSP handle it
        }
        resp, _ := jsonrpc.NewErrorResponse(*msg.ID, jsonrpc.InternalError,
            fmt.Sprintf("formatter failed: %v", err), nil)
        return resp, true
    }

    if !result.Changed {
        resp, _ := jsonrpc.NewResponse(*msg.ID, []lsp.TextEdit{})
        return resp, true
    }

    lines := strings.Count(string(content), "\n")
    edit := lsp.TextEdit{
        Range: lsp.Range{
            Start: lsp.Position{Line: 0, Character: 0},
            End:   lsp.Position{Line: lines + 1, Character: 0},
        },
        NewText: result.Formatted,
    }

    resp, _ := jsonrpc.NewResponse(*msg.ID, []lsp.TextEdit{edit})
    return resp, true
}
```

**Step 3: Update bridge.go tryExternalFormat**

Apply the same pattern to `internal/tools/bridge.go`'s `tryExternalFormat`
(lines 303-339):

```go
func (b *Bridge) tryExternalFormat(ctx context.Context, uri lsp.DocumentURI) (*command.Result, bool) {
    if b.fmtRouter == nil {
        return nil, false
    }

    filePath := uri.Path()
    match := b.fmtRouter.Match(filePath)
    if match == nil {
        return nil, false
    }

    if match.LSPFormat == "prefer" {
        return nil, false
    }

    content, err := b.readFile(uri)
    if err != nil {
        return command.TextErrorResult(fmt.Sprintf("reading file: %v", err)), true
    }

    var result *formatter.Result
    switch match.Mode {
    case "chain":
        result, err = formatter.FormatChain(ctx, match.Formatters, filePath, []byte(content), b.executor)
    case "fallback":
        result, err = formatter.FormatFallback(ctx, match.Formatters, filePath, []byte(content), b.executor)
    }

    if err != nil {
        if match.LSPFormat == "fallback" {
            return nil, false
        }
        return command.TextErrorResult(fmt.Sprintf("formatter failed: %v", err)), true
    }

    if !result.Changed {
        return command.TextResult("No formatting changes needed"), true
    }

    lines := strings.Count(content, "\n")
    edit := lsp.TextEdit{
        Range: lsp.Range{
            Start: lsp.Position{Line: 0, Character: 0},
            End:   lsp.Position{Line: lines + 1, Character: 0},
        },
        NewText: result.Formatted,
    }

    text := formatTextEdits([]lsp.TextEdit{edit})
    return command.TextResult(text), true
}
```

**Step 4: Run all tests**

Run: `nix develop --command go test ./...`
Expected: PASS

**Step 5: Commit**

```
git add internal/server/handler.go internal/server/server.go internal/tools/bridge.go
git commit -m "refactor: update handler and bridge for filetype-based formatter routing"
```

---

### Task 10: Update CLI — `lux list` and `lux format`

**Files:**
- Modify: `cmd/lux/main.go`

**Step 1: Update `list` command**

Replace the current `list` command (lines 63-95 of `cmd/lux/main.go`) to show
the filetype-centric routing table. Load filetype configs, LSPs, and formatters,
then print in tabular format:

```
Filetype  Extensions  LSP                  Formatters       Mode
go        .go         gopls                golines          chain
python    .py         pyright              isort, black     chain
```

**Step 2: Update `format` command**

The `format` command (lines 317-373) currently loads `FormatterConfig` and
creates a single-formatter router. Update to:

1. Load filetype configs
2. Load formatter configs
3. Build formatter map
4. Create `formatter.NewRouter(filetypes, fmtMap)`
5. Match file → get `MatchResult`
6. Execute with `FormatChain` or `FormatFallback` based on mode

**Step 3: Run `lux list` manually to verify**

Run: `nix develop --command go run ./cmd/lux list`
Expected: Tabular output showing filetypes (may be empty without config)

**Step 4: Commit**

```
git add cmd/lux/main.go
git commit -m "refactor: update lux list and format for filetype configs"
```

---

### Task 11: Add `lux init` Command

**Files:**
- Create: `cmd/lux/init.go`

**Step 1: Write the init command**

Create `cmd/lux/init.go` with:

- `initCmd` — creates empty `lsps.toml`, `formatters.toml`, and `filetype/`
  directory
- `--default` flag — writes curated defaults from the design doc
- `--force` flag — allows overwriting existing files

The default configs should be defined as Go string constants or embedded files
in the command file.

**Step 2: Test manually**

Run: `nix develop --command go run ./cmd/lux init --default`
Expected: Creates `~/.config/lux/lsps.toml`, `formatters.toml`, and
`filetype/*.toml` with all defaults from the design doc.

Verify: `nix develop --command go run ./cmd/lux list`
Expected: Shows all 8 filetype configs with their LSPs and formatters.

**Step 3: Commit**

```
git add cmd/lux/init.go
git commit -m "feat: add lux init and lux init --default commands"
```

---

### Task 12: Update `lux add` for Filetype Configs

**Files:**
- Modify: `cmd/lux/main.go` (the `add` command)
- Modify: `internal/capabilities/bootstrap.go` (if it sets extensions)

**Step 1: Update add command**

Update the `add` command to also generate a filetype config when adding an LSP.
After probing capabilities, create `filetype/<name>.toml` with the detected
extensions and language IDs.

Add `--formatter` flag for adding formatters:
- Writes to `formatters.toml`
- Requires `--extensions` flag
- Creates or updates matching filetype config

Add `--filetype` flag for direct filetype config creation:
- Creates `filetype/<name>.toml`
- Accepts `--lsp`, `--formatters`, `--formatter-mode`, `--extensions`

**Step 2: Test manually**

Run: `nix develop --command go run ./cmd/lux add --filetype test --extensions txt --lsp gopls`
Expected: Creates `~/.config/lux/filetype/test.toml`

**Step 3: Commit**

```
git add cmd/lux/main.go internal/capabilities/bootstrap.go
git commit -m "feat: update lux add for filetype configs"
```

---

### Task 13: Run Full Test Suite and Fix Remaining Issues

**Files:**
- Any files with compilation errors or test failures

**Step 1: Build**

Run: `nix develop --command go build ./...`
Expected: No compilation errors. Fix any that arise.

**Step 2: Run all tests**

Run: `nix develop --command go test -v ./...`
Expected: All tests pass. Fix any failures.

**Step 3: Run linter**

Run: `nix develop --command go vet ./...`
Expected: No warnings. Fix any that arise.

**Step 4: Nix build**

Run: `just build`
Expected: Nix build succeeds.

**Step 5: Commit any fixes**

```
git add -A
git commit -m "fix: resolve remaining issues from filetype config migration"
```

---

Plan complete and saved to `docs/plans/2026-02-21-filetype-config-plan.md`. Two execution options:

**1. Subagent-Driven (this session)** — I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** — Open a new session with executing-plans, batch execution with checkpoints

Which approach?