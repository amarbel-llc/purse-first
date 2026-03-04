package pull

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/amarbel-llc/spinclass/internal/git"
	tap "github.com/amarbel-llc/purse-first/packages/tap-dancer/go"
	"github.com/amarbel-llc/spinclass/internal/worktree"
)

type repoInfo struct {
	name     string
	repoPath string
	dirty    bool
}

type worktreeInfo struct {
	repo         string
	branch       string
	repoPath     string
	worktreePath string
	dirty        bool
}

func scanRepos(startDir string) []repoInfo {
	var repos []repoInfo

	// If startDir is a repo, return just that
	gitDir := filepath.Join(startDir, ".git")
	if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
		porcelain := git.StatusPorcelain(startDir)
		repos = append(repos, repoInfo{
			name:     filepath.Base(startDir),
			repoPath: startDir,
			dirty:    porcelain != "",
		})
		return repos
	}

	// Otherwise scan children
	entries, err := os.ReadDir(startDir)
	if err != nil {
		return repos
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		repoPath := filepath.Join(startDir, entry.Name())
		childGitDir := filepath.Join(repoPath, ".git")
		if info, err := os.Stat(childGitDir); err != nil || !info.IsDir() {
			continue
		}
		porcelain := git.StatusPorcelain(repoPath)
		repos = append(repos, repoInfo{
			name:     entry.Name(),
			repoPath: repoPath,
			dirty:    porcelain != "",
		})
	}

	return repos
}

func scanWorktrees(repos []repoInfo) []worktreeInfo {
	var worktrees []worktreeInfo

	for _, repo := range repos {
		for _, wtPath := range worktree.ListWorktrees(repo.repoPath) {
			branch := filepath.Base(wtPath)
			porcelain := git.StatusPorcelain(wtPath)
			worktrees = append(worktrees, worktreeInfo{
				repo:         repo.name,
				branch:       branch,
				repoPath:     repo.repoPath,
				worktreePath: wtPath,
				dirty:        porcelain != "",
			})
		}
	}

	return worktrees
}

func Run(startDir string, dirty bool, format string) error {
	tw := tap.NewWriter(os.Stdout)

	repos := scanRepos(startDir)
	worktrees := scanWorktrees(repos)

	if len(repos) == 0 && len(worktrees) == 0 {
		tw.Skip("pull", "no repos found")
		tw.Plan()
		return nil
	}

	var failed bool

	for _, repo := range repos {
		label := repo.name

		if repo.dirty && !dirty {
			tw.Skip("pull "+label, "dirty")
			continue
		}

		_, err := git.Pull(repo.repoPath)
		if err != nil {
			tw.NotOk("pull "+label, map[string]string{
				"message":  err.Error(),
				"severity": "fail",
			})
			failed = true
			continue
		}
		tw.Ok("pull " + label)
	}

	for _, wt := range worktrees {
		label := filepath.Join(wt.repo, worktree.WorktreesDir, wt.branch)

		if wt.dirty && !dirty {
			tw.Skip("rebase "+label, "dirty")
			continue
		}

		defaultBranch, err := git.DefaultBranch(wt.repoPath)
		if err != nil || defaultBranch == "" {
			tw.NotOk("rebase "+label, map[string]string{
				"message":  "could not determine default branch",
				"severity": "fail",
			})
			failed = true
			continue
		}

		_, err = git.Rebase(wt.worktreePath, defaultBranch)
		if err != nil {
			tw.NotOk("rebase "+label, map[string]string{
				"message":  err.Error(),
				"severity": "fail",
			})
			failed = true
			continue
		}
		tw.Ok("rebase " + label)
	}

	tw.Plan()

	if failed {
		return fmt.Errorf("one or more operations failed")
	}

	return nil
}
