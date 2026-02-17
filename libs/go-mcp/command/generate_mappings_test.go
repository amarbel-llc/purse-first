package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateMappings(t *testing.T) {
	app := NewApp("grit", "Git operations")

	app.AddCommand(&Command{
		Name:        "status",
		Description: Description{Short: "Show status"},
		MapsBash: []BashMapping{
			{Prefixes: []string{"git status"}, UseWhen: "checking repository status"},
		},
	})

	app.AddCommand(&Command{
		Name:        "diff",
		Description: Description{Short: "Show changes"},
		MapsBash: []BashMapping{
			{Prefixes: []string{"git diff"}, UseWhen: "viewing changes"},
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

func TestGenerateMappingsNoMappings(t *testing.T) {
	app := NewApp("test", "test")
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
