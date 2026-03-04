package clean

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"

	"github.com/amarbel-llc/spinclass/internal/git"
	tap "github.com/amarbel-llc/purse-first/packages/tap-dancer/go"
	"github.com/amarbel-llc/spinclass/internal/worktree"
)

var styleCode = lipgloss.NewStyle().Foreground(lipgloss.Color("#E88388")).Background(lipgloss.Color("#1D1F21")).Padding(0, 1)

type FileChange struct {
	Code string
	Path string
}

func ParsePorcelain(porcelain string) []FileChange {
	var changes []FileChange
	for _, line := range strings.Split(porcelain, "\n") {
		if len(line) < 4 {
			continue
		}
		code := line[:2]
		path := line[3:]
		if idx := strings.Index(path, " -> "); idx >= 0 {
			path = path[idx+4:]
		}
		changes = append(changes, FileChange{Code: code, Path: path})
	}
	return changes
}

func (fc FileChange) Description() string {
	switch {
	case fc.Code == "??":
		return "untracked"
	case fc.Code[1] == 'D' || fc.Code[0] == 'D':
		return "deleted"
	case fc.Code[0] == 'A':
		return "added"
	case fc.Code[0] == 'R':
		return "renamed"
	default:
		return "modified"
	}
}

type worktreeInfo struct {
	repo         string
	branch       string
	repoPath     string
	worktreePath string
	merged       bool
	dirty        bool
}

func scanWorktrees(startDir string) []worktreeInfo {
	var worktrees []worktreeInfo

	repos := worktree.ScanRepos(startDir)
	for _, repoPath := range repos {
		repoName := filepath.Base(repoPath)

		defaultBranch, err := git.DefaultBranch(repoPath)
		if err != nil || defaultBranch == "" {
			continue
		}

		for _, wtPath := range worktree.ListWorktrees(repoPath) {
			branch := filepath.Base(wtPath)

			ahead := git.CommitsAhead(wtPath, defaultBranch, branch)
			porcelain := git.StatusPorcelain(wtPath)

			worktrees = append(worktrees, worktreeInfo{
				repo:         repoName,
				branch:       branch,
				repoPath:     repoPath,
				worktreePath: wtPath,
				merged:       ahead == 0,
				dirty:        porcelain != "",
			})
		}
	}

	return worktrees
}

func removeWorktree(wt worktreeInfo) error {
	if err := git.WorktreeRemove(wt.repoPath, wt.worktreePath); err != nil {
		return fmt.Errorf("removing worktree %s: %w", wt.branch, err)
	}
	if err := git.BranchDelete(wt.repoPath, wt.branch); err != nil {
		return fmt.Errorf("deleting branch %s: %w", wt.branch, err)
	}
	return nil
}

func discardFile(wtPath string, fc FileChange) error {
	if fc.Code == "??" {
		return os.Remove(filepath.Join(wtPath, fc.Path))
	}
	if fc.Code[0] != ' ' {
		if err := git.ResetFile(wtPath, fc.Path); err != nil {
			return err
		}
	}
	return git.CheckoutFile(wtPath, fc.Path)
}

func handleDirtyWorktree(wt worktreeInfo) (removed bool, err error) {
	porcelain := git.StatusPorcelain(wt.worktreePath)
	changes := ParsePorcelain(porcelain)

	for _, fc := range changes {
		var discard bool
		prompt := fmt.Sprintf("Discard %s (%s)?", fc.Path, fc.Description())
		err := huh.NewConfirm().
			Title(prompt).
			Value(&discard).
			Run()
		if err != nil {
			return false, err
		}
		if discard {
			if err := discardFile(wt.worktreePath, fc); err != nil {
				log.Warn("failed to discard file", "file", fc.Path, "err", err)
			}
		}
	}

	recheckPorcelain := git.StatusPorcelain(wt.worktreePath)
	if recheckPorcelain != "" {
		return false, nil
	}

	if err := removeWorktree(wt); err != nil {
		return false, err
	}
	return true, nil
}

func Run(startDir string, interactive bool, format string) error {
	var tw *tap.Writer
	if format == "tap" {
		tw = tap.NewWriter(os.Stdout)
	}

	worktrees := scanWorktrees(startDir)
	if len(worktrees) == 0 {
		if tw != nil {
			tw.Skip("clean", "no worktrees found")
			tw.Plan()
		} else {
			log.Info("no worktrees found")
		}
		return nil
	}

	for _, wt := range worktrees {
		if !wt.merged {
			continue
		}

		label := filepath.Join(wt.repo, worktree.WorktreesDir) + "/" + styleCode.Render(wt.branch)

		if !wt.dirty {
			if err := removeWorktree(wt); err != nil {
				if tw != nil {
					tw.NotOk("remove "+label, map[string]string{
						"error": err.Error(),
					})
				} else {
					log.Error("failed to remove worktree", "branch", wt.branch, "error", err)
				}
				continue
			}
			if tw != nil {
				tw.Ok("remove " + label)
			} else {
				log.Info("removed merged worktree", "branch", wt.branch)
			}
			continue
		}

		if interactive {
			wasRemoved, err := handleDirtyWorktree(wt)
			if err != nil {
				if tw != nil {
					tw.NotOk("remove "+label, map[string]string{
						"error": err.Error(),
					})
				} else {
					log.Error("failed to remove worktree", "branch", wt.branch, "error", err)
				}
				continue
			}
			if wasRemoved {
				if tw != nil {
					tw.Ok("remove " + label)
				} else {
					log.Info("removed merged worktree", "branch", wt.branch)
				}
			} else {
				if tw != nil {
					tw.Skip("remove "+label, "kept after interactive review")
				} else {
					log.Info("kept worktree after interactive review", "branch", wt.branch)
				}
			}
		} else {
			if tw != nil {
				tw.Skip("remove "+label, "dirty worktree")
			} else {
				log.Warn("skipping dirty worktree", "branch", wt.branch)
			}
		}
	}

	if tw != nil {
		tw.Plan()
	}
	return nil
}
