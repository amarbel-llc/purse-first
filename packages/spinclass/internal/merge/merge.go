package merge

import (
	"fmt"
	"os"

	"github.com/charmbracelet/log"

	"github.com/amarbel-llc/spinclass/internal/executor"
	"github.com/amarbel-llc/spinclass/internal/git"
	"github.com/amarbel-llc/spinclass/internal/tap"
)

func Run(exec executor.Executor, format string) error {
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

	defaultBranch, err := git.DefaultBranch(repoPath)
	if err != nil {
		return fmt.Errorf("could not determine default branch: %w", err)
	}

	var tw *tap.Writer
	if format == "tap" {
		tw = tap.NewWriter(os.Stdout)
	}

	if tw == nil {
		log.Info("rebasing onto "+defaultBranch, "worktree", branch)
	}

	if err := git.RunPassthroughEnv(cwd, []string{"GIT_SEQUENCE_EDITOR=true"}, "rebase", defaultBranch, "-i"); err != nil {
		if tw != nil {
			tw.NotOk("rebase "+branch, map[string]string{
				"message":  err.Error(),
				"severity": "fail",
			})
			tw.Plan()
		} else {
			log.Error("rebase failed, not merging")
		}
		return err
	}

	if tw != nil {
		tw.Ok("rebase " + branch)
	}

	if tw == nil {
		log.Info("merging worktree", "worktree", branch)
	}

	if err := git.RunPassthrough(repoPath, "merge", "--ff-only", branch); err != nil {
		if tw != nil {
			tw.NotOk("merge "+branch, map[string]string{
				"message":  err.Error(),
				"severity": "fail",
			})
			tw.Plan()
		} else {
			log.Error("merge failed, not removing worktree")
		}
		return err
	}

	if tw != nil {
		tw.Ok("merge " + branch)
	}

	if tw == nil {
		log.Info("removing worktree", "path", cwd)
	}

	if err := git.RunPassthrough(repoPath, "worktree", "remove", cwd); err != nil {
		if tw != nil {
			tw.NotOk("remove worktree "+branch, map[string]string{
				"message":  err.Error(),
				"severity": "fail",
			})
			tw.Plan()
		}
		return err
	}

	if tw != nil {
		tw.Ok("remove worktree " + branch)
		tw.Plan()
	} else {
		log.Info("detaching from session")
	}

	return exec.Detach()
}
