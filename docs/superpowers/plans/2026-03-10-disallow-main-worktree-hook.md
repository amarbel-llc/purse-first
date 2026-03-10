# Disallow Main Worktree Hook Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the experimental `boundary-notify` PreToolUse hook with a narrower `hooks.disallow-main-worktree` flag that blocks tool calls targeting the main worktree when operating from a session worktree.

**Architecture:** The sweatfile gains a new `DisallowMainWorktree` boolean in the `Hooks` struct. The hook command resolves the main repo root via `git.CommonDir`, loads an extended hierarchy that includes the main repo's sweatfile, and denies tool calls whose paths resolve inside the main worktree. The `Experimental` struct and `boundary-notify` field are removed entirely.

**Tech Stack:** Go, TOML (sweatfile), Claude Code PreToolUse hook protocol

---

## Chunk 1: Sweatfile Changes

### Task 1: Add DisallowMainWorktree to Hooks struct and remove Experimental

**Files:**
- Modify: `packages/spinclass/internal/sweatfile/sweatfile.go`

- [ ] **Step 1: Write failing test for parsing disallow-main-worktree**

Add to `packages/spinclass/internal/sweatfile/sweatfile_test.go`:

```go
func TestParseHooksDisallowMainWorktree(t *testing.T) {
	input := `
[hooks]
disallow-main-worktree = true
`
	sf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sf.DisallowMainWorktreeEnabled() {
		t.Error("expected disallow-main-worktree to be enabled")
	}
}

func TestParseHooksDisallowMainWorktreeAbsent(t *testing.T) {
	sf, err := Parse([]byte(`git-excludes = [".claude/"]`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sf.DisallowMainWorktreeEnabled() {
		t.Error("expected disallow-main-worktree to be disabled when absent")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/spinclass-pretooluse-hooks-enforce-worktree --command go test -run TestParseHooksDisallowMainWorktree ./packages/spinclass/internal/sweatfile/...`

Expected: compilation error — `DisallowMainWorktreeEnabled` not defined.

- [ ] **Step 3: Implement the sweatfile changes**

In `packages/spinclass/internal/sweatfile/sweatfile.go`:

1. Add `DisallowMainWorktree *bool` to the `Hooks` struct with TOML tag `disallow-main-worktree`:

```go
type Hooks struct {
	Create              *string `toml:"create"`
	Stop                *string `toml:"stop"`
	DisallowMainWorktree *bool  `toml:"disallow-main-worktree"`
}
```

2. Add accessor method on `Sweatfile`:

```go
func (sf Sweatfile) DisallowMainWorktreeEnabled() bool {
	return sf.Hooks != nil &&
		sf.Hooks.DisallowMainWorktree != nil &&
		*sf.Hooks.DisallowMainWorktree
}
```

3. Remove the `Experimental` struct entirely (lines 20-22).
4. Remove the `Experimental` field from `Sweatfile` (line 33).
5. Remove the `BoundaryNotifyEnabled()` method (lines 50-54).

- [ ] **Step 4: Run tests to verify they pass**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/spinclass-pretooluse-hooks-enforce-worktree --command go test ./packages/spinclass/internal/sweatfile/...`

Expected: PASS (all sweatfile tests including the two new ones).

- [ ] **Step 5: Commit**

```
git add packages/spinclass/internal/sweatfile/sweatfile.go packages/spinclass/internal/sweatfile/sweatfile_test.go
git commit -m "feat(spinclass): add hooks.disallow-main-worktree, remove Experimental"
```

### Task 2: Update Merge to handle DisallowMainWorktree

**Files:**
- Modify: `packages/spinclass/internal/sweatfile/hierarchy.go` (Merge function, lines 149-168)

- [ ] **Step 1: Write failing tests for merge behavior**

Add to `packages/spinclass/internal/sweatfile/sweatfile_test.go`:

```go
func TestMergeDisallowMainWorktreeInherit(t *testing.T) {
	enabled := true
	base := Sweatfile{Hooks: &Hooks{DisallowMainWorktree: &enabled}}
	repo := Sweatfile{}
	merged := Merge(base, repo)
	if !merged.DisallowMainWorktreeEnabled() {
		t.Error("expected inherited disallow-main-worktree")
	}
}

