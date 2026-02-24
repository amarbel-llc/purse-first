package worktree

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/amarbel-llc/spinclass/internal/claude"
	"github.com/amarbel-llc/spinclass/internal/git"
	"github.com/amarbel-llc/spinclass/internal/sweatfile"
)

const WorktreesDir = ".worktrees"

type ResolvedPath struct {
	AbsPath    string // absolute filesystem path to the worktree
	RepoPath   string // absolute path to the parent git repo
	SessionKey string // key for zmx/executor sessions (<repo-dirname>/<branch>)
	Branch     string // branch name
}

// ResolvePath resolves a worktree target relative to a git repo.
//
// target interpretation:
//   - bare branch name (no "/" or ".") -> <repoPath>/.worktrees/<branch>
//   - relative path (contains "/" or ".") -> resolved relative to repoPath
//   - absolute path -> used directly
//
// SessionKey is always <repo-dirname>/<branch>.
func ResolvePath(repoPath, target string) (ResolvedPath, error) {
	var absPath string
	var branch string

	if filepath.IsAbs(target) {
		absPath = filepath.Clean(target)
		branch = filepath.Base(absPath)
	} else if strings.ContainsAny(target, "/.") {
		absPath = filepath.Clean(filepath.Join(repoPath, target))
		branch = filepath.Base(absPath)
	} else {
		// Bare branch name
		branch = target
		absPath = filepath.Join(repoPath, WorktreesDir, branch)
	}

	repoDirname := filepath.Base(repoPath)
	sessionKey := repoDirname + "/" + branch

	return ResolvedPath{
		AbsPath:    absPath,
		RepoPath:   repoPath,
		SessionKey: sessionKey,
		Branch:     branch,
	}, nil
}

// DetectRepo walks up from dir looking for a .git directory (must be a
// directory, not a file — files indicate worktrees). Returns the repo root.
func DetectRepo(dir string) (string, error) {
	dir = filepath.Clean(dir)
	for {
		gitPath := filepath.Join(dir, ".git")
		info, err := os.Lstat(gitPath)
		if err == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no git repository found from %s", dir)
		}
		dir = parent
	}
}

// Create creates a new git worktree and applies sweatfile configuration.
func Create(repoPath, worktreePath string) (sweatfile.LoadResult, error) {
	if err := git.RunPassthrough(repoPath, "worktree", "add", worktreePath); err != nil {
		return sweatfile.LoadResult{}, fmt.Errorf("git worktree add: %w", err)
	}
	return applyWorktreeConfig(repoPath, worktreePath)
}

// CreateFrom creates a new worktree branched from fromPath's current HEAD.
// It runs git worktree add -b from fromPath, then applies sweatfile and
// trusts the workspace, same as Create.
func CreateFrom(repoPath, fromPath, newPath, newBranch string) (sweatfile.LoadResult, error) {
	if err := git.WorktreeAddFrom(fromPath, newBranch, newPath); err != nil {
		return sweatfile.LoadResult{}, fmt.Errorf("git worktree add: %w", err)
	}
	return applyWorktreeConfig(repoPath, newPath)
}

// applyWorktreeConfig excludes .worktrees from git, loads and applies sweatfile,
// and trusts worktreePath in Claude.
func applyWorktreeConfig(repoPath, worktreePath string) (sweatfile.LoadResult, error) {
	if err := excludeWorktreesDir(repoPath); err != nil {
		return sweatfile.LoadResult{}, fmt.Errorf("excluding .worktrees from git: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return sweatfile.LoadResult{}, fmt.Errorf("getting home directory: %w", err)
	}

	result, err := sweatfile.LoadHierarchy(home, repoPath)
	if err != nil {
		return sweatfile.LoadResult{}, fmt.Errorf("loading sweatfile: %w", err)
	}
	if err := sweatfile.Apply(worktreePath, result.Merged); err != nil {
		return sweatfile.LoadResult{}, fmt.Errorf("applying sweatfile: %w", err)
	}

	claudeJSONPath := filepath.Join(home, ".claude.json")
	if err := claude.TrustWorkspace(claudeJSONPath, worktreePath); err != nil {
		return sweatfile.LoadResult{}, fmt.Errorf("trusting workspace in claude: %w", err)
	}

	return result, nil
}

// excludeWorktreesDir appends .worktrees to .git/info/exclude if not already present.
func excludeWorktreesDir(repoPath string) error {
	excludePath := filepath.Join(repoPath, ".git", "info", "exclude")

	if data, err := os.ReadFile(excludePath); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			if strings.TrimSpace(scanner.Text()) == WorktreesDir {
				return nil
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return err
	}

	f, err := os.OpenFile(excludePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = fmt.Fprintln(f, WorktreesDir)
	return err
}

// IsWorktree returns true if path contains a .git file (not directory),
// indicating it is a git worktree rather than the main repository.
func IsWorktree(path string) bool {
	info, err := os.Lstat(filepath.Join(path, ".git"))
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// FillBranchFromGit populates the Branch field from git.
func (rp *ResolvedPath) FillBranchFromGit() error {
	branch, err := git.BranchCurrent(rp.AbsPath)
	if err != nil {
		return err
	}
	rp.Branch = branch
	return nil
}

// ScanRepos scans for repositories that have a .worktrees/ directory.
// If startDir itself is a repo with .worktrees/, returns just that path.
// Otherwise scans immediate children for repos with .worktrees/.
func ScanRepos(startDir string) []string {
	if isRepoWithWorktrees(startDir) {
		return []string{startDir}
	}

	entries, err := os.ReadDir(startDir)
	if err != nil {
		return nil
	}

	var repos []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		child := filepath.Join(startDir, entry.Name())
		if isRepoWithWorktrees(child) {
			repos = append(repos, child)
		}
	}
	return repos
}

func isRepoWithWorktrees(dir string) bool {
	gitInfo, err := os.Stat(filepath.Join(dir, ".git"))
	if err != nil || !gitInfo.IsDir() {
		return false
	}
	wtInfo, err := os.Stat(filepath.Join(dir, WorktreesDir))
	if err != nil || !wtInfo.IsDir() {
		return false
	}
	return true
}

// ListWorktrees returns absolute paths of all worktree directories in <repoPath>/.worktrees/.
func ListWorktrees(repoPath string) []string {
	wtDir := filepath.Join(repoPath, WorktreesDir)
	entries, err := os.ReadDir(wtDir)
	if err != nil {
		return nil
	}

	var worktrees []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		wtPath := filepath.Join(wtDir, entry.Name())
		if IsWorktree(wtPath) {
			worktrees = append(worktrees, wtPath)
		}
	}
	return worktrees
}

// ForkName returns a collision-free branch name for forking sourceBranch.
// It tries <sourceBranch>-1, <sourceBranch>-2, etc., checking for existing
// directories in <repoPath>/.worktrees/.
func ForkName(repoPath, sourceBranch string) string {
	wtDir := filepath.Join(repoPath, WorktreesDir)
	for n := 1; ; n++ {
		candidate := fmt.Sprintf("%s-%d", sourceBranch, n)
		_, err := os.Stat(filepath.Join(wtDir, candidate))
		if os.IsNotExist(err) {
			return candidate
		}
	}
}
