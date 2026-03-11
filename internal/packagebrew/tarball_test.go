package packagebrew

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func setupTestPackage(t *testing.T) (binPath, shareDir string) {
	t.Helper()
	dir := t.TempDir()

	binPath = filepath.Join(dir, "my-tool")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\necho hello"), 0o755); err != nil {
		t.Fatal(err)
	}

	shareDir = filepath.Join(dir, "share", "purse-first", "my-tool")
	pluginDir := filepath.Join(shareDir, ".claude-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(pluginDir, "plugin.json"),
		[]byte(`{"name":"my-tool"}`),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	return binPath, shareDir
}

func tarEntries(t *testing.T, tarballPath string) []string {
	t.Helper()
	f, err := os.Open(tarballPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()

	var names []string
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, hdr.Name)
	}

	sort.Strings(names)
	return names
}

func TestCreateBinaryTarball(t *testing.T) {
	binPath, shareDir := setupTestPackage(t)
	outputDir := t.TempDir()

	path, err := CreateTarball(TarballOptions{
		Name:      "my-tool",
		Version:   "1.0.0",
		Platform:  "darwin-arm64",
		BinPath:   binPath,
		ShareDir:  shareDir,
		OutputDir: outputDir,
	})
	if err != nil {
		t.Fatalf("CreateTarball: %v", err)
	}

	expectedName := "my-tool-1.0.0-darwin-arm64.tar.gz"
	if filepath.Base(path) != expectedName {
		t.Errorf("tarball name = %q, want %q", filepath.Base(path), expectedName)
	}

	entries := tarEntries(t, path)
	wantBin := false
	wantPlugin := false
	for _, e := range entries {
		if e == "bin/my-tool" {
			wantBin = true
		}
		if e == "share/purse-first/my-tool/.claude-plugin/plugin.json" {
			wantPlugin = true
		}
	}

	if !wantBin {
		t.Errorf("tarball missing bin/my-tool, entries: %v", entries)
	}
	if !wantPlugin {
		t.Errorf("tarball missing share data, entries: %v", entries)
	}
}

func TestCreateSkillOnlyTarball(t *testing.T) {
	_, shareDir := setupTestPackage(t)
	outputDir := t.TempDir()

	path, err := CreateTarball(TarballOptions{
		Name:      "my-skills",
		Version:   "0.1.0",
		ShareDir:  shareDir,
		OutputDir: outputDir,
	})
	if err != nil {
		t.Fatalf("CreateTarball: %v", err)
	}

	expectedName := "my-skills-0.1.0.tar.gz"
	if filepath.Base(path) != expectedName {
		t.Errorf("tarball name = %q, want %q", filepath.Base(path), expectedName)
	}

	entries := tarEntries(t, path)
	for _, e := range entries {
		if e == "bin/my-skills" {
			t.Error("skill-only tarball should not contain binaries")
		}
	}
}

func TestCreateMarketplaceTarball(t *testing.T) {
	outputDir := t.TempDir()
	marketplaceJSON := []byte(`{"name":"test"}`)

	path, err := CreateMarketplaceTarball(MarketplaceTarballOptions{
		Name:            "my-marketplace",
		Version:         "1.0.0",
		MarketplaceJSON: marketplaceJSON,
		OutputDir:       outputDir,
	})
	if err != nil {
		t.Fatalf("CreateMarketplaceTarball: %v", err)
	}

	entries := tarEntries(t, path)
	found := false
	for _, e := range entries {
		if e == ".claude-plugin/marketplace.json" {
			found = true
		}
	}
	if !found {
		t.Errorf("marketplace tarball missing .claude-plugin/marketplace.json, entries: %v", entries)
	}

	expectedName := "my-marketplace-1.0.0.tar.gz"
	if filepath.Base(path) != expectedName {
		t.Errorf("tarball name = %q, want %q", filepath.Base(path), expectedName)
	}
}
