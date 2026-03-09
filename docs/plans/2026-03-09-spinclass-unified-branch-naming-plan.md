# Unified Branch Naming Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Unify spinclass's branch naming: all positional args become a snob-cased branch name with automatic existing-branch detection, removing `-b` and claude args.

**Architecture:** `SanitizeBranchName` in `worktree/namegen.go` performs snob-case + git sanitization. `ResolvePath` gains branch existence checks (local + remote) against both raw and sweatfile-transformed names. `CreateBranchName` passes the base name as a CLI argument. The `-b` flag and claude args are removed from the CLI surface.

**Tech Stack:** Go, git CLI, cobra

**Rollback:** `git revert` — this is a breaking CLI change but spinclass is pre-1.0.

---

### Task 1: Add SanitizeBranchName

**Files:**
- Create: `packages/spinclass/internal/worktree/sanitize.go`
- Test: `packages/spinclass/internal/worktree/sanitize_test.go`

**Step 1: Write the failing tests**

Create `packages/spinclass/internal/worktree/sanitize_test.go`:

```go
package worktree

import "testing"

func TestSanitizeBranchName(t *testing.T) {
	tests := []struct {
		name  string
		parts []string
		want  string
	}{
		{
			name:  "multiple words joined with hyphens",
			parts: []string{"this", "is", "a", "branch"},
			want:  "this-is-a-branch",
		},
		{
			name:  "existing hyphens become underscores",
			parts: []string{"branch-name"},
			want:  "branch_name",
		},
		{
			name:  "multi-word with hyphens in parts",
			parts: []string{"this", "is", "the", "branch-name"},
			want:  "this-is-the-branch_name",
		},
		{
			name:  "uppercase lowercased",
			parts: []string{"Fix", "AUTH", "Bug"},
			want:  "fix-auth-bug",
		},
		{
			name:  "single word passthrough",
			parts: []string{"feature"},
			want:  "feature",
		},
		{
			name:  "strips tilde",
			parts: []string{"branch~1"},
			want:  "branch1",
		},
		{
			name:  "strips caret",
			parts: []string{"branch^2"},
			want:  "branch2",
		},
		{
			name:  "strips colon",
			parts: []string{"branch:name"},
			want:  "branchname",
		},
		{
			name:  "strips backslash",
			parts: []string{"branch\\name"},
			want:  "branchname",
		},
		{
			name:  "strips question mark",
			parts: []string{"branch?name"},
			want:  "branchname",
		},
		{
			name:  "strips asterisk",
			parts: []string{"branch*name"},
			want:  "branchname",
		},
		{
			name:  "strips open bracket",
			parts: []string{"branch[name"},
			want:  "branchname",
		},
		{
			name:  "strips space within parts",
			parts: []string{"branch name"},
			want:  "branch-name",
		},
		{
			name:  "collapses consecutive hyphens",
			parts: []string{"a--b"},
			want:  "a_b",
		},
		{
			name:  "strips leading dot",
			parts: []string{".branch"},
			want:  "branch",
		},
		{
			name:  "strips trailing dot",
			parts: []string{"branch."},
			want:  "branch",
		},
		{
			name:  "strips trailing .lock",
			parts: []string{"branch.lock"},
			want:  "branch",
		},
		{
			name:  "collapses double dots",
			parts: []string{"a..b"},
			want:  "a.b",
		},
		{
			name:  "strips at-brace",
			parts: []string{"branch@{name"},
			want:  "branchname",
		},
		{
			name:  "strips control characters",
			parts: []string{"branch\x00name"},
			want:  "branchname",
		},
		{
			name:  "strips leading and trailing hyphens",
			parts: []string{"-branch-"},
			want:  "branch",
		},
		{
			name:  "preserves slashes for branch hierarchy",
			parts: []string{"feature/login"},
			want:  "feature/login",
		},
		{
			name:  "preserves dots mid-word",
			parts: []string{"v1.0"},
			want:  "v1.0",
		},
		{
			name:  "preserves underscores",
			parts: []string{"my_branch"},
			want:  "my_branch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeBranchName(tt.parts)
			if got != tt.want {
				t.Errorf("SanitizeBranchName(%v) = %q, want %q", tt.parts, got, tt.want)
			}
		})
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/light-laurel --command go test -run TestSanitizeBranchName ./packages/spinclass/internal/worktree/...`
Expected: compilation error — `SanitizeBranchName` undefined

