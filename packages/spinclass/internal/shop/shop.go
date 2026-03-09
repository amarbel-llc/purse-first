package shop

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/mattn/go-isatty"

	"github.com/amarbel-llc/spinclass/internal/executor"
	"github.com/amarbel-llc/spinclass/internal/git"
	"github.com/amarbel-llc/spinclass/internal/merge"
	"github.com/amarbel-llc/spinclass/internal/sweatfile"
	"github.com/amarbel-llc/spinclass/internal/worktree"
	tap "github.com/amarbel-llc/purse-first/packages/tap-dancer/go"
)

func Create(
	writer io.Writer,
	worktreePath worktree.ResolvedPath,
	verbose bool,
	format string,
	tapWriter *tap.Writer,
) error {
	existed, err := createWorktree(worktreePath, verbose)
	if err != nil {
		return err
	}

	if format == "tap" {
		if tapWriter == nil {
			tapWriter = tap.NewWriter(writer)
			tapWriter.PlanAhead(1)
		}
		if existed {
			tapWriter.Skip("create "+worktreePath.Branch, "already exists "+worktreePath.AbsPath)
		} else {
			tapWriter.Ok("create " + worktreePath.Branch + " " + worktreePath.AbsPath)
		}
		return nil
	}

	fmt.Fprintln(writer, worktreePath.AbsPath)
	return nil
}

func createWorktree(worktreePath worktree.ResolvedPath, verbose bool) (bool, error) {
	existed := true

	if _, err := os.Stat(worktreePath.AbsPath); os.IsNotExist(err) {
		existed = false
		result, err := worktree.Create(worktreePath.RepoPath, worktreePath.AbsPath, worktreePath.ExistingBranch)
		if err != nil {
			return false, err
		}
		if verbose {
			logSweatfileResult(result)
		}
	}

	return existed, nil
}

func logSweatfileResult(result sweatfile.Hierarchy) {
	for _, src := range result.Sources {
		if src.Found {
			log.Info("loaded sweatfile", "path", src.Path)
			if len(src.File.GitSkipIndex) > 0 {
				log.Info("  git-excludes", "values", src.File.GitSkipIndex)
			}
			if len(src.File.ClaudeAllow) > 0 {
				log.Info("  claude-allow", "values", src.File.ClaudeAllow)
			}
		} else {
			log.Info("sweatfile not found (skipped)", "path", src.Path)
		}
	}
	merged := result.Merged
	log.Info("merged sweatfile",
		"git-excludes", merged.GitSkipIndex,
		"claude-allow", merged.ClaudeAllow,
	)
}

func pullMainWorktree(rp worktree.ResolvedPath, tw *tap.Writer) error {
	label := "pull " + filepath.Base(rp.RepoPath)

	if git.Upstream(rp.RepoPath) == "" {
		if tw != nil {
			tw.Skip(label, "no upstream")
		}
		return nil
	}

	porcelain := git.StatusPorcelain(rp.RepoPath)
	if porcelain != "" {
		if tw != nil {
			tw.Skip(label, "dirty")
		}
		return nil
	}

	_, err := git.Pull(rp.RepoPath)
	if err != nil {
		if tw != nil {
			tw.NotOk(label, map[string]string{
				"message":  err.Error(),
				"severity": "fail",
			})
		}
		return err
	}

	if tw != nil {
		tw.Ok(label)
	}

	return nil
}

func New(w io.Writer, exec executor.Executor, rp worktree.ResolvedPath, format string, mergeOnClose bool, noAttach bool, verbose bool) error {
	var tw *tap.Writer
	if format == "tap" {
		tw = tap.NewWriter(w)
	}

	if err := pullMainWorktree(rp, tw); err != nil {
		return err
	}

	if err := Create(w, rp, verbose, format, tw); err != nil {
		return err
	}

	tp := tap.TestPoint{
		Description: "attach " + rp.Branch,
		Ok:          true,
	}

	if err := exec.Attach(rp.AbsPath, rp.SessionKey, nil, noAttach, &tp); err != nil {
		return fmt.Errorf("attach failed: %w", err)
	}

	if noAttach {
		if tw != nil {
			tw.SkipDiag(tp.Description, tp.Skip, tp.Diagnostics)
			tw.Plan()
		}
		return nil
	}

	interactive := isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())

	return closeShop(w, exec, rp, format, mergeOnClose, verbose, tw, interactive, noAttach)
}

