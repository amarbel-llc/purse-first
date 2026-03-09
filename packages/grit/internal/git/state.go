package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

type InProgressState struct {
	Operation string `json:"operation"`
	Branch    string `json:"branch,omitempty"`
	Step      string `json:"step,omitempty"`
}

func DetectInProgressState(ctx context.Context, repoPath string) (*InProgressState, error) {
	gitDir, err := resolveGitDir(ctx, repoPath)
	if err != nil {
		return nil, err
	}

	if state := detectRebase(gitDir); state != nil {
		return state, nil
	}

	if fileExists(filepath.Join(gitDir, "MERGE_HEAD")) {
		return &InProgressState{Operation: "merge"}, nil
	}

	if fileExists(filepath.Join(gitDir, "CHERRY_PICK_HEAD")) {
		return &InProgressState{Operation: "cherry-pick"}, nil
	}

	if fileExists(filepath.Join(gitDir, "REVERT_HEAD")) {
		return &InProgressState{Operation: "revert"}, nil
	}

	return nil, nil
}

func resolveGitDir(ctx context.Context, repoPath string) (string, error) {
	out, err := Run(ctx, repoPath, "rev-parse", "--git-dir")
	if err != nil {
		return "", err
	}

	gitDir := strings.TrimSpace(out)

	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(repoPath, gitDir)
	}

	return gitDir, nil
}

func detectRebase(gitDir string) *InProgressState {
	rebaseMergeDir := filepath.Join(gitDir, "rebase-merge")
	if dirExists(rebaseMergeDir) {
		state := &InProgressState{Operation: "rebase"}
		state.Branch = readRebaseBranch(rebaseMergeDir)
		state.Step = readRebaseStep(rebaseMergeDir)
		return state
	}

	rebaseApplyDir := filepath.Join(gitDir, "rebase-apply")
	if dirExists(rebaseApplyDir) {
		state := &InProgressState{Operation: "rebase"}
		state.Step = readRebaseStep(rebaseApplyDir)
		return state
	}

	return nil
}

func readRebaseBranch(rebaseDir string) string {
	data, err := os.ReadFile(filepath.Join(rebaseDir, "head-name"))
	if err != nil {
		return ""
	}

	name := strings.TrimSpace(string(data))

	return strings.TrimPrefix(name, "refs/heads/")
}

func readRebaseStep(rebaseDir string) string {
	msgnum, err := os.ReadFile(filepath.Join(rebaseDir, "msgnum"))
	if err != nil {
		return ""
	}

	end, err := os.ReadFile(filepath.Join(rebaseDir, "end"))
	if err != nil {
		return ""
	}

	current := strings.TrimSpace(string(msgnum))
	total := strings.TrimSpace(string(end))

	if current == "" || total == "" {
		return ""
	}

	return current + "/" + total
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
