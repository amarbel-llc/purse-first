package server

import (
	"encoding/json"
	"testing"

	"github.com/amarbel-llc/lux/internal/config/filetype"
	"github.com/amarbel-llc/lux/internal/lsp"
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

func TestRouter_EmptyConfigs(t *testing.T) {
	router, err := NewRouter(nil)
	if err != nil {
		t.Fatal(err)
	}

	got := router.RouteByURI(lsp.DocumentURI("file:///src/main.go"))
	if got != "" {
		t.Errorf("RouteByURI with empty configs = %q, want empty", got)
	}

	ft := router.FiletypeByURI(lsp.DocumentURI("file:///src/main.go"))
	if ft != nil {
		t.Error("expected nil filetype for empty configs")
	}
}

func TestRouter_Route_WithJSONParams(t *testing.T) {
	filetypes := []*filetype.Config{
		{Name: "go", Extensions: []string{"go"}, LSP: "gopls"},
	}

	router, err := NewRouter(filetypes)
	if err != nil {
		t.Fatal(err)
	}

	params := map[string]any{
		"textDocument": map[string]any{
			"uri": "file:///src/main.go",
		},
	}
	paramsJSON, _ := json.Marshal(params)

	got := router.Route(lsp.MethodTextDocumentHover, paramsJSON)
	if got != "gopls" {
		t.Errorf("Route() = %q, want %q", got, "gopls")
	}

	// Unknown file
	unknownParams := map[string]any{
		"textDocument": map[string]any{
			"uri": "file:///src/readme.md",
		},
	}
	unknownJSON, _ := json.Marshal(unknownParams)

	got = router.Route(lsp.MethodTextDocumentHover, unknownJSON)
	if got != "" {
		t.Errorf("Route() for unknown = %q, want empty", got)
	}

	// Invalid JSON
	got = router.Route(lsp.MethodTextDocumentHover, json.RawMessage("invalid"))
	if got != "" {
		t.Errorf("Route() for invalid JSON = %q, want empty", got)
	}
}

func TestRouter_RouteByExtension(t *testing.T) {
	filetypes := []*filetype.Config{
		{Name: "go", Extensions: []string{"go"}, LSP: "gopls"},
	}

	router, err := NewRouter(filetypes)
	if err != nil {
		t.Fatal(err)
	}

	got := router.RouteByExtension(".go")
	if got != "gopls" {
		t.Errorf("RouteByExtension(.go) = %q, want %q", got, "gopls")
	}

	got = router.RouteByExtension(".rs")
	if got != "" {
		t.Errorf("RouteByExtension(.rs) = %q, want empty", got)
	}
}

func TestRouter_RouteByLanguageID(t *testing.T) {
	filetypes := []*filetype.Config{
		{Name: "go", LanguageIDs: []string{"go"}, LSP: "gopls"},
	}

	router, err := NewRouter(filetypes)
	if err != nil {
		t.Fatal(err)
	}

	got := router.RouteByLanguageID("go")
	if got != "gopls" {
		t.Errorf("RouteByLanguageID(go) = %q, want %q", got, "gopls")
	}

	got = router.RouteByLanguageID("rust")
	if got != "" {
		t.Errorf("RouteByLanguageID(rust) = %q, want empty", got)
	}
}
