package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTrustWorkspaceNewFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".claude.json")

	err := TrustWorkspace(configPath, "/home/user/eng/worktrees/repo/branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	doc := readJSON(t, configPath)
	projects, _ := doc["projects"].(map[string]any)
	if projects == nil {
		t.Fatal("expected projects key")
	}

	entry, _ := projects["/home/user/eng/worktrees/repo/branch"].(map[string]any)
	if entry == nil {
		t.Fatal("expected project entry")
	}

	accepted, _ := entry["hasTrustDialogAccepted"].(bool)
	if !accepted {
		t.Error("expected hasTrustDialogAccepted to be true")
	}
}

func TestTrustWorkspacePreservesExisting(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".claude.json")

	existing := map[string]any{
		"numStartups": float64(5),
		"projects": map[string]any{
			"/other/path": map[string]any{
				"hasTrustDialogAccepted": true,
				"customKey":             "value",
			},
		},
	}
	writeJSON(t, configPath, existing)

	err := TrustWorkspace(configPath, "/new/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	doc := readJSON(t, configPath)

	if doc["numStartups"] != float64(5) {
		t.Error("expected numStartups to be preserved")
	}

	projects, _ := doc["projects"].(map[string]any)
	otherEntry, _ := projects["/other/path"].(map[string]any)
	if otherEntry == nil {
		t.Fatal("expected other project entry to be preserved")
	}
	if otherEntry["customKey"] != "value" {
		t.Error("expected customKey in other entry to be preserved")
	}

	newEntry, _ := projects["/new/path"].(map[string]any)
	if newEntry == nil {
		t.Fatal("expected new project entry")
	}
	accepted, _ := newEntry["hasTrustDialogAccepted"].(bool)
	if !accepted {
		t.Error("expected hasTrustDialogAccepted to be true")
	}
}

func TestTrustWorkspaceIdempotent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".claude.json")

	existing := map[string]any{
		"projects": map[string]any{
			"/already/trusted": map[string]any{
				"hasTrustDialogAccepted": true,
				"allowedTools":           []string{"Read"},
			},
		},
	}
	writeJSON(t, configPath, existing)

	err := TrustWorkspace(configPath, "/already/trusted")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	doc := readJSON(t, configPath)
	projects, _ := doc["projects"].(map[string]any)
	entry, _ := projects["/already/trusted"].(map[string]any)
	if entry == nil {
		t.Fatal("expected project entry")
	}

	accepted, _ := entry["hasTrustDialogAccepted"].(bool)
	if !accepted {
		t.Error("expected hasTrustDialogAccepted to remain true")
	}

	tools, _ := entry["allowedTools"].([]any)
	if len(tools) != 1 {
		t.Errorf("expected allowedTools to be preserved, got %v", tools)
	}
}

func TestTrustWorkspaceCorruptFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".claude.json")

	os.WriteFile(configPath, []byte("not valid json!!!"), 0o644)

	err := TrustWorkspace(configPath, "/some/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	doc := readJSON(t, configPath)
	projects, _ := doc["projects"].(map[string]any)
	entry, _ := projects["/some/path"].(map[string]any)
	accepted, _ := entry["hasTrustDialogAccepted"].(bool)
	if !accepted {
		t.Error("expected hasTrustDialogAccepted to be true")
	}
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parsing JSON: %v", err)
	}
	return doc
}

func writeJSON(t *testing.T, path string, doc map[string]any) {
	t.Helper()
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshaling JSON: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("writing file: %v", err)
	}
}
