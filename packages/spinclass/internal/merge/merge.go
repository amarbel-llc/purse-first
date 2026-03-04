package merge

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"

	"github.com/amarbel-llc/spinclass/internal/executor"
	"github.com/amarbel-llc/spinclass/internal/git"
	tap "github.com/amarbel-llc/purse-first/packages/tap-dancer/go"
	"github.com/amarbel-llc/spinclass/internal/worktree"
)

func Run(execr executor.Executor, format string, target string, gitSync bool, verbose bool) error {
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

	defaultBranch, err := resolveDefaultBranch(repoPath)
	if err != nil {
		return err
	}

	return Resolved(execr, os.Stdout, nil, format, repoPath, wtPath, branch, defaultBranch, gitSync, verbose)
}

func Resolved(execr executor.Executor, w io.Writer, tw *tap.Writer, format, repoPath, wtPath, branch, defaultBranch string, gitSync bool, verbose bool) error {
	if info, err := os.Stat(repoPath); err != nil || !info.IsDir() {
		return fmt.Errorf("repository not found: %s", repoPath)
	}

	if defaultBranch == "" {
		var err error
		defaultBranch, err = resolveDefaultBranch(repoPath)
		if err != nil {
			return err
		}
	}

	ownWriter := false
	if tw == nil && format == "tap" {
		tw = tap.NewWriter(w)
		ownWriter = true
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
			if ownWriter {
				tw.Plan()
			}
			return err
		}
		if verbose && out != "" {
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
			if ownWriter {
				tw.Plan()
			}
			return err
		}
		if verbose && out != "" {
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
			if ownWriter {
				tw.Plan()
			}
			return err
		}
		if verbose && out != "" {
			tw.OkDiag("remove worktree "+branch, &tap.Diagnostics{Extras: map[string]any{"output": out}})
		} else {
			tw.Ok("remove worktree " + branch)
		}
	} else {
		if err := git.RunPassthrough(repoPath, "worktree", "remove", wtPath); err != nil {
			return err
		}
	}

	if gitSync {
		if tw == nil {
			log.Info("pulling", "repo", repoPath)
		}

		if tw != nil {
			out, err := git.Pull(repoPath)
			if err != nil {
				diag := map[string]string{"severity": "fail", "message": err.Error()}
				if out != "" {
					diag["output"] = out
				}
				tw.NotOk("pull", diag)
				if ownWriter {
					tw.Plan()
				}
				return err
			}
			if verbose && out != "" {
				tw.OkDiag("pull", &tap.Diagnostics{Extras: map[string]any{"output": out}})
			} else {
				tw.Ok("pull")
			}
		} else {
			if err := git.RunPassthrough(repoPath, "pull"); err != nil {
				return err
			}
		}

		if tw == nil {
			log.Info("pushing", "repo", repoPath)
		}

		if tw != nil {
			out, err := git.Push(repoPath)
			if err != nil {
				diag := map[string]string{"severity": "fail", "message": err.Error()}
				if out != "" {
					diag["output"] = out
				}
				tw.NotOk("push", diag)
				if ownWriter {
					tw.Plan()
				}
				return err
			}
			if verbose && out != "" {
				tw.OkDiag("push", &tap.Diagnostics{Extras: map[string]any{"output": out}})
			} else {
				tw.Ok("push")
			}
		} else {
			if err := git.RunPassthrough(repoPath, "push"); err != nil {
				return err
			}
		}
	}

	if ownWriter {
		tw.Plan()
	}

	if tw == nil {
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

	var selected string
	options := make([]huh.Option[string], len(branches))
	for i, b := range branches {
		options[i] = huh.NewOption(b, b)
	}

	err = huh.NewSelect[string]().
		Title("Select worktree to merge").
		Options(options...).
		Value(&selected).
		Run()
	if err != nil {
		return "", "", fmt.Errorf("worktree selection cancelled")
	}

	for i, b := range branches {
		if b == selected {
			return paths[i], b, nil
		}
	}

	return "", "", fmt.Errorf("selected worktree not found: %s", selected)
}

func resolveDefaultBranch(repoPath string) (string, error) {
	branch, err := git.DefaultBranch(repoPath)
	if errors.Is(err, git.ErrAmbiguousDefaultBranch) {
		return promptDefaultBranch()
	}
	if err != nil {
		return "", fmt.Errorf("could not determine default branch: %w", err)
	}
	return branch, nil
}

func promptDefaultBranch() (string, error) {
	var selected string
	err := huh.NewSelect[string]().
		Title("Both main and master branches exist. Which should be the rebase target?").
		Options(
			huh.NewOption("main", "main"),
			huh.NewOption("master", "master"),
		).
		Value(&selected).
		Run()
	if err != nil {
		return "", fmt.Errorf("branch selection cancelled: %w", err)
	}
	return selected, nil
}
