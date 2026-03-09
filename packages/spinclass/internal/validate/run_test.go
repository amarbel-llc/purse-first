package validate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSweatfile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creating directory for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func TestRunValidHierarchy(t *testing.T) {
	home := t.TempDir()
	repoDir := filepath.Join(home, "eng", "myrepo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	globalPath := filepath.Join(home, ".config", "spinclass", "sweatfile")
	writeSweatfile(t, globalPath, `claude-allow = ["Read", "Bash(git *)"]`)

	repoSweatfile := filepath.Join(repoDir, "sweatfile")
	writeSweatfile(t, repoSweatfile, `git-excludes = [".direnv/"]`)

	var buf bytes.Buffer
	exitCode := Run(&buf, home, repoDir)

	out := buf.String()
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d\noutput:\n%s", exitCode, out)
	}
	if !strings.HasPrefix(out, "TAP version 14\n") {
		t.Errorf("expected TAP version header, got: %q", out)
	}
	if !strings.Contains(out, "# Subtest:") {
		t.Errorf("expected subtests, got: %q", out)
	}
}

func TestRunInvalidSyntax(t *testing.T) {
	home := t.TempDir()
	repoDir := filepath.Join(home, "eng", "myrepo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	repoSweatfile := filepath.Join(repoDir, "sweatfile")
	writeSweatfile(t, repoSweatfile, `claude-allow = ["Bash(git *"]`)

	var buf bytes.Buffer
	exitCode := Run(&buf, home, repoDir)

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(buf.String(), "not ok") {
		t.Errorf("expected not ok in output, got: %q", buf.String())
	}
}

func TestRunNoSweatfiles(t *testing.T) {
	home := t.TempDir()
	repoDir := filepath.Join(home, "eng", "myrepo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	exitCode := Run(&buf, home, repoDir)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(buf.String(), "# SKIP") {
		t.Errorf("expected SKIP directives, got: %q", buf.String())
	}
}

func TestRunInvalidTOML(t *testing.T) {
	home := t.TempDir()
	repoDir := filepath.Join(home, "eng", "myrepo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	repoSweatfile := filepath.Join(repoDir, "sweatfile")
	writeSweatfile(t, repoSweatfile, `this is not valid toml [[[`)

	var buf bytes.Buffer
	exitCode := Run(&buf, home, repoDir)

	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(buf.String(), "not ok") {
		t.Errorf("expected not ok in output, got: %q", buf.String())
	}
}

func TestRunDuplicatesInMerged(t *testing.T) {
	home := t.TempDir()
	repoDir := filepath.Join(home, "eng", "myrepo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	globalPath := filepath.Join(home, ".config", "spinclass", "sweatfile")
	writeSweatfile(t, globalPath, `claude-allow = ["Read"]`)

	repoSweatfile := filepath.Join(repoDir, "sweatfile")
	writeSweatfile(t, repoSweatfile, `claude-allow = ["Read"]`)

	var buf bytes.Buffer
	exitCode := Run(&buf, home, repoDir)

	// Duplicates are warnings, not errors — exit code 0
	if exitCode != 0 {
		t.Errorf("expected exit code 0 (warnings only), got %d\noutput:\n%s", exitCode, buf.String())
	}
}
