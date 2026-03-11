package packagebrew

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func setupIntegrationTest(t *testing.T) (configPath, outputDir string) {
	t.Helper()
	dir := t.TempDir()
	outputDir = filepath.Join(dir, "output")

	// Create fake binary.
	binDir := filepath.Join(dir, "bins")
	os.MkdirAll(binDir, 0o755)
	binPath := filepath.Join(binDir, "my-tool")
	os.WriteFile(binPath, []byte("#!/bin/sh\necho hello"), 0o755)

	// Create share directories.
	shareBase := filepath.Join(dir, "shares")

	toolShare := filepath.Join(shareBase, "my-tool")
	toolPlugin := filepath.Join(toolShare, ".claude-plugin")
	os.MkdirAll(toolPlugin, 0o755)
	os.WriteFile(filepath.Join(toolPlugin, "plugin.json"), []byte(`{
		"name": "my-tool",
		"mcpServers": {
			"my-tool": {"type": "stdio", "command": "my-tool"}
		}
	}`), 0o644)

	skillShare := filepath.Join(shareBase, "my-skills")
	skillPlugin := filepath.Join(skillShare, ".claude-plugin")
	skillDir := filepath.Join(skillShare, "skills", "test-skill")
	os.MkdirAll(skillPlugin, 0o755)
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillPlugin, "plugin.json"), []byte(`{"name":"my-skills"}`), 0o644)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: test\n---\nSkill content"), 0o644)

	// Write config.
	configPath = filepath.Join(dir, "brew-config.json")
	config := []byte(fmt.Sprintf(`{
		"name": "test-marketplace",
		"description": "Test",
		"owner": {"name": "tester"},
		"releaseRepo": "org/tap",
		"tapName": "org/tap",
		"license": "MIT",
		"packages": {
			"my-tool": {
				"description": "A tool",
				"version": "1.0.0",
				"binary": true,
				"platforms": {"darwin-arm64": %q},
				"share": %q,
				"brewDeps": []
			},
			"my-skills": {
				"description": "Skills",
				"version": "0.1.0",
				"binary": false,
				"share": %q,
				"brewDeps": []
			}
		}
	}`, binPath, toolShare, skillShare))

	os.WriteFile(configPath, config, 0o644)
	return configPath, outputDir
}

func TestRun(t *testing.T) {
	configPath, outputDir := setupIntegrationTest(t)

	err := Run(RunOptions{
		ConfigPath:  configPath,
		OutputDir:   outputDir,
		AutoInstall: true,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Check formulas exist.
	for _, name := range []string{"my-tool.rb", "my-skills.rb", "test-marketplace.rb"} {
		path := filepath.Join(outputDir, "Formula", name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("missing formula: %s", name)
		}
	}

	// Check tarballs exist.
	toolTarball := filepath.Join(outputDir, "tarballs", "my-tool-1.0.0-darwin-arm64.tar.gz")
	if _, err := os.Stat(toolTarball); err != nil {
		t.Error("missing binary tarball")
	}

	skillTarball := filepath.Join(outputDir, "tarballs", "my-skills-0.1.0.tar.gz")
	if _, err := os.Stat(skillTarball); err != nil {
		t.Error("missing skill tarball")
	}

	metaTarball := filepath.Join(outputDir, "tarballs", "test-marketplace-0.1.0.tar.gz")
	if _, err := os.Stat(metaTarball); err != nil {
		t.Error("missing meta tarball")
	}

	// Check marketplace.json exists.
	mpPath := filepath.Join(outputDir, ".claude-plugin", "marketplace.json")
	if _, err := os.Stat(mpPath); err != nil {
		t.Error("missing marketplace.json")
	}

	// Check README exists.
	readmePath := filepath.Join(outputDir, "README.md")
	if _, err := os.Stat(readmePath); err != nil {
		t.Error("missing README.md")
	}
}