**Step 3: Write the implementation**

Create `packages/spinclass/internal/worktree/sanitize.go`:

```go
package worktree

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	gitInvalidChars    = regexp.MustCompile(`[~^:?*\[\]\\` + "\x00-\x1f\x7f" + `]`)
	consecutiveHyphens = regexp.MustCompile(`-{2,}`)
	consecutiveUnders  = regexp.MustCompile(`_{2,}`)
	doubleDots         = regexp.MustCompile(`\.{2,}`)
	atBrace            = regexp.MustCompile(`@\{`)
	trailingLock       = regexp.MustCompile(`\.lock$`)
)

// SanitizeBranchName joins parts into a git-safe branch name using snob-case:
// existing hyphens become underscores, parts are joined with hyphens,
// the result is lowercased, and git-invalid characters are stripped.
func SanitizeBranchName(parts []string) string {
	// Replace hyphens with underscores within each part, and spaces with hyphens
	sanitized := make([]string, len(parts))
	for i, part := range parts {
		part = strings.ReplaceAll(part, "-", "_")
		part = strings.ReplaceAll(part, " ", "-")
		sanitized[i] = part
	}

	result := strings.Join(sanitized, "-")
	result = strings.Map(unicode.ToLower, result)

	// Strip git-invalid characters
	result = atBrace.ReplaceAllString(result, "")
	result = gitInvalidChars.ReplaceAllString(result, "")

	// Collapse consecutive separators
	result = consecutiveHyphens.ReplaceAllString(result, "-")
	result = consecutiveUnders.ReplaceAllString(result, "_")
	result = doubleDots.ReplaceAllString(result, ".")

	// Strip trailing .lock
	result = trailingLock.ReplaceAllString(result, "")

	// Strip leading/trailing dots, hyphens, underscores
	result = strings.TrimLeft(result, ".-_")
	result = strings.TrimRight(result, ".-_")

	return result
}
```

**Step 4: Run tests to verify they pass**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/light-laurel --command go test -run TestSanitizeBranchName ./packages/spinclass/internal/worktree/...`
Expected: PASS

**Step 5: Commit**

```
feat(spinclass): add SanitizeBranchName for snob-case branch naming
```

---

### Task 2: Add RemoteBranchExists to git package

**Files:**
- Modify: `packages/spinclass/internal/git/git.go`

**Step 1: Write the implementation**

Add to `packages/spinclass/internal/git/git.go` after `BranchExists`:

```go
// RemoteBranchExists checks if a branch exists on the origin remote.
func RemoteBranchExists(repoPath, branch string) bool {
	_, err := Run(repoPath, "rev-parse", "--verify", "refs/remotes/origin/"+branch)
	return err == nil
}
```

No separate test needed — this is a one-line wrapper over `Run` + `rev-parse`, same pattern as `BranchExists` which is also untested in isolation (tested via integration in shop tests).

**Step 2: Commit**

```
feat(spinclass): add RemoteBranchExists to git package
```

---

### Task 3: Update CreateBranchName to pass base as argument

**Files:**
- Modify: `packages/spinclass/internal/sweatfile/sweatfile.go:52-74`

**Step 1: Update the implementation**

Replace `CreateBranchName` in `sweatfile.go`:

```go
func (sweatfile Sweatfile) CreateBranchName(
	base string,
) (string, error) {
	if sweatfile.BranchNameCommand == "" {
		return base, nil
	}

	cmdComponents, err := shlex.Split(sweatfile.BranchNameCommand)
	if err != nil {
		return "", err
	}

	cmdComponents = append(cmdComponents, base)
	cmd := exec.Command(cmdComponents[0], cmdComponents[1:]...)
	cmd.Stderr = os.Stderr

	replacementBytes, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(bytes.TrimSpace(replacementBytes)), nil
}
```

Key changes: `base` appended as argument, stdin no longer connected to terminal, stdout no longer connected (was a bug — `cmd.Stdout = os.Stdout` conflicts with `cmd.Output()`).

**Step 2: Run existing tests**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/light-laurel --command go test ./packages/spinclass/...`
Expected: PASS (no tests directly exercise branch-name-command with stdin)

**Step 3: Commit**

```
fix(spinclass): pass base name as argument to branch-name-command
```

---

### Task 4: Update ResolvePath to accept args and detect existing branches

**Files:**
- Modify: `packages/spinclass/internal/worktree/worktree.go:17-54`
- Test: `packages/spinclass/internal/worktree/worktree_test.go`

**Step 1: Write failing tests**

Add to `worktree_test.go`:

```go
func TestResolvePathMultipleArgs(t *testing.T) {
	home := t.TempDir()
	repoPath := filepath.Join(home, "repos", "myrepo")

	rp, err := ResolvePath(sweatfile.Sweatfile{}, repoPath, []string{"this", "is", "the", "branch-name"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rp.Branch != "this-is-the-branch_name" {
		t.Errorf("Branch = %q, want %q", rp.Branch, "this-is-the-branch_name")
	}
	if rp.ExistingBranch != "" {
		t.Errorf("ExistingBranch = %q, want empty for new branch", rp.ExistingBranch)
	}
}

func TestResolvePathDetectsLocalBranch(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "myrepo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit(repoDir, "init")
	runGit(repoDir, "config", "user.email", "test@test.com")
	runGit(repoDir, "config", "user.name", "Test")
	runGit(repoDir, "commit", "--allow-empty", "-m", "initial")
	runGit(repoDir, "branch", "existing_branch")

	rp, err := ResolvePath(sweatfile.Sweatfile{}, repoDir, []string{"existing-branch"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// "existing-branch" sanitizes to "existing_branch" which matches the local branch
	if rp.ExistingBranch != "existing_branch" {
		t.Errorf("ExistingBranch = %q, want %q", rp.ExistingBranch, "existing_branch")
	}
	if rp.Branch != "existing_branch" {
		t.Errorf("Branch = %q, want %q", rp.Branch, "existing_branch")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/light-laurel --command go test -run "TestResolvePathMultipleArgs|TestResolvePathDetectsLocalBranch" ./packages/spinclass/internal/worktree/...`
Expected: compilation error — `ResolvePath` signature mismatch

**Step 3: Update ResolvedPath struct and ResolvePath**

In `worktree.go`, update the struct:

```go
type ResolvedPath struct {
	AbsPath        string // absolute filesystem path to the worktree
	RepoPath       string // absolute path to the parent git repo
	SessionKey     string // key for zmx/executor sessions (<repo-dirname>/<branch>)
	Branch         string // branch name
	ExistingBranch string // non-empty when an existing branch was detected
}
```

Replace `ResolvePath`:

```go
// ResolvePath resolves a worktree target from positional args.
//
// If args is empty, a random name is generated.
// Otherwise, args are sanitized into a branch name via SanitizeBranchName.
// The sanitized name is checked against existing local and remote branches
// (both pre- and post-sweatfile transformation).
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

	rawName := SanitizeBranchName(args)

	transformedName, err := sf.CreateBranchName(rawName)
	if err != nil {
		return ResolvedPath{}, err
	}

	branch, existingBranch := detectBranch(repoPath, rawName, transformedName)

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

// detectBranch checks for an existing branch matching rawName or
// transformedName, checking local branches first, then remote.
// Returns (branchToUse, existingBranch). existingBranch is empty
// when creating a new branch.
func detectBranch(repoPath, rawName, transformedName string) (string, string) {
	if git.BranchExists(repoPath, rawName) {
		return rawName, rawName
	}
	if transformedName != rawName && git.BranchExists(repoPath, transformedName) {
		return transformedName, transformedName
	}
	if git.RemoteBranchExists(repoPath, rawName) {
		return rawName, rawName
	}
	if transformedName != rawName && git.RemoteBranchExists(repoPath, transformedName) {
		return transformedName, transformedName
	}
	return transformedName, ""
}
```

**Step 4: Fix existing tests**

Update `TestResolvePathBranchName` and `TestResolvePathSessionKey` to use the new signature:

```go
func TestResolvePathBranchName(t *testing.T) {
	home := t.TempDir()
	repoPath := filepath.Join(home, "repos", "myrepo")

	rp, err := ResolvePath(sweatfile.Sweatfile{}, repoPath, []string{"feature-x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantBranch := "feature_x" // hyphens become underscores in snob-case
	wantAbs := filepath.Join(repoPath, WorktreesDir, wantBranch)
	if rp.AbsPath != wantAbs {
		t.Errorf("AbsPath = %q, want %q", rp.AbsPath, wantAbs)
	}
	if rp.Branch != wantBranch {
		t.Errorf("Branch = %q, want %q", rp.Branch, wantBranch)
	}
	if rp.RepoPath != repoPath {
		t.Errorf("RepoPath = %q, want %q", rp.RepoPath, repoPath)
	}
}

func TestResolvePathSessionKey(t *testing.T) {
	home := t.TempDir()
	repoPath := filepath.Join(home, "repos", "myrepo")

	rp, err := ResolvePath(sweatfile.Sweatfile{}, repoPath, []string{"feature-x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantKey := "myrepo/feature_x"
	if rp.SessionKey != wantKey {
		t.Errorf("SessionKey = %q, want %q", rp.SessionKey, wantKey)
	}
}
```

**Step 5: Run all worktree tests**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/light-laurel --command go test ./packages/spinclass/internal/worktree/...`
Expected: PASS

**Step 6: Commit**

```
feat(spinclass): update ResolvePath to accept args slice with branch detection
```

---

### Task 5: Update shop.go to use ResolvedPath.ExistingBranch

**Files:**
- Modify: `packages/spinclass/internal/shop/shop.go:22-50,52-67,126-165`
- Test: `packages/spinclass/internal/shop/shop_test.go`

**Step 1: Update Create and createWorktree signatures**

Remove the `existingBranch` parameter from both — use `worktreePath.ExistingBranch` instead:

```go
func Create(
	writer io.Writer,
	worktreePath worktree.ResolvedPath,
	verbose bool,
	format string,
	tapWriter *tap.Writer,
) error {
	existed, err := createWorktree(worktreePath, verbose)
	// ... rest unchanged
}

func createWorktree(worktreePath worktree.ResolvedPath, verbose bool) (bool, error) {
	existed := true

	if _, err := os.Stat(worktreePath.AbsPath); os.IsNotExist(err) {
		existed = false
		result, err := worktree.Create(worktreePath.RepoPath, worktreePath.AbsPath, worktreePath.ExistingBranch)
		if err != nil {
			return false, err
		}
		if verbose {
			logSweatfileResult(result)
		}
	}

	return existed, nil
}
```

**Step 2: Update New signature — remove existingBranch and claudeArgs**

```go
func New(w io.Writer, exec executor.Executor, rp worktree.ResolvedPath, format string, mergeOnClose bool, noAttach bool, verbose bool) error {
	var tw *tap.Writer
	if format == "tap" {
		tw = tap.NewWriter(w)
	}

	if err := pullMainWorktree(rp, tw); err != nil {
		return err
	}

	if err := Create(w, rp, verbose, format, tw); err != nil {
		return err
	}

	tp := tap.TestPoint{
		Description: "attach " + rp.Branch,
		Ok:          true,
	}

	if err := exec.Attach(rp.AbsPath, rp.SessionKey, nil, noAttach, &tp); err != nil {
		return fmt.Errorf("attach failed: %w", err)
	}

	if noAttach {
		if tw != nil {
			tw.SkipDiag(tp.Description, tp.Skip, tp.Diagnostics)
			tw.Plan()
		}
		return nil
	}

	interactive := isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())

	return closeShop(w, exec, rp, format, mergeOnClose, verbose, tw, interactive, nil, noAttach)
}
```

**Step 3: Fix all test call sites**

Update every call to `Create` and `New` in `shop_test.go`:

- `Create(&buf, rp, "", false, "tap", nil)` → `Create(&buf, rp, false, "tap", nil)`
- `New(&buf, mock, rp, "tap", nil, "", false, false, false)` → `New(&buf, mock, rp, "tap", false, false, false)`
- `New(&buf, mock, rp, "tap", nil, "", true, true, false)` → `New(&buf, mock, rp, "tap", true, true, false)`
- etc.

There are 8 call sites in shop_test.go to update. Remove the `claudeArgs` (nil) and `existingBranch` ("") parameters from each.

**Step 4: Run tests**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/light-laurel --command go test ./packages/spinclass/internal/shop/...`
Expected: PASS