func TestMergeDisallowMainWorktreeOverride(t *testing.T) {
	enabled := true
	disabled := false
	base := Sweatfile{Hooks: &Hooks{DisallowMainWorktree: &enabled}}
	repo := Sweatfile{Hooks: &Hooks{DisallowMainWorktree: &disabled}}
	merged := Merge(base, repo)
	if merged.DisallowMainWorktreeEnabled() {
		t.Error("expected overridden disallow-main-worktree to be disabled")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/spinclass-pretooluse-hooks-enforce-worktree --command go test -run TestMergeDisallowMainWorktree ./packages/spinclass/internal/sweatfile/...`

Expected: `TestMergeDisallowMainWorktreeInherit` fails (merge doesn't propagate the field).

- [ ] **Step 3: Add merge logic**

In `packages/spinclass/internal/sweatfile/hierarchy.go`, inside the `if repo.Hooks != nil` block (after the Stop merge at line 158), add:

```go
		if repo.Hooks.DisallowMainWorktree != nil {
			merged.Hooks.DisallowMainWorktree = repo.Hooks.DisallowMainWorktree
		}
```

Also remove the `Experimental` merge block (lines 161-168).

- [ ] **Step 4: Run tests to verify they pass**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/spinclass-pretooluse-hooks-enforce-worktree --command go test ./packages/spinclass/internal/sweatfile/...`

Expected: PASS.

- [ ] **Step 5: Commit**

```
git add packages/spinclass/internal/sweatfile/hierarchy.go packages/spinclass/internal/sweatfile/sweatfile_test.go
git commit -m "feat(spinclass): merge disallow-main-worktree, remove Experimental merge"
```

### Task 3: Add LoadWorktreeHierarchy for worktree-aware cascade

**Files:**
- Modify: `packages/spinclass/internal/sweatfile/hierarchy.go`
- Modify: `packages/spinclass/internal/sweatfile/sweatfile_test.go`

- [ ] **Step 1: Write failing test for worktree hierarchy**

Add to `packages/spinclass/internal/sweatfile/sweatfile_test.go`:

```go
func TestLoadWorktreeHierarchyMainRepoSweatfileIncluded(t *testing.T) {
	home := t.TempDir()
	mainRepo := filepath.Join(home, "eng", "repos", "myrepo")
	worktreeDir := filepath.Join(mainRepo, ".worktrees", "my-branch")
	os.MkdirAll(worktreeDir, 0o755)

	// Main repo sweatfile enables disallow-main-worktree
	writeSweatfile(t, filepath.Join(mainRepo, "sweatfile"),
		"[hooks]\ndisallow-main-worktree = true\n")

	result, err := LoadWorktreeHierarchy(home, mainRepo, worktreeDir)
	if err != nil {
		t.Fatalf("LoadWorktreeHierarchy returned error: %v", err)
	}

	if !result.Merged.DisallowMainWorktreeEnabled() {
		t.Error("expected disallow-main-worktree from main repo sweatfile")
	}
}

func TestLoadWorktreeHierarchyWorktreeOverridesMainRepo(t *testing.T) {
	home := t.TempDir()
	mainRepo := filepath.Join(home, "eng", "repos", "myrepo")
	worktreeDir := filepath.Join(mainRepo, ".worktrees", "my-branch")
	os.MkdirAll(worktreeDir, 0o755)

	// Main repo enables it
	writeSweatfile(t, filepath.Join(mainRepo, "sweatfile"),
		"[hooks]\ndisallow-main-worktree = true\n")

	// Worktree disables it
	writeSweatfile(t, filepath.Join(worktreeDir, "sweatfile"),
		"[hooks]\ndisallow-main-worktree = false\n")

	result, err := LoadWorktreeHierarchy(home, mainRepo, worktreeDir)
	if err != nil {
		t.Fatalf("LoadWorktreeHierarchy returned error: %v", err)
	}

	if result.Merged.DisallowMainWorktreeEnabled() {
		t.Error("expected worktree sweatfile to override main repo")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/spinclass-pretooluse-hooks-enforce-worktree --command go test -run TestLoadWorktreeHierarchy ./packages/spinclass/internal/sweatfile/...`

Expected: compilation error — `LoadWorktreeHierarchy` not defined.

- [ ] **Step 3: Implement LoadWorktreeHierarchy**

Add to `packages/spinclass/internal/sweatfile/hierarchy.go`:

```go
// LoadWorktreeHierarchy loads the sweatfile cascade for a worktree context.
// It delegates to LoadHierarchy for global → intermediate dirs → main repo,
// then appends the worktree's own sweatfile as the highest-priority layer.
func LoadWorktreeHierarchy(home, mainRepoRoot, worktreeDir string) (Hierarchy, error) {
	hierarchy, err := LoadHierarchy(home, mainRepoRoot)
	if err != nil {
		return Hierarchy{}, err
	}

	worktreePath := filepath.Join(filepath.Clean(worktreeDir), "sweatfile")
	sf, err := Load(worktreePath)
	if err != nil {
		return Hierarchy{}, err
	}

	_, found := fileExists(worktreePath)
	hierarchy.Sources = append(hierarchy.Sources, LoadSource{
		Path: worktreePath, Found: found, File: sf,
	})
	if found {
		hierarchy.Merged = Merge(hierarchy.Merged, sf)
	}

	return hierarchy, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/spinclass-pretooluse-hooks-enforce-worktree --command go test ./packages/spinclass/internal/sweatfile/...`

Expected: PASS.

- [ ] **Step 5: Commit**

```
git add packages/spinclass/internal/sweatfile/hierarchy.go packages/spinclass/internal/sweatfile/sweatfile_test.go
git commit -m "feat(spinclass): add LoadWorktreeHierarchy for worktree-aware cascade"
```

## Chunk 2: Hook Logic Replacement

### Task 4: Replace PreToolUse hook logic

**Files:**
- Modify: `packages/spinclass/internal/hooks/hooks.go`
- Modify: `packages/spinclass/internal/hooks/hooks_test.go`

- [ ] **Step 1: Write failing tests for new hook behavior**

Replace the boundary-notify tests in `packages/spinclass/internal/hooks/hooks_test.go` with:

```go
func TestDisallowMainWorktreeOffAllowsEverything(t *testing.T) {
	mainRepo := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(mainRepo, "secret.go")

	input := makeInput("Read", map[string]any{"file_path": target}, outside)

	var stdout bytes.Buffer
	err := Run(bytes.NewReader(input), &stdout, mainRepo, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.Len() != 0 {
		t.Errorf("expected no output when flag is off, got %q", stdout.String())
	}
}

func TestDisallowMainWorktreeOnDeniesMainRepoPath(t *testing.T) {
	mainRepo := t.TempDir()
	worktreeCwd := t.TempDir()
	target := filepath.Join(mainRepo, "main.go")

	input := makeInput("Read", map[string]any{"file_path": target}, worktreeCwd)

	var stdout bytes.Buffer
	err := Run(bytes.NewReader(input), &stdout, mainRepo, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.Len() == 0 {
		t.Fatal("expected deny output for path in main worktree")
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON, got %q: %v", stdout.String(), err)
	}

	hso, ok := result["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("expected hookSpecificOutput in output")
	}

	if hso["permissionDecision"] != "deny" {
		t.Errorf("expected permissionDecision deny, got %v", hso["permissionDecision"])
	}

	reason, ok := hso["reason"].(string)
	if !ok || reason == "" {
		t.Fatal("expected reason in output")
	}

	if !strings.Contains(reason, "main worktree") {
		t.Errorf("expected reason to mention main worktree, got %q", reason)
	}
}

func TestDisallowMainWorktreeOnAllowsWorktreePath(t *testing.T) {
	mainRepo := t.TempDir()
	worktreeCwd := t.TempDir()
	target := filepath.Join(worktreeCwd, "file.go")

	input := makeInput("Read", map[string]any{"file_path": target}, worktreeCwd)

	var stdout bytes.Buffer
	err := Run(bytes.NewReader(input), &stdout, mainRepo, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.Len() != 0 {
		t.Errorf("expected no output for worktree path, got %q", stdout.String())
	}
}

func TestDisallowMainWorktreeOnAllowsUnrelatedPath(t *testing.T) {
	mainRepo := t.TempDir()
	worktreeCwd := t.TempDir()
	unrelated := t.TempDir()
	target := filepath.Join(unrelated, "file.go")

	input := makeInput("Read", map[string]any{"file_path": target}, worktreeCwd)

	var stdout bytes.Buffer
	err := Run(bytes.NewReader(input), &stdout, mainRepo, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.Len() != 0 {
		t.Errorf("expected no output for unrelated path, got %q", stdout.String())
	}
}

func TestDisallowMainWorktreeEmptyMainRepoAllows(t *testing.T) {
	worktreeCwd := t.TempDir()
	target := filepath.Join(worktreeCwd, "file.go")

	input := makeInput("Read", map[string]any{"file_path": target}, worktreeCwd)

	var stdout bytes.Buffer
	err := Run(bytes.NewReader(input), &stdout, "", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.Len() != 0 {
		t.Errorf("expected no output with empty main repo, got %q", stdout.String())
	}
}

func TestDisallowMainWorktreeGlobInMainRepo(t *testing.T) {
	mainRepo := t.TempDir()
	worktreeCwd := t.TempDir()

	input := makeInput("Glob", map[string]any{"path": mainRepo}, worktreeCwd)

	var stdout bytes.Buffer
	err := Run(bytes.NewReader(input), &stdout, mainRepo, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.Len() == 0 {
		t.Fatal("expected deny output for Glob targeting main worktree")
	}
}

func TestDisallowMainWorktreeBashAbsolutePathInMainRepo(t *testing.T) {
	mainRepo := t.TempDir()
	worktreeCwd := t.TempDir()
	target := filepath.Join(mainRepo, "src/main.go")

	input := makeInput("Bash", map[string]any{"command": "cat " + target}, worktreeCwd)

	var stdout bytes.Buffer
	err := Run(bytes.NewReader(input), &stdout, mainRepo, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.Len() == 0 {
		t.Fatal("expected deny output for Bash command targeting main worktree")
	}
}

func TestDisallowMainWorktreeSymlinkResolution(t *testing.T) {
	mainRepo := t.TempDir()
	worktreeCwd := t.TempDir()

	// Create a file in main repo and a symlink to it from worktree
	target := filepath.Join(mainRepo, "real.go")
	os.WriteFile(target, []byte("package main"), 0o644)
	link := filepath.Join(worktreeCwd, "link.go")
	os.Symlink(target, link)

	input := makeInput("Read", map[string]any{"file_path": link}, worktreeCwd)

	var stdout bytes.Buffer
	err := Run(bytes.NewReader(input), &stdout, mainRepo, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.Len() == 0 {
		t.Fatal("expected deny output for symlink resolving to main worktree")
	}
}

func TestDisallowMainWorktreeNonExistentFileInMainRepo(t *testing.T) {
	mainRepo := t.TempDir()
	worktreeCwd := t.TempDir()

	// Parent dir exists in main repo, file does not
	subdir := filepath.Join(mainRepo, "src")
	os.MkdirAll(subdir, 0o755)
	target := filepath.Join(subdir, "new.go")

	input := makeInput("Write", map[string]any{"file_path": target}, worktreeCwd)

	var stdout bytes.Buffer
	err := Run(bytes.NewReader(input), &stdout, mainRepo, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.Len() == 0 {
		t.Fatal("expected deny output for new file targeting main worktree")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/spinclass-pretooluse-hooks-enforce-worktree --command go test -run TestDisallowMainWorktree ./packages/spinclass/internal/hooks/...`

Expected: compilation errors — `Run` signature has changed.

- [ ] **Step 3: Replace hook logic**

Rewrite `packages/spinclass/internal/hooks/hooks.go`:

1. Change the `Run` function signature. Remove `boundary`, `allowed`, `boundaryNotify` params. Add `mainRepoRoot` and `disallowMainWorktree`:

```go
func Run(r io.Reader, w io.Writer, mainRepoRoot string, disallowMainWorktree bool) error {
	var input hookInput
	if err := json.NewDecoder(r).Decode(&input); err != nil {
		return fmt.Errorf("decoding hook input: %w", err)
	}

	switch input.HookEventName {
	case "Stop":
		return runStopHook(input, w)
	default:
		return runPreToolUse(input, w, mainRepoRoot, disallowMainWorktree)
	}
}
```

2. Replace `runPreToolUse`:

```go
func runPreToolUse(input hookInput, w io.Writer, mainRepoRoot string, disallowMainWorktree bool) error {
	if !disallowMainWorktree || mainRepoRoot == "" {
		return nil
	}

	mainRepoRoot = resolvePath(mainRepoRoot)

	paths := extractPaths(input)
	if paths == nil {
		return nil
	}

	for _, p := range paths {
		if isInsideMainWorktree(p, mainRepoRoot) {
			output := map[string]any{
				"hookSpecificOutput": map[string]any{
					"hookEventName":      "PreToolUse",
					"permissionDecision": "deny",
					"reason": fmt.Sprintf(
						"Path %s is in the main worktree (%s). Restrict operations to the session worktree.",
						p, mainRepoRoot,
					),
				},
			}
			return json.NewEncoder(w).Encode(output)
		}
	}

	return nil
}
```

3. Replace `isInsideBoundary` and `isInsideAllowed` with:

```go
func isInsideMainWorktree(path, mainRepoRoot string) bool {
	resolved := resolvePath(path)
	return resolved == mainRepoRoot || strings.HasPrefix(resolved, mainRepoRoot+string(filepath.Separator))
}
```

4. Remove `isInsideBoundary` and `isInsideAllowed` functions.

5. Remove the `"fmt"` import reference to `evalOrClean` if no longer used. Keep `resolvePath` (still used). Remove `evalOrClean` (thin wrapper, inline `resolvePath` calls directly).

- [ ] **Step 4: Update old tests to match new Run signature**

The stop hook tests (`TestStopHookEventRouteApproves`, `TestStopHookBlocksOnFailure`, `TestStopHookApprovesOnSecondInvocation`, `TestStopHookApprovesOnSuccess`) call `Run` with the old signature. Update each to use the new signature:

```go
// Old:
err := Run(bytes.NewReader(input), &out, "", nil, false)
// New:
err := Run(bytes.NewReader(input), &out, "", false)
```

Remove the old boundary-notify tests: `TestViolationWritesJSONApproval`, `TestNoViolationProducesNoOutput`, `TestBoundaryNotifyDisabledProducesNoOutput`, `TestNoBoundaryProducesNoOutput`.

- [ ] **Step 5: Run all hook tests**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/spinclass-pretooluse-hooks-enforce-worktree --command go test -v ./packages/spinclass/internal/hooks/...`

Expected: all tests PASS.

- [ ] **Step 6: Commit**

```
git add packages/spinclass/internal/hooks/hooks.go packages/spinclass/internal/hooks/hooks_test.go
git commit -m "feat(spinclass): replace boundary-notify with disallow-main-worktree deny"
```

### Task 5: Update hook command to use new hierarchy and Run signature

**Files:**
- Modify: `packages/spinclass/internal/hooks/cmd.go`

- [ ] **Step 1: Update cmd.go**

Replace the `RunE` function body in `NewHooksCmd`:

```go
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return nil
			}

			if !worktree.IsWorktree(cwd) {
				toplevel, err := gitToplevel(cwd)
				if err != nil {
					return nil
				}
				if !worktree.IsWorktree(toplevel) {
					// Not in a worktree — run with flag off
					return Run(os.Stdin, os.Stdout, "", false)
				}
				cwd = toplevel
			}

			mainRepoRoot, err := git.CommonDir(cwd)
			if err != nil {
				return nil
			}

			home, _ := os.UserHomeDir()
			var disallowMainWorktree bool
			if home != "" {
				result, err := sweatfile.LoadWorktreeHierarchy(home, mainRepoRoot, cwd)
				if err == nil {
					disallowMainWorktree = result.Merged.DisallowMainWorktreeEnabled()
				}
			}

			return Run(os.Stdin, os.Stdout, mainRepoRoot, disallowMainWorktree)
		},
