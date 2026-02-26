package completions

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/amarbel-llc/spinclass/internal/worktree"
)

func Local(startDir string, w io.Writer) {
	// If startDir is a repo, list its worktrees
	gitDir := filepath.Join(startDir, ".git")
	if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
		repoName := filepath.Base(startDir)
		fmt.Fprintf(w, "%s/\tnew worktree\n", repoName)

		for _, wtPath := range worktree.ListWorktrees(startDir) {
			branch := filepath.Base(wtPath)
			fmt.Fprintf(w, "%s\texisting worktree\n", branch)
		}
		return
	}

	// Otherwise scan children for repos
	entries, err := os.ReadDir(startDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		child := filepath.Join(startDir, entry.Name())
		childGitDir := filepath.Join(child, ".git")
		if info, err := os.Stat(childGitDir); err != nil || !info.IsDir() {
			continue
		}

		repoName := entry.Name()
		fmt.Fprintf(w, "%s/\tnew worktree\n", repoName)

		for _, wtPath := range worktree.ListWorktrees(child) {
			branch := filepath.Base(wtPath)
			fmt.Fprintf(w, "%s\texisting worktree\n", branch)
		}
	}
}
