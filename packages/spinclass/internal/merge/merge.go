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
	tap "github.com/amarbel-llc/tap-dancer/go"
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

	if tw != nil {
		out, err := git.RunEnv(wtPath, []string{"GIT_SEQUENCE_EDITOR=true"}, "rebase", defaultBranch, "-i")
		if err != nil {
			diag := map[string]string{"severity": "fail", "message": err.Error()}
			if out != "" {
				diag["output"] = out
			}
			tw.NotOk("rebase "+branch, diag)
			tw.Plan()
			return err
		}
		if out != "" {
			tw.OkDiag("rebase "+branch, &tap.Diagnostics{Extras: map[string]any{"output": out}})
		} else {
			tw.Ok("rebase " + branch)
		}
	} else {
		if err := git.RunPassthroughEnv(wtPath, []string{"GIT_SEQUENCE_EDITOR=true"}, "rebase", defaultBranch, "-i"); err != nil {
			log.Error("rebase failed, not merging")
			return err
		}
	}

	if tw == nil {
		log.Info("merging worktree", "worktree", branch)
	}

	if tw != nil {
		out, err := git.Run(repoPath, "merge", "--ff-only", branch)
		if err != nil {
			diag := map[string]string{"severity": "fail", "message": err.Error()}
			if out != "" {
				diag["output"] = out
			}
			tw.NotOk("merge "+branch, diag)
			tw.Plan()
			return err
		}
		if out != "" {
			tw.OkDiag("merge "+branch, &tap.Diagnostics{Extras: map[string]any{"output": out}})
		} else {
			tw.Ok("merge " + branch)
		}
	} else {
		if err := git.RunPassthrough(repoPath, "merge", "--ff-only", branch); err != nil {
			log.Error("merge failed, not removing worktree")
			return err
		}
	}

	if tw == nil {
		log.Info("removing worktree", "path", wtPath)
	}

	if tw != nil {
		out, err := git.Run(repoPath, "worktree", "remove", wtPath)
		if err != nil {
			diag := map[string]string{"severity": "fail", "message": err.Error()}
			if out != "" {
				diag["output"] = out
			}
			tw.NotOk("remove worktree "+branch, diag)
			tw.Plan()
			return err
		}
		if out != "" {
			tw.OkDiag("remove worktree "+branch, &tap.Diagnostics{Extras: map[string]any{"output": out}})
		} else {
			tw.Ok("remove worktree " + branch)
		}
		tw.Plan()
	} else {
		if err := git.RunPassthrough(repoPath, "worktree", "remove", wtPath); err != nil {
			return err
		}
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
