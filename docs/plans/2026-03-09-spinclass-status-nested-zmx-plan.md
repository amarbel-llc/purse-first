# Spinclass Status: Nested Worktrees + zmx Sessions — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Restructure `spinclass status` output so worktrees appear nested under their parent repos with tree connectors, and add a column showing active zmx sessions.

**Architecture:** Replace flat `[]BranchStatus` with grouped `[]RepoStatus`. Add `ListSessions()` to parse `zmx -g sc list` output into a session key set. Replace lipgloss table rendering with tree-style text using aligned columns and `├`/`└` connectors. Color stays via lipgloss styles on individual values.

**Tech Stack:** Go, lipgloss (styles only, not table widget), zmx CLI

---

### Task 1: Add `ListSessions` to executor package

**Files:**
- Create: `packages/spinclass/internal/executor/sessions.go`
- Test: `packages/spinclass/internal/executor/sessions_test.go`

**Step 1: Write the failing test**

In `packages/spinclass/internal/executor/sessions_test.go`:

```go
package executor

import "testing"

func TestParseSessionsMultipleLines(t *testing.T) {
	input := "  session_name=finicky/rich-magnolia\tstatus=Unexpected\t(cleaning up)\n" +
		"→ session_name=purse-first/fresh-oak\tstatus=Unexpected\t(cleaning up)\n" +
		"  session_name=purse-first/ready-sycamore\tstatus=Unexpected\t(cleaning up)\n"
	sessions := parseSessions(input)
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}
	for _, key := range []string{"finicky/rich-magnolia", "purse-first/fresh-oak", "purse-first/ready-sycamore"} {
		if !sessions[key] {
			t.Errorf("expected session %q to be present", key)
		}
	}
}

func TestParseSessionsEmpty(t *testing.T) {
	sessions := parseSessions("")
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestParseSessionsNoMatch(t *testing.T) {
	sessions := parseSessions("some garbage output\nanother line\n")
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `nix develop --command go test -run TestParseSessions ./packages/spinclass/internal/executor/...`
Expected: FAIL — `parseSessions` undefined

**Step 3: Write minimal implementation**

In `packages/spinclass/internal/executor/sessions.go`:

```go
package executor

import (
	"os/exec"
	"strings"
)

func parseSessions(output string) map[string]bool {
	sessions := make(map[string]bool)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// line may start with "→ " for current session
		line = strings.TrimPrefix(line, "→ ")
		line = strings.TrimSpace(line)
		for _, field := range strings.Split(line, "\t") {
			field = strings.TrimSpace(field)
			if strings.HasPrefix(field, "session_name=") {
				key := strings.TrimPrefix(field, "session_name=")
				if key != "" {
					sessions[key] = true
				}
			}
		}
	}
	return sessions
}

func ListSessions() map[string]bool {
	cmd := exec.Command("zmx", "-g", "sc", "list")
	out, err := cmd.Output()
	if err != nil {
		return make(map[string]bool)
	}
	return parseSessions(string(out))
}
```

**Step 4: Run test to verify it passes**

Run: `nix develop --command go test -run TestParseSessions ./packages/spinclass/internal/executor/...`
Expected: PASS

**Step 5: Commit**

```
feat(spinclass): add zmx session listing and parsing
```

---

### Task 2: Add `RepoStatus` type and `Session` field

**Files:**
- Modify: `packages/spinclass/internal/status/status.go`

**Step 1: Add new types and update `BranchStatus`**

At the top of `status.go`, add `Session` field to `BranchStatus` and add `RepoStatus`:

```go
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
```

This is a data-only change. No tests yet — the new struct is exercised by the
next tasks.

**Step 2: Commit**

```
refactor(spinclass): add RepoStatus type and Session field to BranchStatus
```

---

### Task 3: Update `CollectRepoStatus` and `CollectStatus` to return `[]RepoStatus`

**Files:**
- Modify: `packages/spinclass/internal/status/status.go`

**Step 1: Update `CollectRepoStatus` signature and body**

Change `CollectRepoStatus` to accept a session set and return `RepoStatus`:

```go
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
```

**Step 2: Update `CollectStatus` to use `ListSessions` and return `[]RepoStatus`**

```go
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
```

Add import for `"github.com/amarbel-llc/spinclass/internal/executor"`.

**Step 3: Commit**

```
refactor(spinclass): return grouped RepoStatus from CollectStatus
```

---

### Task 4: Rewrite `Render` for tree-style output

**Files:**
- Modify: `packages/spinclass/internal/status/status.go`
- Test: `packages/spinclass/internal/status/status_test.go`

**Step 1: Write failing tests for tree render**

Replace the existing render tests in `status_test.go` with tests for the new
output format:

```go
func TestRenderTreeStructure(t *testing.T) {
	repos := []RepoStatus{
		{
			Main: BranchStatus{
				Repo: "myrepo", Branch: "main", Dirty: "clean",
				Remote: "≡ origin/main", LastCommit: "2025-01-01",
				LastModified: "2025-01-01",
			},
			Worktrees: []BranchStatus{
				{
					Repo: "myrepo", Branch: "feature-x", Dirty: "2M 1?",
					Remote: "↑3 origin/feature-x", LastCommit: "2025-01-02",
					LastModified: "2025-01-02", IsWorktree: true, Session: true,
				},
			},
		},
	}

	output := Render(repos)
	if !strings.Contains(output, "myrepo") {
		t.Error("expected repo name in output")
	}
	if !strings.Contains(output, "feature-x") {
		t.Error("expected worktree branch in output")
	}
	if !strings.Contains(output, "└") {
		t.Error("expected tree connector └ for last worktree")
	}
	if !strings.Contains(output, "● zmx") {
		t.Error("expected zmx indicator for active session")
	}
}

