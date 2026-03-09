package status

import (
	"strings"
	"testing"
)

func TestParseDirtyStatusClean(t *testing.T) {
	result := parseDirtyStatus("")
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestParseDirtyStatusModified(t *testing.T) {
	result := parseDirtyStatus(" M file.txt")
	if result != "1M" {
		t.Errorf("expected 1M, got %q", result)
	}
}

func TestParseDirtyStatusUntracked(t *testing.T) {
	result := parseDirtyStatus("?? newfile.txt")
	if result != "1?" {
		t.Errorf("expected 1?, got %q", result)
	}
}

func TestParseDirtyStatusMixed(t *testing.T) {
	input := " M file1.txt\n?? file2.txt\nA  file3.txt"
	result := parseDirtyStatus(input)
	if !strings.Contains(result, "1M") {
		t.Errorf("expected 1M in %q", result)
	}
	if !strings.Contains(result, "1?") {
		t.Errorf("expected 1? in %q", result)
	}
	if !strings.Contains(result, "1A") {
		t.Errorf("expected 1A in %q", result)
	}
}

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
