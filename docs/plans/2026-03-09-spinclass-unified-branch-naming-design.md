# Spinclass Unified Branch Naming Design

## Problem

`spinclass new` has two separate mechanisms for specifying branches: a single
positional argument for new branches and `-b/--branch` for existing branches.
Branch names must be typed exactly, with no character sanitization. Multi-word
descriptions require manual hyphenation.

## Solution

Unify positional arguments into a single branch-naming flow:

1. All positional args are joined into a "snob-case" branch name
2. The name is checked against existing local and remote branches
3. If found, the worktree checks out the existing branch; otherwise creates new
4. The `-b` flag and claude args passthrough are removed

## Snob-Case Transformation

`SanitizeBranchName(parts []string) string` in `worktree/namegen.go`:

1. For each part, replace hyphens with underscores
2. Join parts with hyphens
3. Lowercase the result
4. Strip git-invalid characters: control chars, `~`, `^`, `:`, `\`, `..`,
   `@{`, `?`, `*`, `[`, leading/trailing `.`, trailing `.lock`
5. Collapse consecutive hyphens and underscores
6. Strip leading/trailing hyphens and underscores

Examples:
- `["this", "is", "the", "branch-name"]` → `this-is-the-branch_name`
- `["Fix", "AUTH", "Bug"]` → `fix-auth-bug`
- `["feature/login"]` → `feature/login` (slashes preserved for branch hierarchy)

## ResolvePath Changes

Signature changes from `(sweatfile, repoPath, branch string)` to
`(sweatfile, repoPath string, args []string)`.

Flow:
1. `len(args) == 0` → `RandomName(repoPath)`, create new branch
2. `rawName := SanitizeBranchName(args)`
3. `transformedName := sweatfile.CreateBranchName(rawName)` (base passed as CLI
   arg to command)
4. Branch detection order:
   - `git.BranchExists(repoPath, rawName)` → existing, use rawName
   - `git.BranchExists(repoPath, transformedName)` → existing, use
     transformedName
   - `git.RemoteBranchExists(repoPath, rawName)` → existing, use rawName
   - `git.RemoteBranchExists(repoPath, transformedName)` → existing, use
     transformedName
   - No match → create new with transformedName
5. `ResolvedPath.ExistingBranch` set when an existing branch is detected

## CreateBranchName Changes

`CreateBranchName(base string)` passes `base` as a CLI argument to the
configured branch-name-command, rather than connecting stdin to the terminal.
Stdout is captured as the result. When no command is configured, returns `base`
unchanged.

## ResolvedPath Struct

New field `ExistingBranch string`. When non-empty, `worktree.Create` passes it
to `git worktree add <path> <branch>` to check out the existing branch.

## CLI Changes

- Remove `-b/--branch` flag from `newCmd`
- Remove `newBranch` variable
- Remove `claudeArgs` — all positional args are branch name parts
- Pass `args` to `ResolvePath` directly

## shop.go Changes

- `New` drops `existingBranch` and `claudeArgs` parameters
- Existing branch info comes from `ResolvedPath.ExistingBranch`
- Command construction drops claude args

## git.go Addition

`RemoteBranchExists(repoPath, branch string) bool` checks
`refs/remotes/origin/<branch>` via `git rev-parse --verify`.

## Files Changed

1. `worktree/namegen.go` — add `SanitizeBranchName`
2. `worktree/worktree.go` — modify `ResolvePath`, add `ExistingBranch` field
3. `git/git.go` — add `RemoteBranchExists`
4. `sweatfile/sweatfile.go` — modify `CreateBranchName` arg passing
5. `cmd/spinclass/main.go` — remove `-b`, remove claudeArgs
6. `shop/shop.go` — remove params, use `ResolvedPath.ExistingBranch`
7. `shop/shop_test.go` — update test signatures
8. `completions/completions.go` — remove `-b` references if present