func TestRenderMultipleWorktrees(t *testing.T) {
	repos := []RepoStatus{
		{
			Main: BranchStatus{
				Repo: "myrepo", Branch: "main", Dirty: "clean",
				Remote: "≡ origin/main",
			},
			Worktrees: []BranchStatus{
				{
					Repo: "myrepo", Branch: "wt-a", Dirty: "1M",
					IsWorktree: true,
				},
				{
					Repo: "myrepo", Branch: "wt-b", Dirty: "clean",
					IsWorktree: true, Session: true,
				},
			},
		},
	}

	output := Render(repos)
	if !strings.Contains(output, "├") {
		t.Error("expected tree connector ├ for non-last worktree")
	}
	if !strings.Contains(output, "└") {
		t.Error("expected tree connector └ for last worktree")
	}
}

func TestRenderNoWorktrees(t *testing.T) {
	repos := []RepoStatus{
		{
			Main: BranchStatus{
				Repo: "solo", Branch: "main", Dirty: "clean",
				Remote: "≡ origin/main",
			},
		},
	}

	output := Render(repos)
	if !strings.Contains(output, "solo") {
		t.Error("expected repo name")
	}
	if strings.Contains(output, "├") || strings.Contains(output, "└") {
		t.Error("did not expect tree connectors with no worktrees")
	}
}