func closeShop(w io.Writer, exec executor.Executor, rp worktree.ResolvedPath, format string, mergeOnClose bool, verbose bool, tw *tap.Writer, interactive bool, noAttach bool) error {
	if rp.Branch == "" {
		if err := rp.FillBranchFromGit(); err != nil {
			log.Warn("could not determine current branch")
			return nil
		}
	}

	defaultBranch, err := git.DefaultBranch(rp.RepoPath)
	if errors.Is(err, git.ErrAmbiguousDefaultBranch) {
		if interactive {
			defaultBranch, err = promptDefaultBranch()
			if err != nil {
				log.Warn("branch selection cancelled")
				return nil
			}
		} else {
			log.Warn("both main and master branches exist, skipping rebase")
			return nil
		}
	} else if err != nil || defaultBranch == "" {
		log.Warn("could not determine default branch")
		return nil
	}

	worktreeStatus := git.StatusPorcelain(rp.AbsPath)
	isClean := worktreeStatus == ""

	if isClean && mergeOnClose {
		err := merge.Resolved(exec, w, tw, format, rp.RepoPath, rp.AbsPath, rp.Branch, defaultBranch, false, verbose)
		if tw != nil {
			tw.Plan()
		}
		return err
	}

	if interactive && mergeOnClose {
		for {
			action, promptErr := promptDirtyAction(rp.Branch)
			if promptErr != nil {
				break
			}

			switch action {
			case actionDiscard:
				if discardErr := discardAll(rp.AbsPath); discardErr != nil {
					if tw != nil {
						tw.NotOk("discard "+rp.Branch, map[string]string{
							"severity": "fail",
							"message":  discardErr.Error(),
						})
						tw.Plan()
					}
					return discardErr
				}
				mergeErr := merge.Resolved(exec, w, tw, format, rp.RepoPath, rp.AbsPath, rp.Branch, defaultBranch, false, verbose)
				if tw != nil {
					tw.Plan()
				}
				return mergeErr

			case actionReattach:
				tp := tap.TestPoint{
					Description: "reattach " + rp.Branch,
					Ok:          true,
				}
				if attachErr := exec.Attach(rp.AbsPath, rp.SessionKey, nil, noAttach, &tp); attachErr != nil {
					return fmt.Errorf("reattach failed: %w", attachErr)
				}
				worktreeStatus = git.StatusPorcelain(rp.AbsPath)
				isClean = worktreeStatus == ""
				if isClean {
					mergeErr := merge.Resolved(exec, w, tw, format, rp.RepoPath, rp.AbsPath, rp.Branch, defaultBranch, false, verbose)
					if tw != nil {
						tw.Plan()
					}
					return mergeErr
				}
				continue

			case actionExit:
			}
			break
		}
	}

	commitsAhead := git.CommitsAhead(rp.AbsPath, defaultBranch, rp.Branch)
	desc := statusDescription(defaultBranch, commitsAhead, worktreeStatus)

	if tw != nil {
		tw.Ok("close " + rp.Branch + " # " + desc)
		tw.Plan()
	} else if format == "tap" {
		tw = tap.NewWriter(w)
		tw.Ok("close " + rp.Branch + " # " + desc)
		tw.Plan()
	} else {
		log.Info(desc, "worktree", rp.SessionKey)
	}

	return nil
}

func discardAll(wtPath string) error {
	if _, err := git.Run(wtPath, "checkout", "."); err != nil {
		return fmt.Errorf("git checkout: %w", err)
	}
	if _, err := git.Run(wtPath, "clean", "-fd"); err != nil {
		return fmt.Errorf("git clean: %w", err)
	}
	return nil
}

func statusDescription(defaultBranch string, commitsAhead int, porcelain string) string {
	var parts []string

	if commitsAhead == 1 {
		parts = append(parts, fmt.Sprintf("1 commit ahead of %s", defaultBranch))
	} else {
		parts = append(parts, fmt.Sprintf("%d commits ahead of %s", commitsAhead, defaultBranch))
	}

	if porcelain == "" {
		parts = append(parts, "clean")
	} else {
		parts = append(parts, "dirty")
	}

	if commitsAhead == 0 && porcelain == "" {
		parts = append(parts, "(merged)")
	}

	return strings.Join(parts, ", ")
}

// Fork creates a new worktree branched from rp's current HEAD.
// If newBranch is empty, a name is auto-generated as <rp.Branch>-N.
// Does not attach to the new session.
func Fork(
	writer io.Writer,
	worktreePath worktree.ResolvedPath,
	newBranch string,
	format string,
) error {
	if newBranch == "" {
		newBranch = worktree.ForkName(worktreePath.RepoPath, worktreePath.Branch)
	}

	newPath := filepath.Join(worktreePath.RepoPath, worktree.WorktreesDir, newBranch)

	if _, err := worktree.CreateFrom(
		worktreePath.RepoPath,
		worktreePath.AbsPath,
		newPath,
		newBranch,
	); err != nil {
		return err
	}

	if format == "tap" {
		tw := tap.NewWriter(writer)
		tw.PlanAhead(1)
		tw.Ok("fork " + newBranch + " " + newPath)
		return nil
	}

	fmt.Fprintln(writer, newPath)
	return nil
}
