package status

import (
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"

	"github.com/amarbel-llc/spinclass/internal/git"
	"github.com/amarbel-llc/spinclass/internal/tap"
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

func CollectRepoStatus(repoPath string) []BranchStatus {
	repoLabel := filepath.Base(repoPath)
	var rows []BranchStatus

	mainBranch, err := git.BranchCurrent(repoPath)
	if err == nil && mainBranch != "" {
		rows = append(rows, CollectBranchStatus(repoLabel, repoPath, mainBranch))
	}

	for _, wtPath := range worktree.ListWorktrees(repoPath) {
		branch := filepath.Base(wtPath)
		bs := CollectBranchStatus(repoLabel, wtPath, branch)
		bs.IsWorktree = true
		rows = append(rows, bs)
	}

	return rows
}

func CollectStatus(startDir string) []BranchStatus {
	var all []BranchStatus

	repos := worktree.ScanRepos(startDir)
	for _, repoPath := range repos {
		rows := CollectRepoStatus(repoPath)
		all = append(all, rows...)
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

var (
	styleHeader = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	styleCode   = lipgloss.NewStyle().Foreground(lipgloss.Color("#E88388")).Background(lipgloss.Color("#1D1F21")).Padding(0, 1)
)

func Render(rows []BranchStatus) string {
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

func RenderTap(rows []BranchStatus, w io.Writer) {
	tw := tap.NewWriter(w)
	for _, r := range rows {
		desc := r.Repo + " " + styleCode.Render(r.Branch)
		tw.Ok(desc)
	}
	tw.Plan()
}