```

Add import for `"github.com/amarbel-llc/spinclass/internal/git"` and remove the now-unused `detectBoundary` function, the `"path/filepath"` import (if no longer needed), and the allowed list construction.

The `gitToplevel` helper is still needed for the worktree detection logic.

- [ ] **Step 2: Verify the package compiles**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/spinclass-pretooluse-hooks-enforce-worktree --command go build ./packages/spinclass/...`

Expected: compiles with no errors.

- [ ] **Step 3: Run all spinclass tests**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/spinclass-pretooluse-hooks-enforce-worktree --command go test ./packages/spinclass/...`

Expected: all tests PASS.

- [ ] **Step 4: Commit**

```
git add packages/spinclass/internal/hooks/cmd.go
git commit -m "feat(spinclass): wire hook command to disallow-main-worktree"
```

## Chunk 3: Cleanup

### Task 6: Update FDR-0004

**Files:**
- Modify: `docs/features/0004-worktree-boundary-enforcement.md`

- [ ] **Step 1: Update FDR status and content**

Update the YAML front matter status from `experimental` to `superseded`. Add a note at the top of the document body pointing to the new behavior:

```markdown
> **Superseded:** This feature has been replaced by `hooks.disallow-main-worktree`.
> See `docs/superpowers/specs/2026-03-10-disallow-main-worktree-hook-design.md`.
```

- [ ] **Step 2: Commit**

```
git add docs/features/0004-worktree-boundary-enforcement.md
git commit -m "docs(spinclass): mark FDR-0004 as superseded by disallow-main-worktree"
```

### Task 7: Final verification

- [ ] **Step 1: Run full spinclass test suite**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/spinclass-pretooluse-hooks-enforce-worktree --command go test -v ./packages/spinclass/...`

Expected: all tests PASS, no references to `boundary-notify` or `Experimental` remain.

- [ ] **Step 2: Grep for stale references**

Run: `grep -r 'BoundaryNotify\|boundary-notify\|Experimental' packages/spinclass/`

Expected: no matches.

- [ ] **Step 3: Build the package**

Run: `nix build /Users/sfriedenberg/eng/repos/purse-first/.worktrees/spinclass-pretooluse-hooks-enforce-worktree#spinclass`

Expected: builds successfully.
