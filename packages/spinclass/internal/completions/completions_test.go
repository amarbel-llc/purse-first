package completions

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalListsRepos(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a repo as a child of tmpDir
	os.MkdirAll(filepath.Join(tmpDir, "myrepo", ".git"), 0o755)

	var buf bytes.Buffer
	Local(tmpDir, &buf)

	output := buf.String()
	if !strings.Contains(output, "myrepo/") {
		t.Errorf("expected repo listing, got %q", output)
	}
	if !strings.Contains(output, "new worktree") {
		t.Errorf("expected 'new worktree' description, got %q", output)
	}
}

func TestLocalListsExistingWorktrees(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "myrepo")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755)
	wtDir := filepath.Join(repoDir, ".worktrees", "feature-x")
	os.MkdirAll(wtDir, 0o755)
	os.WriteFile(filepath.Join(wtDir, ".git"), []byte("gitdir: ../../.git/worktrees/feature-x\n"), 0o644)

	var buf bytes.Buffer
	Local(tmpDir, &buf)

	output := buf.String()
	if !strings.Contains(output, "myrepo/.worktrees/feature-x") {
		t.Errorf("expected existing worktree, got %q", output)
	}
	if !strings.Contains(output, "existing worktree") {
		t.Errorf("expected 'existing worktree' description, got %q", output)
	}
}

func TestLocalHandlesMultipleRepos(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "repo-a", ".git"), 0o755)
	os.MkdirAll(filepath.Join(tmpDir, "repo-b", ".git"), 0o755)

	var buf bytes.Buffer
	Local(tmpDir, &buf)

	output := buf.String()
	if !strings.Contains(output, "repo-a/") {
		t.Errorf("expected repo-a, got %q", output)
	}
	if !strings.Contains(output, "repo-b/") {
		t.Errorf("expected repo-b, got %q", output)
	}
}

func TestLocalOutputIsTabSeparated(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "myrepo", ".git"), 0o755)

	var buf bytes.Buffer
	Local(tmpDir, &buf)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) == 0 {
		t.Fatal("expected output lines")
	}
	if !strings.Contains(lines[0], "\t") {
		t.Errorf("expected tab-separated output, got %q", lines[0])
	}
}

func TestLocalHandlesNoRepos(t *testing.T) {
	tmpDir := t.TempDir()

	var buf bytes.Buffer
	Local(tmpDir, &buf)

	if buf.Len() != 0 {
		t.Errorf("expected empty output, got %q", buf.String())
	}
}

func TestLocalFromInsideRepo(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "myrepo")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755)
	wtDir := filepath.Join(repoDir, ".worktrees", "feat")
	os.MkdirAll(wtDir, 0o755)
	os.WriteFile(filepath.Join(wtDir, ".git"), []byte("gitdir: ../../.git/worktrees/feat\n"), 0o644)

	var buf bytes.Buffer
	Local(repoDir, &buf)

	output := buf.String()
	if !strings.Contains(output, "myrepo/") {
		t.Errorf("expected repo listing from inside repo, got %q", output)
	}
	if !strings.Contains(output, "feat") {
		t.Errorf("expected worktree listing from inside repo, got %q", output)
	}
}
