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

func TestValidate_DuplicateLanguageID(t *testing.T) {
	configs := []*Config{
		{Name: "go", LanguageIDs: []string{"go"}, LSP: "gopls"},
		{Name: "golang", LanguageIDs: []string{"go"}, LSP: "gopls"},
	}
	err := Validate(configs, map[string]bool{"gopls": true}, nil)
	if err == nil {
		t.Fatal("expected error for duplicate language_id")
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

	goConfig := findByName(merged, "go")
	if goConfig == nil {
		t.Fatal("missing go config from global")
	}
	rustConfig := findByName(merged, "rust")
	if rustConfig == nil {
		t.Fatal("missing rust config from project")
	}
}

func TestMerge_NilInputs(t *testing.T) {
	// Both nil
	merged := Merge(nil, nil)
	if len(merged) != 0 {
		t.Fatalf("expected 0 configs for nil/nil, got %d", len(merged))
	}

	// Global only
	global := []*Config{{Name: "go", Extensions: []string{"go"}, LSP: "gopls"}}
	merged = Merge(global, nil)
	if len(merged) != 1 {
		t.Fatalf("expected 1 config for global/nil, got %d", len(merged))
	}

	// Project only
	project := []*Config{{Name: "rust", Extensions: []string{"rs"}, LSP: "rust-analyzer"}}
	merged = Merge(nil, project)
	if len(merged) != 1 {
		t.Fatalf("expected 1 config for nil/project, got %d", len(merged))
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

func TestSaveTo(t *testing.T) {
	dir := t.TempDir()

	cfg := &Config{
		Name:       "go",
		Extensions: []string{"go"},
		LSP:        "gopls",
		Formatters: []string{"golines"},
	}

	if err := SaveTo(dir, cfg); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	// Verify file was created
	path := filepath.Join(dir, "go.toml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}

	// Verify content can be loaded back
	configs, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	if configs[0].LSP != "gopls" {
		t.Errorf("lsp = %q, want %q", configs[0].LSP, "gopls")
	}
	if len(configs[0].Extensions) != 1 || configs[0].Extensions[0] != "go" {
		t.Errorf("extensions = %v, want [go]", configs[0].Extensions)
	}
}

func TestSaveTo_EmptyName(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{Extensions: []string{"go"}, LSP: "gopls"}

	err := SaveTo(dir, cfg)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestSaveTo_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")

	cfg := &Config{
		Name:       "go",
		Extensions: []string{"go"},
		LSP:        "gopls",
	}

	if err := SaveTo(dir, cfg); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	path := filepath.Join(dir, "go.toml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
}

func TestSaveIfNotExistsTo_NewFile(t *testing.T) {
	dir := t.TempDir()

	cfg := &Config{
		Name:       "go",
		Extensions: []string{"go"},
		LSP:        "gopls",
	}

	created, err := SaveIfNotExistsTo(dir, cfg)
	if err != nil {
		t.Fatalf("SaveIfNotExistsTo: %v", err)
	}
	if !created {
		t.Error("expected created = true for new file")
	}

	// Verify file exists
	path := filepath.Join(dir, "go.toml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
}

func TestSaveIfNotExistsTo_ExistingFile(t *testing.T) {
	dir := t.TempDir()

	// Create the file first
	cfg := &Config{
		Name:       "go",
		Extensions: []string{"go"},
		LSP:        "gopls",
	}
	if err := SaveTo(dir, cfg); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	// Try to save again with different content
	cfg2 := &Config{
		Name:       "go",
		Extensions: []string{"go"},
		LSP:        "gopls-v2",
	}
	created, err := SaveIfNotExistsTo(dir, cfg2)
	if err != nil {
		t.Fatalf("SaveIfNotExistsTo: %v", err)
	}
	if created {
		t.Error("expected created = false for existing file")
	}

	// Verify original content is preserved
	configs, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	if configs[0].LSP != "gopls" {
		t.Errorf("lsp = %q, want %q (original should be preserved)", configs[0].LSP, "gopls")
	}
}

func TestSaveIfNotExistsTo_EmptyName(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{Extensions: []string{"go"}, LSP: "gopls"}

	_, err := SaveIfNotExistsTo(dir, cfg)
	if err == nil {
		t.Fatal("expected error for empty name")
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