**Step 5: Commit**

```
refactor(spinclass): remove existingBranch and claudeArgs from shop functions
```

---

### Task 6: Update main.go — remove -b flag and claude args

**Files:**
- Modify: `packages/spinclass/cmd/spinclass/main.go:26-99,310-315`

**Step 1: Update the new command and init**

Remove `newBranch` variable, update the `newCmd` handler to pass all args to `ResolvePath`, and remove the `-b` flag registration.

In the `var` block, remove `newBranch string`.

Update the `newCmd` `RunE`:

```go
RunE: func(cmd *cobra.Command, args []string) error {
	format := outputFormat
	if format == "" {
		format = "tap"
	}

	exec := executor.ZmxExecutor{}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	repoPath, err := worktree.DetectRepo(cwd)
	if err != nil {
		return err
	}

	hierarchy, err := sweatfile.LoadDefaultHierarchy()
	if err != nil {
		return err
	}

	resolvedPath, err := worktree.ResolvePath(hierarchy.Merged, repoPath, args)
	if err != nil {
		return err
	}

	return shop.New(
		os.Stdout,
		exec,
		resolvedPath,
		format,
		newMergeOnClose,
		newNoAttach,
		verbose,
	)
},
```

Update `Use` and `Long` on `newCmd`:

```go
Use:   "new [name parts...]",
Short: "Create (if needed) and attach to a worktree session",
Long:  `Create a worktree if it doesn't exist, then attach to a session. Name parts are joined into a sanitized branch name (snob-case). If an existing branch matches, it is checked out. If no name is provided, a random name is generated.`,
```

In `init()`, remove the line:
```go
newCmd.Flags().StringVarP(&newBranch, "branch", "b", "", "use an existing branch for the session")
```

**Step 2: Run all spinclass tests**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/light-laurel --command go test ./packages/spinclass/...`
Expected: PASS

