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

func TestRouterMatch_LSPFormat(t *testing.T) {
	tests := []struct {
		name          string
		lspFormat     string
		formatters    []string
		wantLSPFormat string
	}{
		{"explicit never", "never", []string{"golines"}, "never"},
		{"explicit fallback", "fallback", []string{"golines"}, "fallback"},
		{"default with formatters is never", "", []string{"golines"}, "never"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filetypes := []*filetype.Config{
				{Name: "go", Extensions: []string{"go"}, Formatters: tt.formatters, LSPFormat: tt.lspFormat},
			}
			fmtMap := map[string]*config.Formatter{
				"golines": {Name: "golines", Flake: "nixpkgs#golines"},
			}

			router, err := NewRouter(filetypes, fmtMap)
			if err != nil {
				t.Fatal(err)
			}

			result := router.Match("/src/main.go")
			if result == nil {
				t.Fatal("expected match result")
			}
			if result.LSPFormat != tt.wantLSPFormat {
				t.Errorf("LSPFormat = %q, want %q", result.LSPFormat, tt.wantLSPFormat)
			}
		})
	}
}

func TestRouterMatch_NilFiletypes(t *testing.T) {
	router, err := NewRouter(nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	result := router.Match("/src/main.go")
	if result != nil {
		t.Error("expected nil result for nil filetypes")
	}
}

func TestRouterMatch_FormatterOrdering(t *testing.T) {
	filetypes := []*filetype.Config{
		{Name: "python", Extensions: []string{"py"}, Formatters: []string{"isort", "black", "autopep8"}},
	}
	formatters := map[string]*config.Formatter{
		"isort":    {Name: "isort", Flake: "nixpkgs#isort"},
		"black":    {Name: "black", Flake: "nixpkgs#black"},
		"autopep8": {Name: "autopep8", Flake: "nixpkgs#autopep8"},
	}

	router, err := NewRouter(filetypes, formatters)
	if err != nil {
		t.Fatal(err)
	}

	result := router.Match("/src/main.py")
	if result == nil {
		t.Fatal("expected match result")
	}
	if len(result.Formatters) != 3 {
		t.Fatalf("expected 3 formatters, got %d", len(result.Formatters))
	}

	// Verify order matches the filetype config declaration order
	expectedOrder := []string{"isort", "black", "autopep8"}
	for i, f := range result.Formatters {
		if f.Name != expectedOrder[i] {
			t.Errorf("formatter[%d] = %q, want %q", i, f.Name, expectedOrder[i])
		}
	}
}

func TestRouterMatch_FiletypeWithoutFormatters(t *testing.T) {
	filetypes := []*filetype.Config{
		{Name: "go", Extensions: []string{"go"}, LSP: "gopls"},
	}
	formatters := map[string]*config.Formatter{
		"golines": {Name: "golines", Flake: "nixpkgs#golines"},
	}

	router, err := NewRouter(filetypes, formatters)
	if err != nil {
		t.Fatal(err)
	}

	result := router.Match("/src/main.go")
	if result != nil {
		t.Error("expected nil result for filetype without formatters")
	}
}
