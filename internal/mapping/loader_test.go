package mapping

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeMappingFile(t *testing.T, dir string, mf MappingFile) {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(mf)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, mf.Server+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMappingsFromGlobalDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	mf := MappingFile{
		Server: "lux",
		Mappings: []Mapping{
			{
				Replaces:   "Read",
				Extensions: []string{".go"},
				Tools: []ToolSuggestion{
					{Name: "lsp_hover", UseWhen: "type info"},
				},
				Reason: "Use lux",
			},
		},
	}

	writeMappingFile(t, filepath.Join(tmpDir, "purse-first"), mf)

	files, err := LoadMappings(t.TempDir()) // project dir with no .purse-first
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 mapping file, got %d", len(files))
	}

	if files[0].Server != "lux" {
		t.Errorf("expected server lux, got %s", files[0].Server)
	}
}

func TestLoadMappingsLocalOverridesGlobal(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	globalMF := MappingFile{
		Server: "lux",
		Mappings: []Mapping{
			{Replaces: "Read", Extensions: []string{".go"}, Reason: "global"},
		},
	}

	localMF := MappingFile{
		Server: "lux",
		Mappings: []Mapping{
			{Replaces: "Read", Extensions: []string{".go", ".py"}, Reason: "local override"},
		},
	}

	writeMappingFile(t, filepath.Join(tmpDir, "purse-first"), globalMF)
	writeMappingFile(t, filepath.Join(projectDir, ".purse-first"), localMF)

	files, err := LoadMappings(projectDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 mapping file, got %d", len(files))
	}

	if files[0].Mappings[0].Reason != "local override" {
		t.Errorf("expected local override, got %s", files[0].Mappings[0].Reason)
	}
}

func TestLoadMappingsMultipleServers(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	luxMF := MappingFile{
		Server:   "lux",
		Mappings: []Mapping{{Replaces: "Read"}},
	}

	nixMF := MappingFile{
		Server:   "nix",
		Mappings: []Mapping{{Replaces: "Bash"}},
	}

	dir := filepath.Join(tmpDir, "purse-first")
	writeMappingFile(t, dir, luxMF)
	writeMappingFile(t, dir, nixMF)

	files, err := LoadMappings(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 mapping files, got %d", len(files))
	}
}

func TestLoadMappingsNoFiles(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	files, err := LoadMappings(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 0 {
		t.Fatalf("expected 0 mapping files, got %d", len(files))
	}
}

func TestFindMatchByToolAndExtension(t *testing.T) {
	files := []MappingFile{
		{
			Server: "lux",
			Mappings: []Mapping{
				{
					Replaces:   "Read",
					Extensions: []string{".go", ".py"},
					Tools: []ToolSuggestion{
						{Name: "lsp_hover", UseWhen: "type info"},
					},
					Reason: "Use lux",
				},
			},
		},
	}

	match := FindMatch(files, "Read", "/path/to/foo.go")
	if match == nil {
		t.Fatal("expected match, got nil")
	}
	if match.Server != "lux" {
		t.Errorf("expected server lux, got %s", match.Server)
	}
}

func TestFindMatchNoExtensionMatch(t *testing.T) {
	files := []MappingFile{
		{
			Server: "lux",
			Mappings: []Mapping{
				{
					Replaces:   "Read",
					Extensions: []string{".go"},
				},
			},
		},
	}

	match := FindMatch(files, "Read", "/path/to/foo.txt")
	if match != nil {
		t.Errorf("expected no match for .txt, got %+v", match)
	}
}

func TestFindMatchNoToolMatch(t *testing.T) {
	files := []MappingFile{
		{
			Server: "lux",
			Mappings: []Mapping{
				{
					Replaces:   "Read",
					Extensions: []string{".go"},
				},
			},
		},
	}

	match := FindMatch(files, "Write", "/path/to/foo.go")
	if match != nil {
		t.Errorf("expected no match for Write, got %+v", match)
	}
}

func TestFindMatchEmptyExtensionsMatchesAll(t *testing.T) {
	files := []MappingFile{
		{
			Server: "nix",
			Mappings: []Mapping{
				{
					Replaces:   "Bash",
					Extensions: nil,
					Tools: []ToolSuggestion{
						{Name: "nix_eval", UseWhen: "evaluating nix"},
					},
				},
			},
		},
	}

	match := FindMatch(files, "Bash", "/any/path.whatever")
	if match == nil {
		t.Fatal("expected match with empty extensions, got nil")
	}
}
