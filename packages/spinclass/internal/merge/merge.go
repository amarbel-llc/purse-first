package merge

import (
	"fmt"
	"os"

	"github.com/charmbracelet/log"

	"github.com/amarbel-llc/spinclass/internal/executor"
	"github.com/amarbel-llc/spinclass/internal/git"
)

func Run(exec executor.Executor) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	repoPath, err := git.CommonDir(cwd)
	if err != nil {
		return fmt.Errorf("not in a worktree directory: %s", cwd)
	}

	branch, err := git.BranchCurrent(cwd)
	if err != nil {
		return fmt.Errorf("could not determine current branch: %w", err)
	}

	if info, err := os.Stat(repoPath); err != nil || !info.IsDir() {
		return fmt.Errorf("repository not found: %s", repoPath)
	}

	log.Info("merging worktree", "worktree", branch)

	if err := git.RunPassthrough(repoPath, "merge", "--no-ff", branch, "-m", "Merge worktree: "+branch); err != nil {
		log.Error("merge failed, not removing worktree")
		return err
	}

	log.Info("removing worktree", "path", cwd)
	if err := git.RunPassthrough(repoPath, "worktree", "remove", cwd); err != nil {
		return err
	}

	log.Info("detaching from session")
	return exec.Detach()
}
