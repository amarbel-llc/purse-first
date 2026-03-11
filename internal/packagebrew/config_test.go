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
