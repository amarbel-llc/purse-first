package status

import (
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"

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

var (
	styleRepo        = lipgloss.NewStyle().Bold(true)
	styleDirty       = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleClean       = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleRemoteSync  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleRemoteDrift = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleRemoteNone  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleSession     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleDim         = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

type renderRow struct {
	prefix   string
	branch   string
	dirty    string
	remote   string
	commit   string
	modified string
	session  string
}

func collectRenderRows(repos []RepoStatus) []renderRow {
	var rows []renderRow
	for _, rs := range repos {
		mainSession := ""
		if rs.Main.Session {
			mainSession = "● zmx"
		}
		rows = append(rows, renderRow{
			prefix:   rs.Main.Repo,
			branch:   rs.Main.Branch,
			dirty:    rs.Main.Dirty,
			remote:   rs.Main.Remote,
			commit:   rs.Main.LastCommit,
			modified: rs.Main.LastModified,
			session:  mainSession,
		})

		for i, wt := range rs.Worktrees {
			connector := "├"
			if i == len(rs.Worktrees)-1 {
				connector = "└"
			}
			session := ""
			if wt.Session {
				session = "● zmx"
			}
			rows = append(rows, renderRow{
				prefix:   "  " + connector + " ",
				branch:   wt.Branch,
				dirty:    wt.Dirty,
				remote:   wt.Remote,
				commit:   wt.LastCommit,
				modified: wt.LastModified,
				session:  session,
			})
		}
	}
	return rows
}

func padRight(s string, displayWidth int) string {
	w := lipgloss.Width(s)
	if w >= displayWidth {
		return s
	}
	return s + strings.Repeat(" ", displayWidth-w)
}

func Render(repos []RepoStatus) string {
	rows := collectRenderRows(repos)
	if len(rows) == 0 {
		return ""
	}

	// Calculate column widths using display width (not byte length)
	// to handle multi-byte Unicode characters like ├, └, ≡, ↑, ↓, ●
	widths := [7]int{}
	for _, r := range rows {
		cols := [7]string{r.prefix, r.branch, r.dirty, r.remote, r.commit, r.modified, r.session}
		for i, c := range cols {
			if w := lipgloss.Width(c); w > widths[i] {
				widths[i] = w
			}
		}
	}

	var lines []string
	for _, r := range rows {
		prefix := padRight(r.prefix, widths[0])
		branch := padRight(r.branch, widths[1])
		commit := padRight(r.commit, widths[4])
		modified := padRight(r.modified, widths[5])

		dirtyPad := padRight(r.dirty, widths[2])
		var styledDirty string
		if r.dirty == "clean" {
			styledDirty = styleClean.Render(dirtyPad)
		} else {
			styledDirty = styleDirty.Render(dirtyPad)
		}

		remotePad := padRight(r.remote, widths[3])
		var styledRemote string
		if strings.HasPrefix(r.remote, "≡") {
			styledRemote = styleRemoteSync.Render(remotePad)
		} else if strings.Contains(r.remote, "↑") || strings.Contains(r.remote, "↓") {
			styledRemote = styleRemoteDrift.Render(remotePad)
		} else {
			styledRemote = styleRemoteNone.Render(remotePad)
		}

		sessionPad := padRight(r.session, widths[6])
		var styledSession string
		if r.session != "" {
			styledSession = styleSession.Render(sessionPad)
		} else {
			styledSession = sessionPad
		}

		var styledPrefix string
		if strings.Contains(r.prefix, "├") || strings.Contains(r.prefix, "└") {
			styledPrefix = styleDim.Render(prefix)
		} else {
			styledPrefix = styleRepo.Render(prefix)
		}

		line := styledPrefix + "  " + branch + "  " + styledDirty + "  " +
			styledRemote + "  " + commit + "  " + modified
		if r.session != "" {
			line += "  " + styledSession
		}
		lines = append(lines, strings.TrimRight(line, " "))
	}

	return strings.Join(lines, "\n")
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
