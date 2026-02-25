package shop

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/amarbel-llc/spinclass/internal/executor"
	"github.com/amarbel-llc/spinclass/internal/git"
	"github.com/amarbel-llc/spinclass/internal/merge"
	"github.com/amarbel-llc/spinclass/internal/sweatfile"
	tap "github.com/amarbel-llc/tap-dancer/go"
	"github.com/amarbel-llc/spinclass/internal/worktree"
)

func createWorktree(rp worktree.ResolvedPath, verbose bool) (bool, error) {
	existed := true

	if _, err := os.Stat(rp.AbsPath); os.IsNotExist(err) {
		existed = false
		result, err := worktree.Create(rp.RepoPath, rp.AbsPath)
		if err != nil {
			return false, err
		}
		if verbose {
			logSweatfileResult(result)
		}
	}

	return existed, nil
}

func Create(w io.Writer, rp worktree.ResolvedPath, verbose bool, format string, tw *tap.Writer) error {
	existed, err := createWorktree(rp, verbose)
	if err != nil {
		return err
	}

	if format == "tap" {
		if tw == nil {
			tw = tap.NewWriter(w)
			tw.PlanAhead(1)
		}
		if existed {
			tw.Skip("create "+rp.Branch, "already exists "+rp.AbsPath)
		} else {
			tw.Ok("create " + rp.Branch + " " + rp.AbsPath)
		}
		return nil
	}

	fmt.Fprintln(w, rp.AbsPath)
	return nil
}

func logSweatfileResult(result sweatfile.LoadResult) {
	for _, src := range result.Sources {
		if src.Found {
			log.Info("loaded sweatfile", "path", src.Path)
			if len(src.File.GitExcludes) > 0 {
				log.Info("  git_excludes", "values", src.File.GitExcludes)
			}
			if len(src.File.ClaudeAllow) > 0 {
				log.Info("  claude_allow", "values", src.File.ClaudeAllow)
			}
		} else {
			log.Info("sweatfile not found (skipped)", "path", src.Path)
		}
	}
	merged := result.Merged
	log.Info("merged sweatfile",
		"git_excludes", merged.GitExcludes,
		"claude_allow", merged.ClaudeAllow,
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

func New(w io.Writer, exec executor.Executor, rp worktree.ResolvedPath, format string, claudeArgs []string, noMerge bool, noAttach bool, verbose bool) error {
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

	var command []string
	if len(claudeArgs) > 0 {
		command = append([]string{"claude"}, claudeArgs...)
	}

	tp := tap.TestPoint{
		Description: "attach " + rp.Branch,
		Ok:          true,
	}

	if err := exec.Attach(rp.AbsPath, rp.SessionKey, command, noAttach, &tp); err != nil {
		return fmt.Errorf("attach failed: %w", err)
	}

	if noAttach {
		if tw != nil {
			tw.SkipDiag(tp.Description, tp.Skip, tp.Diagnostics)
			tw.Plan()
		}
		return nil
	}

	return closeShop(w, exec, rp, format, noMerge, verbose, tw)
}

func closeShop(w io.Writer, exec executor.Executor, rp worktree.ResolvedPath, format string, noMerge bool, verbose bool, tw *tap.Writer) error {
	if rp.Branch == "" {
		if err := rp.FillBranchFromGit(); err != nil {
			log.Warn("could not determine current branch")
			return nil
		}
	}

	defaultBranch, err := git.BranchCurrent(rp.RepoPath)
	if err != nil || defaultBranch == "" {
		log.Warn("could not determine default branch")
		return nil
	}

	worktreeStatus := git.StatusPorcelain(rp.AbsPath)
	isClean := worktreeStatus == ""

	if isClean && !noMerge {
		err := merge.Resolved(exec, w, tw, format, rp.RepoPath, rp.AbsPath, rp.Branch, verbose)
		if tw != nil {
			tw.Plan()
		}
		return err
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
func Fork(w io.Writer, rp worktree.ResolvedPath, newBranch string, format string) error {
	if newBranch == "" {
		newBranch = worktree.ForkName(rp.RepoPath, rp.Branch)
	}

	newPath := filepath.Join(rp.RepoPath, worktree.WorktreesDir, newBranch)

	if _, err := worktree.CreateFrom(rp.RepoPath, rp.AbsPath, newPath, newBranch); err != nil {
		return err
	}

	if format == "tap" {
		tw := tap.NewWriter(w)
		tw.PlanAhead(1)
		tw.Ok("fork " + newBranch + " " + newPath)
		return nil
	}

	fmt.Fprintln(w, newPath)
	return nil
}
