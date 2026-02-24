package shop

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/amarbel-llc/spinclass/internal/executor"
	"github.com/amarbel-llc/spinclass/internal/flake"
	"github.com/amarbel-llc/spinclass/internal/git"
	"github.com/amarbel-llc/spinclass/internal/merge"
	"github.com/amarbel-llc/spinclass/internal/sweatfile"
	"github.com/amarbel-llc/spinclass/internal/tap"
	"github.com/amarbel-llc/spinclass/internal/worktree"
)

func createWorktree(rp worktree.ResolvedPath, verbose bool) error {
	if _, err := os.Stat(rp.AbsPath); os.IsNotExist(err) {
		result, err := worktree.Create(rp.RepoPath, rp.AbsPath)
		if err != nil {
			return err
		}
		if verbose {
			logSweatfileResult(result)
		}
	}

	return nil
}

func Create(rp worktree.ResolvedPath, verbose bool, format string) error {
	if err := createWorktree(rp, verbose); err != nil {
		return err
	}

	if format == "tap" {
		tw := tap.NewWriter(os.Stdout)
		tw.PlanAhead(1)
		tw.Ok("create " + rp.Branch)
	} else {
		fmt.Println(rp.AbsPath)
	}

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

func Attach(exec executor.Executor, rp worktree.ResolvedPath, format string, claudeArgs []string, noMerge bool) error {
	if err := createWorktree(rp, false); err != nil {
		return err
	}

	if format != "tap" {
		fmt.Println(rp.AbsPath)
	}

	var command []string
	if len(claudeArgs) > 0 {
		if flake.HasDevShell(rp.AbsPath) {
			log.Info("flake.nix detected, starting claude in nix develop")
			command = append([]string{"nix", "develop", "--command", "claude"}, claudeArgs...)
		} else {
			command = append([]string{"claude"}, claudeArgs...)
		}
	} else if flake.HasDevShell(rp.AbsPath) {
		log.Info("flake.nix detected, starting session in nix develop")
		command = []string{"nix", "develop", "--command", os.Getenv("SHELL")}
	}

	if err := exec.Attach(rp.AbsPath, rp.SessionKey, command); err != nil {
		return fmt.Errorf("attach failed: %w", err)
	}

	return closeShop(exec, rp, format, noMerge)
}

func closeShop(exec executor.Executor, rp worktree.ResolvedPath, format string, noMerge bool) error {
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
		return merge.Resolved(exec, format, rp.RepoPath, rp.AbsPath, rp.Branch)
	}

	commitsAhead := git.CommitsAhead(rp.AbsPath, defaultBranch, rp.Branch)
	desc := statusDescription(defaultBranch, commitsAhead, worktreeStatus)

	if format == "tap" {
		tw := tap.NewWriter(os.Stdout)
		tw.Ok("create " + rp.Branch)
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
