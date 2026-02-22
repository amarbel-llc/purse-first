package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func Run(repoPath string, args ...string) (string, error) {
	cmdArgs := append([]string{"-C", repoPath}, args...)
	cmd := exec.Command("git", cmdArgs...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimRight(string(exitErr.Stderr), "\n"))
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func RunPassthrough(repoPath string, args ...string) error {
	cmdArgs := append([]string{"-C", repoPath}, args...)
	cmd := exec.Command("git", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func BranchCurrent(repoPath string) (string, error) {
	return Run(repoPath, "branch", "--show-current")
}

func CommitsAhead(worktreePath, base, branch string) int {
	out, err := Run(worktreePath, "rev-list", base+".."+branch, "--count")
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(out)
	return n
}

func StatusPorcelain(path string) string {
	out, err := Run(path, "status", "--porcelain")
	if err != nil {
		return ""
	}
	return out
}

func RevListLeftRight(path string) (ahead, behind int) {
	out, err := Run(path, "rev-list", "--left-right", "--count", "@{upstream}...HEAD")
	if err != nil {
		return 0, 0
	}
	parts := strings.Fields(out)
	if len(parts) != 2 {
		return 0, 0
	}
	behind, _ = strconv.Atoi(parts[0])
	ahead, _ = strconv.Atoi(parts[1])
	return ahead, behind
}

func Upstream(path string) string {
	out, err := Run(path, "rev-parse", "--abbrev-ref", "@{upstream}")
	if err != nil {
		return ""
	}
	return out
}

func LastCommitDate(path string) string {
	out, err := Run(path, "log", "-1", "--format=%cs")
	if err != nil {
		return "n/a"
	}
	return out
}

func HasDirtyTracked(repoPath string) bool {
	cmd := exec.Command("git", "-C", repoPath, "diff", "--quiet")
	if err := cmd.Run(); err != nil {
		return true
	}
	cmd = exec.Command("git", "-C", repoPath, "diff", "--cached", "--quiet")
	if err := cmd.Run(); err != nil {
		return true
	}
	return false
}

func CheckoutFile(repoPath, file string) error {
	_, err := Run(repoPath, "checkout", "--", file)
	return err
}

func ResetFile(repoPath, file string) error {
	_, err := Run(repoPath, "reset", "HEAD", "--", file)
	return err
}

func WorktreeRemove(repoPath, worktreePath string) error {
	_, err := Run(repoPath, "worktree", "remove", worktreePath)
	return err
}

func BranchDelete(repoPath, branch string) error {
	_, err := Run(repoPath, "branch", "-d", branch)
	return err
}

func DefaultBranch(repoPath string) (string, error) {
	return BranchCurrent(repoPath)
}

func NewestFileTime(path string) time.Time {
	var newest time.Time
	filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Skip .git directories
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		if !info.IsDir() && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
		return nil
	})
	return newest
}

func CommonDir(worktreePath string) (string, error) {
	out, err := Run(worktreePath, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(out) {
		out = filepath.Join(worktreePath, out)
	}
	out = filepath.Clean(out)
	// Strip trailing .git to get the repo root
	if filepath.Base(out) == ".git" {
		out = filepath.Dir(out)
	}
	return out, nil
}

func Pull(repoPath string) (string, error) {
	return Run(repoPath, "pull")
}

func Rebase(repoPath, onto string) (string, error) {
	return Run(repoPath, "rebase", onto)
}
