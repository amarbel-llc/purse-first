package status

import (
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"

	"github.com/amarbel-llc/spinclass/internal/executor"
	"github.com/amarbel-llc/spinclass/internal/git"
	tap "github.com/amarbel-llc/purse-first/packages/tap-dancer/go"
	"github.com/amarbel-llc/spinclass/internal/worktree"
)

type BranchStatus struct {
	Repo         string
	Branch       string
	Dirty        string
	Remote       string
	LastCommit   string
	LastModified string
	IsWorktree   bool
	Session      bool
}

type RepoStatus struct {
	Main      BranchStatus
	Worktrees []BranchStatus
}

func CollectBranchStatus(repoLabel, branchPath, branchName string) BranchStatus {
	bs := BranchStatus{
		Repo:   repoLabel,
		Branch: branchName,
	}

	porcelain := git.StatusPorcelain(branchPath)
	if porcelain != "" {
		bs.Dirty = parseDirtyStatus(porcelain)
	} else {
		bs.Dirty = "clean"
	}

	upstream := git.Upstream(branchPath)
	if upstream != "" {
		ahead, behind := git.RevListLeftRight(branchPath)
		var parts []string
		if ahead > 0 {
			parts = append(parts, fmt.Sprintf("↑%d", ahead))
		}
		if behind > 0 {
			parts = append(parts, fmt.Sprintf("↓%d", behind))
		}
		if len(parts) > 0 {
			bs.Remote = strings.Join(parts, " ") + " " + upstream
		} else {
			bs.Remote = "≡ " + upstream
		}
	}

	bs.LastCommit = git.LastCommitDate(branchPath)

	newest := git.NewestFileTime(branchPath)
	if !newest.IsZero() {
		bs.LastModified = newest.Format("2006-01-02")
	} else {
		bs.LastModified = "n/a"
	}

	return bs
}

func parseDirtyStatus(porcelain string) string {
	lines := strings.Split(porcelain, "\n")

	reModified := regexp.MustCompile(`^.M`)
	reAdded := regexp.MustCompile(`^A`)
	reDeleted := regexp.MustCompile(`^.D`)
	reUntracked := regexp.MustCompile(`^\?\?`)

	var modified, added, deleted, untracked int
	for _, line := range lines {
		if line == "" {
			continue
		}
		if reModified.MatchString(line) {
			modified++
		}
		if reAdded.MatchString(line) {
			added++
		}
		if reDeleted.MatchString(line) {
			deleted++
		}
		if reUntracked.MatchString(line) {
			untracked++
		}
	}

	var parts []string
	if modified > 0 {
		parts = append(parts, fmt.Sprintf("%dM", modified))
	}
	if added > 0 {
		parts = append(parts, fmt.Sprintf("%dA", added))
	}
	if deleted > 0 {
		parts = append(parts, fmt.Sprintf("%dD", deleted))
	}
	if untracked > 0 {
		parts = append(parts, fmt.Sprintf("%d?", untracked))
	}
	return strings.Join(parts, " ")
}

func CollectRepoStatus(repoPath string, sessions map[string]bool) RepoStatus {
	repoLabel := filepath.Base(repoPath)
	var rs RepoStatus

	mainBranch, err := git.BranchCurrent(repoPath)
	if err == nil && mainBranch != "" {
		rs.Main = CollectBranchStatus(repoLabel, repoPath, mainBranch)
		rs.Main.Session = sessions[repoLabel+"/"+mainBranch]
	}

	for _, wtPath := range worktree.ListWorktrees(repoPath) {
		branch := filepath.Base(wtPath)
		bs := CollectBranchStatus(repoLabel, wtPath, branch)
		bs.IsWorktree = true
		bs.Session = sessions[repoLabel+"/"+branch]
		rs.Worktrees = append(rs.Worktrees, bs)
	}

	return rs
}

func CollectStatus(startDir string) []RepoStatus {
	sessions := executor.ListSessions()
	var all []RepoStatus

	repos := worktree.ScanRepos(startDir)
	for _, repoPath := range repos {
		rs := CollectRepoStatus(repoPath, sessions)
		all = append(all, rs)
	}

	return all
}

func (bs BranchStatus) isClean() bool {
	return bs.Dirty == "clean" && (strings.HasPrefix(bs.Remote, "≡") || bs.Remote == "")
}

func renderTable(data [][]string) string {
	headers := []string{"Repo", "Branch", "Status", "Remote", "Commit", "Modified"}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("15"))).
		Headers(headers...).
		Rows(data...).
		StyleFunc(func(row, col int) lipgloss.Style {
			base := lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1)

			if row == table.HeaderRow {
				return base.Bold(true)
			}

			switch col {
			case 2: // Status
				val := data[row][col]
				if val == "clean" {
					return base.Foreground(lipgloss.Color("2"))
				}
				return base.Foreground(lipgloss.Color("1"))
			case 3: // Remote
				val := data[row][col]
				if strings.HasPrefix(val, "≡") {
					return base.Foreground(lipgloss.Color("2"))
				}
				if strings.Contains(val, "↑") || strings.Contains(val, "↓") {
					return base.Foreground(lipgloss.Color("3"))
				}
				return base.Foreground(lipgloss.Color("8"))
			}

			return base
		})

	return t.Render()
}

var styleHeader = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))

func Render(repos []RepoStatus) string {
	var rows []BranchStatus
	for _, rs := range repos {
		rows = append(rows, rs.Main)
		rows = append(rows, rs.Worktrees...)
	}

	var repoRows, worktreeRows, cleanRows [][]string
	for _, r := range rows {
		row := []string{r.Repo, r.Branch, r.Dirty, r.Remote, r.LastCommit, r.LastModified}
		if r.isClean() {
			cleanRows = append(cleanRows, row)
		} else if r.IsWorktree {
			worktreeRows = append(worktreeRows, row)
		} else {
			repoRows = append(repoRows, row)
		}
	}

	var sections []string
	if len(repoRows) > 0 {
		sections = append(sections, styleHeader.Render("Repos")+"\n"+renderTable(repoRows))
	}
	if len(worktreeRows) > 0 {
		sections = append(sections, styleHeader.Render("Worktrees")+"\n"+renderTable(worktreeRows))
	}
	if len(cleanRows) > 0 {
		sections = append(sections, styleHeader.Render("Clean")+"\n"+renderTable(cleanRows))
	}

	return strings.Join(sections, "\n\n")
}

func RenderTap(repos []RepoStatus, w io.Writer) {
	tw := tap.NewWriter(w)
	for _, rs := range repos {
		tw.Ok(rs.Main.Repo + " " + rs.Main.Branch)
		for _, wt := range rs.Worktrees {
			tw.Ok(wt.Repo + " " + wt.Branch)
		}
	}
	tw.Plan()
}
