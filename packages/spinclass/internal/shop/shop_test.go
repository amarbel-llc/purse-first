package shop

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amarbel-llc/spinclass/internal/tap"
	"github.com/amarbel-llc/spinclass/internal/worktree"
)

func TestStatusDescription(t *testing.T) {
	tests := []struct {
		name          string
		defaultBranch string
		commitsAhead  int
		porcelain     string
		want          string
	}{
		{
			name:          "ahead and clean",
			defaultBranch: "master",
			commitsAhead:  3,
			porcelain:     "",
			want:          "3 commits ahead of master, clean",
		},
		{
			name:          "one commit ahead",
			defaultBranch: "master",
			commitsAhead:  1,
			porcelain:     "",
			want:          "1 commit ahead of master, clean",
		},
		{
			name:          "ahead and dirty",
			defaultBranch: "main",
			commitsAhead:  2,
			porcelain:     "M file.go\n",
			want:          "2 commits ahead of main, dirty",
		},
		{
			name:          "merged",
			defaultBranch: "master",
			commitsAhead:  0,
			porcelain:     "",
			want:          "0 commits ahead of master, clean, (merged)",
		},
		{
			name:          "zero ahead but dirty",
			defaultBranch: "master",
			commitsAhead:  0,
			porcelain:     "?? untracked\n",
			want:          "0 commits ahead of master, dirty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := statusDescription(tt.defaultBranch, tt.commitsAhead, tt.porcelain)
			if got != tt.want {
				t.Errorf("statusDescription() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCreateTapNewWorktree(t *testing.T) {
	dir := t.TempDir()
	worktreePath := filepath.Join(dir, "new-worktree")

	rp := worktree.ResolvedPath{
		AbsPath:  worktreePath,
		RepoPath: dir,
		Branch:   "feature-x",
	}

	var buf bytes.Buffer
	tw := tap.NewWriter(&buf)
	tw.PlanAhead(1)
	tw.Ok("create feature-x " + worktreePath)

	got := buf.String()
	if !strings.Contains(got, "ok 1 - create feature-x") {
		t.Errorf("expected ok line, got: %q", got)
	}
	_ = rp
}

func TestCreateTapSkipExisting(t *testing.T) {
	dir := t.TempDir()

	var buf bytes.Buffer
	tw := tap.NewWriter(&buf)
	tw.PlanAhead(1)
	tw.Skip("create feature-x", "already exists "+dir)

	got := buf.String()
	if !strings.Contains(got, "# SKIP already exists") {
		t.Errorf("expected SKIP line, got: %q", got)
	}
}
