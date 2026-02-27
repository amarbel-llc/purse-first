package sweatfile

import (
	"os"
	"path/filepath"

	"github.com/amarbel-llc/spinclass/internal/git"
)

// TODO replace with util
func fileExists(path string) (os.FileInfo, bool) {
	info, err := os.Stat(path)
	return info, err == nil
}

func resolveExcludePath(worktreePath string) (string, error) {
	rel, err := git.Run(worktreePath, "rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(rel) {
		rel = filepath.Join(worktreePath, rel)
	}
	return rel, nil
}