**Step 3: Commit**

```
feat(spinclass): unify branch naming via positional args, remove -b flag
```

---

### Task 7: Update shell completions

**Files:**
- Modify: `packages/spinclass/completions/spinclass.bash-completion:19-23,29-31`
- Modify: `packages/spinclass/completions/spinclass.fish:97-105`

**Step 1: Update bash completions**

Remove the `--branch`/`-b` handler block (lines 19-23) and remove `--branch -b` from the `new` case flags list (line 30).

After edit, the `new` case should offer: `--merge-on-close --no-attach --format`

**Step 2: Update fish completions**

Remove the `-b`/`--branch` completion block (lines 97-105).

**Step 3: Commit**

```
fix(spinclass): update shell completions to remove -b/--branch flag
```

---

### Task 8: Final integration test

**Step 1: Build the package**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/light-laurel --command go build -o /dev/null ./packages/spinclass/...`
Expected: builds successfully

**Step 2: Run all spinclass tests**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/light-laurel --command go test ./packages/spinclass/...`
Expected: all PASS

**Step 3: Verify no other packages reference the removed parameters**

Run: `grep -r "newBranch\|existingBranch\|claudeArgs" packages/spinclass/` — should return nothing except possibly comments. The `existingBranch` parameter on `worktree.Create` still exists and is used from `ResolvedPath.ExistingBranch`.
