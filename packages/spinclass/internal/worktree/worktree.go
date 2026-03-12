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
	AbsPath        string // absolute filesystem path to the worktree
	RepoPath       string // absolute path to the parent git repo
	SessionKey     string // key for zmx/executor sessions (<repo-dirname>/<branch>)
	Branch         string // branch name
	ExistingBranch string // non-empty when an existing branch was detected
}

// ResolvePath resolves a worktree target relative to a git repo.
//
// When args is empty a random name is generated. Otherwise the args are
// sanitised into a branch name and checked against local and remote refs
// to detect existing branches.
//
// SessionKey is always <repo-dirname>/<branch>.
func ResolvePath(
	sf sweatfile.Sweatfile,
	repoPath string,
	args []string,
) (ResolvedPath, error) {
	if len(args) == 0 {
		branch := RandomName(repoPath)
		absPath := filepath.Join(repoPath, WorktreesDir, branch)
		repoDirname := filepath.Base(repoPath)
		return ResolvedPath{
			AbsPath:    absPath,
			RepoPath:   repoPath,
			SessionKey: repoDirname + "/" + branch,
			Branch:     branch,
		}, nil
	}

	unsanitizedName := strings.Join(args, "-")
	sanitizedName := SanitizeBranchName(args)
	if sanitizedName == "" {
		return ResolvedPath{}, fmt.Errorf("branch name is empty after sanitization of %q", args)
	}

	transformedName, err := sf.CreateBranchName(sanitizedName)
	if err != nil {
		return ResolvedPath{}, err
	}

	branch, existingBranch := detectBranch(repoPath, unsanitizedName, sanitizedName, transformedName)

	absPath := filepath.Join(repoPath, WorktreesDir, branch)
	repoDirname := filepath.Base(repoPath)
	sessionKey := repoDirname + "/" + branch

	return ResolvedPath{
		AbsPath:        absPath,
		RepoPath:       repoPath,
		SessionKey:     sessionKey,
		Branch:         branch,
		ExistingBranch: existingBranch,
	}, nil
}

func detectBranch(repoPath string, candidates ...string) (string, string) {
	seen := make(map[string]bool)
	var unique []string
	for _, c := range candidates {
		if c != "" && !seen[c] {
			seen[c] = true
			unique = append(unique, c)
		}
	}

	for _, name := range unique {
		if git.BranchExists(repoPath, name) {
			return name, name
		}
	}
	for _, name := range unique {
		if git.RemoteBranchExists(repoPath, name) {
			return name, name
		}
	}

	// No existing branch found — use the last candidate (most transformed).
	return unique[len(unique)-1], ""
}

// DetectRepo walks up from dir looking for a .git directory (must be a
// directory, not a file — files indicate worktrees). Respects
// GIT_CEILING_DIRECTORIES to prevent discovery above certain paths.
// Returns the repo root.
func DetectRepo(dir string) (string, error) {
	dir = filepath.Clean(dir)
	ceilings := parseCeilingDirs()

	for {
		gitPath := filepath.Join(dir, ".git")
		info, err := os.Lstat(gitPath)
		if err == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir || isCeiling(dir, ceilings) {
			return "", fmt.Errorf("no git repository found from %s", dir)
		}
		dir = parent
	}
}

func parseCeilingDirs() []string {
	env := os.Getenv("GIT_CEILING_DIRECTORIES")
	if env == "" {
		return nil
	}

	var dirs []string
	for _, d := range filepath.SplitList(env) {
		if clean := filepath.Clean(d); filepath.IsAbs(clean) {
			dirs = append(dirs, clean)
		}
	}
	return dirs
}

func isCeiling(dir string, ceilings []string) bool {
	for _, c := range ceilings {
		if dir == c {
			return true
		}
	}
	return false
}

// Create creates a new git worktree and applies sweatfile configuration.
// If existingBranch is non-empty, the worktree checks out that branch
// instead of creating a new one from the directory name.
func Create(repoPath, worktreePath, existingBranch string) (sweatfile.Hierarchy, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return sweatfile.Hierarchy{}, fmt.Errorf("getting home directory: %w", err)
	}

	sweetfile, err := sweatfile.LoadHierarchy(home, repoPath)
	if err != nil {
		return sweetfile, fmt.Errorf("loading sweatfile: %w", err)
	}

	if existingBranch != "" {
		if err := git.RunPassthrough(repoPath, "worktree", "add", worktreePath, existingBranch); err != nil {
			return sweatfile.Hierarchy{}, fmt.Errorf("git worktree add: %w", err)
		}
	} else {
		if err := git.RunPassthrough(repoPath, "worktree", "add", worktreePath); err != nil {
			return sweatfile.Hierarchy{}, fmt.Errorf("git worktree add: %w", err)
		}
	}

	return sweetfile, applyWorktreeConfig(home, sweetfile, repoPath, worktreePath)
}

// CreateFrom creates a new worktree branched from fromPath's current HEAD.
// It runs git worktree add -b from fromPath, then applies sweatfile and
// trusts the workspace, same as Create.
func CreateFrom(repoPath, fromPath, newPath, newBranch string) (sweatfile.Hierarchy, error) {
	if err := git.WorktreeAddFrom(fromPath, newBranch, newPath); err != nil {
		return sweatfile.Hierarchy{}, fmt.Errorf("git worktree add: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return sweatfile.Hierarchy{}, fmt.Errorf("getting home directory: %w", err)
	}

	sweetfile, err := sweatfile.LoadHierarchy(home, repoPath)
	if err != nil {
		return sweetfile, fmt.Errorf("loading sweatfile: %w", err)
	}

	return sweetfile, applyWorktreeConfig(home, sweetfile, repoPath, newPath)
}

// applyWorktreeConfig excludes .worktrees from git, loads and applies sweatfile,
// and trusts worktreePath in Claude.
func applyWorktreeConfig(
	home string,
	sweetfile sweatfile.Hierarchy,
	repoPath string,
	worktreePath string,
) error {
	if err := excludeWorktreesDir(repoPath); err != nil {
		return fmt.Errorf("excluding %s from git: %w", WorktreesDir, err)
	}

	tmpDir := filepath.Join(worktreePath, ".tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return fmt.Errorf("creating .tmp directory: %w", err)
	}

	if err := sweetfile.Merged.Apply(worktreePath); err != nil {
		return fmt.Errorf("applying sweatfile: %w", err)
	}

	claudeJSONPath := filepath.Join(home, ".claude.json")
	if err := claude.TrustWorkspace(claudeJSONPath, worktreePath); err != nil {
		return fmt.Errorf("trusting workspace in claude: %w", err)
	}

	if err := sweetfile.Merged.RunCreateHook(worktreePath); err != nil {
		git.RunPassthrough(repoPath, "worktree", "remove", "--force", worktreePath)
		return fmt.Errorf("create hook failed: %w", err)
	}

	return nil
}

// excludeWorktreesDir appends WorktreesDir to .git/info/exclude if not already present.
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
	return git.IsWorktree(path)
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

// ScanRepos scans for repositories that have a WorktreesDir directory.
// If startDir itself is a repo with WorktreesDir, returns just that path.
// Otherwise scans immediate children for repos with WorktreesDir.
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

// ListWorktrees returns absolute paths of all worktree directories in <repoPath>/<WorktreesDir>/.
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
