package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateMappings(t *testing.T) {
	app := NewUtility("grit", "Git operations")

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		MapsTools: []ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git status"}, UseWhen: "checking repository status"},
		},
	})

	app.AddCommand(&Command{
		Name:        "diff",
		Description: Description{Short: "Show changes"},
		MapsTools: []ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git diff"}, UseWhen: "viewing changes"},
		},
	})

	app.AddCommand(&Command{
		Name:        "internal",
		Description: Description{Short: "Internal only"},
		Hidden:      true,
	})

	dir := t.TempDir()
	if err := app.GenerateMappings(dir); err != nil {
		t.Fatalf("GenerateMappings: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "grit", "mappings.json"))
	if err != nil {
		t.Fatalf("read mappings.json: %v", err)
	}

	var mf struct {
		Server   string `json:"server"`
		Mappings []struct {
			Replaces        string   `json:"replaces"`
			CommandPrefixes []string `json:"command_prefixes"`
			Tools           []struct {
				Name    string `json:"name"`
				UseWhen string `json:"use_when"`
			} `json:"tools"`
			Reason string `json:"reason"`
		} `json:"mappings"`
	}
	if err := json.Unmarshal(data, &mf); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if mf.Server != "grit" {
		t.Errorf("server = %q, want grit", mf.Server)
	}
	if len(mf.Mappings) != 2 {
		t.Fatalf("mappings len = %d, want 2", len(mf.Mappings))
	}
	for _, m := range mf.Mappings {
		if m.Replaces != "Bash" {
			t.Errorf("replaces = %q, want Bash", m.Replaces)
		}
	}
}

func TestGenerateMappingsWithExtensions(t *testing.T) {
	app := NewUtility("lux", "LSP multiplexer")

	app.AddCommand(&Command{
		Name:        "hover",
		Description: Description{Short: "Get type info"},
		MapsTools: []ToolMapping{
			{Replaces: "Read", Extensions: []string{".go", ".py"}, UseWhen: "getting type info"},
		},
	})

	app.AddCommand(&Command{
		Name:        "document_symbols",
		Description: Description{Short: "Get symbols"},
		MapsTools: []ToolMapping{
			{Replaces: "Read", Extensions: []string{".go", ".py"}, UseWhen: "understanding file structure"},
		},
	})

	dir := t.TempDir()
	if err := app.GenerateMappings(dir); err != nil {
		t.Fatalf("GenerateMappings: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "lux", "mappings.json"))
	if err != nil {
		t.Fatalf("read mappings.json: %v", err)
	}

	var mf struct {
		Server   string `json:"server"`
		Mappings []struct {
			Replaces   string   `json:"replaces"`
			Extensions []string `json:"extensions"`
			Tools      []struct {
				Name    string `json:"name"`
				UseWhen string `json:"use_when"`
			} `json:"tools"`
		} `json:"mappings"`
	}
	if err := json.Unmarshal(data, &mf); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if mf.Server != "lux" {
		t.Errorf("server = %q, want lux", mf.Server)
	}

	// Two separate entries (no consolidation yet — one per MapsTools entry)
	if len(mf.Mappings) != 2 {
		t.Fatalf("mappings len = %d, want 2", len(mf.Mappings))
	}

	for _, m := range mf.Mappings {
		if m.Replaces != "Read" {
			t.Errorf("replaces = %q, want Read", m.Replaces)
		}
		if len(m.Extensions) != 2 || m.Extensions[0] != ".go" {
			t.Errorf("extensions = %v, want [.go .py]", m.Extensions)
		}
	}
}

func TestGenerateMappingsNoMappings(t *testing.T) {
	app := NewUtility("test", "test")
	app.AddCommand(&Command{Name: "foo"})

	dir := t.TempDir()
	if err := app.GenerateMappings(dir); err != nil {
		t.Fatalf("GenerateMappings: %v", err)
	}

	path := filepath.Join(dir, "test", "mappings.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("mappings.json should not exist when no commands have bash mappings")
	}
}
