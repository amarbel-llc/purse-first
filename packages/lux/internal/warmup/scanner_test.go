package warmup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/amarbel-llc/lux/internal/config"
	"github.com/amarbel-llc/lux/internal/config/filetype"
)

func TestScanner_ScanDirectories(t *testing.T) {
	dir := t.TempDir()

	os.MkdirAll(filepath.Join(dir, "src"), 0755)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(dir, "src", "lib.py"), []byte(""), 0644)

	cfg := &config.Config{
		LSPs: []config.LSP{
			{Name: "gopls", Flake: "nixpkgs#gopls"},
			{Name: "pyright", Flake: "nixpkgs#pyright"},
			{Name: "rust-analyzer", Flake: "nixpkgs#rust-analyzer"},
		},
	}

	filetypes := []*filetype.Config{
		{Name: "go", Extensions: []string{".go"}, LSP: "gopls"},
		{Name: "python", Extensions: []string{".py"}, LSP: "pyright"},
		{Name: "rust", Extensions: []string{".rs"}, LSP: "rust-analyzer"},
	}

	scanner := NewScanner(cfg, filetypes)
	result := scanner.ScanDirectories([]string{dir})

	if !result.LSPNames["gopls"] {
		t.Error("expected gopls to be found")
	}
	if !result.LSPNames["pyright"] {
		t.Error("expected pyright to be found")
	}
	if result.LSPNames["rust-analyzer"] {
		t.Error("expected rust-analyzer to NOT be found (no .rs files)")
	}
}

func TestScanner_ScanDirectories_SkipsDirs(t *testing.T) {
	dir := t.TempDir()

	os.MkdirAll(filepath.Join(dir, "node_modules"), 0755)
	os.WriteFile(filepath.Join(dir, "node_modules", "index.js"), []byte(""), 0644)
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)
	os.WriteFile(filepath.Join(dir, ".git", "config.py"), []byte(""), 0644)

	cfg := &config.Config{
		LSPs: []config.LSP{
			{Name: "tsserver", Flake: "nixpkgs#tsserver"},
			{Name: "pyright", Flake: "nixpkgs#pyright"},
		},
	}

	filetypes := []*filetype.Config{
		{Name: "javascript", Extensions: []string{".js"}, LSP: "tsserver"},
		{Name: "python", Extensions: []string{".py"}, LSP: "pyright"},
	}

	scanner := NewScanner(cfg, filetypes)
	result := scanner.ScanDirectories([]string{dir})

	if result.LSPNames["tsserver"] {
		t.Error("expected tsserver to NOT be found (in node_modules)")
	}
	if result.LSPNames["pyright"] {
		t.Error("expected pyright to NOT be found (in .git)")
	}
}

func TestScanner_ScanDirectories_EmptyConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	scanner := NewScanner(cfg, nil)
	result := scanner.ScanDirectories([]string{dir})

	if len(result.LSPNames) != 0 {
		t.Errorf("expected 0 LSPs found, got %d", len(result.LSPNames))
	}
}

func TestScanner_ScanDirectories_NilFiletypes(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)

	cfg := &config.Config{
		LSPs: []config.LSP{
			{Name: "gopls", Flake: "nixpkgs#gopls"},
		},
	}

	scanner := NewScanner(cfg, nil)
	result := scanner.ScanDirectories([]string{dir})

	if len(result.LSPNames) != 0 {
		t.Errorf("expected 0 LSPs found without filetypes, got %d", len(result.LSPNames))
	}
}

func TestScanner_AllLSPNames(t *testing.T) {
	cfg := &config.Config{
		LSPs: []config.LSP{
			{Name: "gopls", Flake: "f"},
			{Name: "pyright", Flake: "f"},
		},
	}

	scanner := NewScanner(cfg, nil)
	names := scanner.AllLSPNames()

	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}
	found := make(map[string]bool)
	for _, n := range names {
		found[n] = true
	}
	if !found["gopls"] || !found["pyright"] {
		t.Error("expected gopls and pyright in names")
	}
}
