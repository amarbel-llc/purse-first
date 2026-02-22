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

func TestRenderProducesOutput(t *testing.T) {
	rows := []BranchStatus{
		{
			Repo:         "eng/repos/myrepo",
			Branch:       "main",
			Dirty:        "clean",
			Remote:       "≡ origin/main",
			LastCommit:   "2025-01-01",
			LastModified: "2025-01-01",
		},
		{
			Repo:         "eng/repos/myrepo",
			Branch:       "feature-x",
			Dirty:        "2M 1?",
			Remote:       "↑3 origin/feature-x",
			LastCommit:   "2025-01-02",
			LastModified: "2025-01-02",
			IsWorktree:   true,
		},
	}

	output := Render(rows)
	if output == "" {
		t.Error("expected non-empty render output")
	}
	if !strings.Contains(output, "Repo") {
		t.Error("expected header 'Repo' in output")
	}
	if !strings.Contains(output, "myrepo") {
		t.Error("expected 'myrepo' in output")
	}
}

func TestRenderSectionHeaders(t *testing.T) {
	rows := []BranchStatus{
		{
			Repo:       "eng/repos/dirty-repo",
			Branch:     "main",
			Dirty:      "1M",
			Remote:     "↑1 origin/main",
			IsWorktree: false,
		},
		{
			Repo:       "eng/repos/dirty-repo",
			Branch:     "feature",
			Dirty:      "2M",
			Remote:     "↑2 origin/feature",
			IsWorktree: true,
		},
		{
			Repo:       "eng/repos/clean-repo",
			Branch:     "main",
			Dirty:      "clean",
			Remote:     "≡ origin/main",
			IsWorktree: false,
		},
	}

	output := Render(rows)
	if !strings.Contains(output, "Repos") {
		t.Error("expected 'Repos' section header")
	}
	if !strings.Contains(output, "Worktrees") {
		t.Error("expected 'Worktrees' section header")
	}
	if !strings.Contains(output, "Clean") {
		t.Error("expected 'Clean' section header")
	}
}

func TestRenderCleanGrouping(t *testing.T) {
	rows := []BranchStatus{
		{
			Repo:       "eng/repos/repo-a",
			Branch:     "main",
			Dirty:      "clean",
			Remote:     "≡ origin/main",
			IsWorktree: false,
		},
		{
			Repo:       "eng/repos/repo-a",
			Branch:     "feature",
			Dirty:      "clean",
			Remote:     "",
			IsWorktree: true,
		},
	}

	output := Render(rows)
	if !strings.Contains(output, "Clean") {
		t.Error("expected 'Clean' section header")
	}
	if strings.Contains(output, "Repos") {
		t.Error("did not expect 'Repos' section when all rows are clean")
	}
	if strings.Contains(output, "Worktrees") {
		t.Error("did not expect 'Worktrees' section when all rows are clean")
	}
	if !strings.Contains(output, "repo-a") {
		t.Error("expected 'repo-a' in clean section")
	}
}
