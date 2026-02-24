package merge

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/amarbel-llc/spinclass/internal/executor"
	"github.com/amarbel-llc/spinclass/internal/git"
	"github.com/amarbel-llc/spinclass/internal/tap"
	"github.com/amarbel-llc/spinclass/internal/worktree"
)

func Run(execr executor.Executor, format string, target string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	var repoPath, wtPath, branch string

	if worktree.IsWorktree(cwd) && target == "" {
		repoPath, err = git.CommonDir(cwd)
		if err != nil {
			return fmt.Errorf("not in a worktree directory: %s", cwd)
		}
		wtPath = cwd
		branch, err = git.BranchCurrent(cwd)
		if err != nil {
			return fmt.Errorf("could not determine current branch: %w", err)
		}
	} else {
		if worktree.IsWorktree(cwd) {
			repoPath, err = git.CommonDir(cwd)
		} else {
			repoPath, err = worktree.DetectRepo(cwd)
		}
		if err != nil {
			return fmt.Errorf("not in a git repository: %s", cwd)
		}

		if target != "" {
			wtPath, branch, err = resolveWorktree(repoPath, target)
		} else {
			wtPath, branch, err = chooseWorktree(repoPath)
		}
		if err != nil {
			return err
		}
	}

	return Resolved(execr, format, repoPath, wtPath, branch)
}

func Resolved(execr executor.Executor, format, repoPath, wtPath, branch string) error {
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

	if err := git.RunPassthroughEnv(wtPath, []string{"GIT_SEQUENCE_EDITOR=true"}, "rebase", defaultBranch, "-i"); err != nil {
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
		log.Info("removing worktree", "path", wtPath)
	}

	if err := git.RunPassthrough(repoPath, "worktree", "remove", wtPath); err != nil {
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

	return execr.Detach()
}

func resolveWorktree(repoPath, target string) (wtPath, branch string, err error) {
	paths := worktree.ListWorktrees(repoPath)
	for _, p := range paths {
		if filepath.Base(p) == target {
			return p, target, nil
		}
	}
	return "", "", fmt.Errorf("worktree not found: %s", target)
}

func chooseWorktree(repoPath string) (wtPath, branch string, err error) {
	paths := worktree.ListWorktrees(repoPath)
	if len(paths) == 0 {
		return "", "", fmt.Errorf("no worktrees found in %s", repoPath)
	}

	branches := make([]string, len(paths))
	for i, p := range paths {
		branches[i] = filepath.Base(p)
	}

	cmd := exec.Command("gum", "choose")
	cmd.Args = append(cmd.Args, branches...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("worktree selection cancelled")
	}

	selected := strings.TrimSpace(string(out))
	if selected == "" {
		return "", "", fmt.Errorf("no worktree selected")
	}

	for i, b := range branches {
		if b == selected {
			return paths[i], b, nil
		}
	}

	return "", "", fmt.Errorf("selected worktree not found: %s", selected)
}