func TestRenderNoSession(t *testing.T) {
	repos := []RepoStatus{
		{
			Main: BranchStatus{
				Repo: "myrepo", Branch: "main", Dirty: "clean",
			},
			Worktrees: []BranchStatus{
				{
					Repo: "myrepo", Branch: "wt", Dirty: "clean",
					IsWorktree: true, Session: false,
				},
			},
		},
	}

	output := Render(repos)
	if strings.Contains(output, "● zmx") {
		t.Error("did not expect zmx indicator when session is false")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `nix develop --command go test -run TestRenderTree ./packages/spinclass/internal/status/...`
Expected: FAIL — `Render` signature mismatch (expects `[]BranchStatus`)

**Step 3: Rewrite `Render` and remove old `renderTable`**

Replace `Render`, `renderTable`, and the `isClean` method. Remove the
`lipgloss/table` import. Keep the `lipgloss` import for styles.

```go
var (
	styleRepo    = lipgloss.NewStyle().Bold(true)
	styleDirty   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleClean   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleRemoteSync   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleRemoteDrift  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleRemoteNone   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleSession = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

type renderRow struct {
	prefix   string // "  ├ " or "  └ " or ""
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
		// Main branch row — prefix is the repo name (handled in rendering)
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

func Render(repos []RepoStatus) string {
	rows := collectRenderRows(repos)
	if len(rows) == 0 {
		return ""
	}

	// Calculate column widths
	widths := [7]int{} // prefix, branch, dirty, remote, commit, modified, session
	for _, r := range rows {
		cols := [7]string{r.prefix, r.branch, r.dirty, r.remote, r.commit, r.modified, r.session}
		for i, c := range cols {
			if len(c) > widths[i] {
				widths[i] = len(c)
			}
		}
	}

	var lines []string
	for _, r := range rows {
		prefix := fmt.Sprintf("%-*s", widths[0], r.prefix)
		branch := fmt.Sprintf("%-*s", widths[1], r.branch)
		commit := fmt.Sprintf("%-*s", widths[4], r.commit)
		modified := fmt.Sprintf("%-*s", widths[5], r.modified)

		// Style the dirty column
		dirtyPad := fmt.Sprintf("%-*s", widths[2], r.dirty)
		var styledDirty string
		if r.dirty == "clean" {
			styledDirty = styleClean.Render(dirtyPad)
		} else {
			styledDirty = styleDirty.Render(dirtyPad)
		}

		// Style the remote column
		remotePad := fmt.Sprintf("%-*s", widths[3], r.remote)
		var styledRemote string
		if strings.HasPrefix(r.remote, "≡") {
			styledRemote = styleRemoteSync.Render(remotePad)
		} else if strings.Contains(r.remote, "↑") || strings.Contains(r.remote, "↓") {
			styledRemote = styleRemoteDrift.Render(remotePad)
		} else {
			styledRemote = styleRemoteNone.Render(remotePad)
		}

		// Style the session column
		sessionPad := fmt.Sprintf("%-*s", widths[6], r.session)
		var styledSession string
		if r.session != "" {
			styledSession = styleSession.Render(sessionPad)
		} else {
			styledSession = sessionPad
		}

		// Style the prefix — bold for repo name (no connector), dim for connectors
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
```

Remove `renderTable`, `isClean`, `styleHeader`, and `styleCode` — they are
unused now. Remove the `"github.com/charmbracelet/lipgloss/table"` import.

**Step 4: Run tests to verify they pass**

Run: `nix develop --command go test ./packages/spinclass/internal/status/...`
Expected: PASS (the old tests have been replaced)

**Step 5: Commit**

```
feat(spinclass): tree-style status rendering with zmx session column
```

---

### Task 5: Update `RenderTap` for `[]RepoStatus`

**Files:**
- Modify: `packages/spinclass/internal/status/status.go`

**Step 1: Update `RenderTap` signature**

```go
func RenderTap(repos []RepoStatus, w io.Writer) {
	tw := tap.NewWriter(w)
	for _, rs := range repos {
		tw.Ok(rs.Main.Repo + " " + styleCode.Render(rs.Main.Branch))
		for _, wt := range rs.Worktrees {
			tw.Ok(wt.Repo + " " + styleCode.Render(wt.Branch))
		}
	}
	tw.Plan()
}
```

Note: `styleCode` was removed in Task 4. Replace with a local inline style or
just use the branch name directly without styling (TAP is typically
machine-consumed):

```go
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
```

**Step 2: Commit**

```
refactor(spinclass): update RenderTap for RepoStatus
```

---

### Task 6: Update CLI `statusCmd` for new return type

**Files:**
- Modify: `packages/spinclass/cmd/spinclass/main.go`

**Step 1: Update `statusCmd.RunE`**

The call to `status.CollectStatus(cwd)` now returns `[]status.RepoStatus`
instead of `[]status.BranchStatus`. The `Render` and `RenderTap` functions
already accept the new type from Tasks 4–5. The only change needed is the
length check:

```go
RunE: func(cmd *cobra.Command, args []string) error {
    cwd, err := os.Getwd()
    if err != nil {
        return err
    }

    format := outputFormat
    if format == "" {
        format = "table"
    }

    repos := status.CollectStatus(cwd)
    if len(repos) == 0 {
        log.Info("no repos found")
        return nil
    }

    if format == "tap" {
        status.RenderTap(repos, os.Stdout)
    } else {
        fmt.Println(status.Render(repos))
    }
    return nil
},
```

Variable name changes from `rows` to `repos` for clarity, but otherwise the
structure is the same.

**Step 2: Build to verify compilation**

Run: `nix develop --command go build ./packages/spinclass/...`
Expected: successful build

**Step 3: Commit**

```
refactor(spinclass): update status command for RepoStatus type
```

---

### Task 7: Manual verification

**Step 1: Build and run**

Run: `nix develop --command go run ./packages/spinclass/cmd/spinclass status`
Expected: tree-style output with repos, nested worktrees, zmx session indicators

**Step 2: Verify TAP output**

Run: `nix develop --command go run ./packages/spinclass/cmd/spinclass status --format tap`
Expected: TAP-14 output with one `ok` per branch

**Step 3: Run full test suite**

Run: `nix develop --command go test ./packages/spinclass/...`
Expected: all tests pass

**Step 4: Commit (if any fixups needed)**

```
fix(spinclass): <description of fixup>
```
