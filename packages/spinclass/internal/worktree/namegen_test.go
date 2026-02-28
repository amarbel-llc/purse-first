package worktree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRandomNameFormat(t *testing.T) {
	repoPath := t.TempDir()

	name := RandomName(repoPath)
	parts := strings.SplitN(name, "-", 2)
	if len(parts) != 2 {
		t.Fatalf("RandomName() = %q, want adjective-noun format", name)
	}
	if parts[0] == "" || parts[1] == "" {
		t.Fatalf("RandomName() = %q, has empty adjective or noun", name)
	}
}

func TestRandomNameAvoidsCollision(t *testing.T) {
	repoPath := t.TempDir()
	wtDir := filepath.Join(repoPath, WorktreesDir)
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Generate a name, then create a directory with that name to force collision
	first := RandomName(repoPath)
	if err := os.Mkdir(filepath.Join(wtDir, first), 0o755); err != nil {
		t.Fatal(err)
	}

	// Generate many names — none should equal the taken name
	for range 100 {
		name := RandomName(repoPath)
		if name == first {
			t.Fatalf("RandomName() returned colliding name %q", name)
		}
	}
}

func TestRandomNameUsesValidWords(t *testing.T) {
	repoPath := t.TempDir()

	adjSet := make(map[string]bool, len(adjectives))
	for _, a := range adjectives {
		adjSet[a] = true
	}
	nounSet := make(map[string]bool, len(nouns))
	for _, n := range nouns {
		nounSet[n] = true
	}

	for range 50 {
		name := RandomName(repoPath)
		parts := strings.SplitN(name, "-", 2)
		if !adjSet[parts[0]] {
			t.Errorf("adjective %q not in word list", parts[0])
		}
		if !nounSet[parts[1]] {
			t.Errorf("noun %q not in word list", parts[1])
		}
	}
}
