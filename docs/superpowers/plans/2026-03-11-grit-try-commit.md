# grit try_commit Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `try_commit` MCP tool to grit that stages, commits, and returns context (diff stats + post-commit status) in a single call.

**Architecture:** New handler in `packages/grit/internal/tools/try_commit.go` reusing existing `git.Run`, `git.ParseStatus`, `git.ParseDiffNumstat`, `git.ParseCommit`, and `git.DetectInProgressState`. New `TryCommitResult` type in `git/types.go`. Registered in `registry.go`.

**Tech Stack:** Go, go-mcp `command` package

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `packages/grit/internal/git/types.go` | Modify | Add `TryCommitResult` struct |
| `packages/grit/internal/tools/try_commit.go` | Create | Command registration + handler |
| `packages/grit/internal/tools/registry.go` | Modify | Wire `registerTryCommitCommands` |
| `packages/grit/internal/tools/try_commit_test.go` | Create | Integration test with temp git repo |

---

## Chunk 1: Implementation

### Task 1: Add TryCommitResult type

**Files:**
- Modify: `packages/grit/internal/git/types.go` (append after `RebaseResult`)

- [ ] **Step 1: Add the type**

Append to `packages/grit/internal/git/types.go`:

```go
type TryCommitResult struct {
	Commit CommitResult `json:"commit"`
	Staged []DiffStat   `json:"staged"`
	Status StatusResult `json:"status"`
}
```

- [ ] **Step 2: Verify it compiles**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/grit-commit-agent-flow-with-smaller-context --command go build ./packages/grit/...`
Expected: no errors

- [ ] **Step 3: Commit**

```
git add packages/grit/internal/git/types.go
git commit -m "feat(grit): add TryCommitResult type"
```

### Task 2: Write try_commit handler test

**Files:**
- Create: `packages/grit/internal/tools/try_commit_test.go`

- [ ] **Step 1: Write the test**

Create `packages/grit/internal/tools/try_commit_test.go`:

```go
package tools

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/friedenberg/grit/internal/git"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "commit.gpgSign", "false"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	return dir
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func initialCommit(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, dir, ".gitkeep", "")

	cmds := [][]string{
		{"git", "add", ".gitkeep"},
		{"git", "commit", "-m", "initial"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}
}

func TestHandleTryCommit(t *testing.T) {
	dir := setupTestRepo(t)
	writeFile(t, dir, "hello.txt", "hello world\n")

	args, _ := json.Marshal(map[string]any{
		"repo_path": dir,
		"message":   "add hello",
		"paths":     []string{"hello.txt"},
	})

	result, err := handleTryCommit(context.Background(), args, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsErr {
		t.Fatalf("result is error: %s", result.Text)
	}

	tcr, ok := result.JSON.(git.TryCommitResult)
	if !ok {
		t.Fatalf("unexpected result type: %T", result.JSON)
	}

	if tcr.Commit.Status != "committed" {
		t.Errorf("commit.status = %q, want %q", tcr.Commit.Status, "committed")
	}

	if tcr.Commit.Subject != "add hello" {
		t.Errorf("commit.subject = %q, want %q", tcr.Commit.Subject, "add hello")
	}

	if len(tcr.Staged) != 1 {
		t.Fatalf("staged len = %d, want 1", len(tcr.Staged))
	}

	if tcr.Staged[0].Path != "hello.txt" {
		t.Errorf("staged[0].path = %q, want %q", tcr.Staged[0].Path, "hello.txt")
	}

	if len(tcr.Status.Entries) != 0 {
		t.Errorf("status.entries len = %d, want 0 (clean after commit)", len(tcr.Status.Entries))
	}
}

func TestHandleTryCommitWithRemainingChanges(t *testing.T) {
	dir := setupTestRepo(t)
	writeFile(t, dir, "a.txt", "aaa\n")
	writeFile(t, dir, "b.txt", "bbb\n")

	args, _ := json.Marshal(map[string]any{
		"repo_path": dir,
		"message":   "add a only",
		"paths":     []string{"a.txt"},
	})

	result, err := handleTryCommit(context.Background(), args, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tcr, ok := result.JSON.(git.TryCommitResult)
	if !ok {
		t.Fatalf("unexpected result type: %T", result.JSON)
	}

	if tcr.Commit.Status != "committed" {
		t.Errorf("commit.status = %q, want %q", tcr.Commit.Status, "committed")
	}

	// b.txt should appear as untracked in post-commit status
	found := false
	for _, e := range tcr.Status.Entries {
		if e.Path == "b.txt" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected b.txt in post-commit status entries, got: %v", tcr.Status.Entries)
	}
}

func TestHandleTryCommitBadPaths(t *testing.T) {
	dir := setupTestRepo(t)

	args, _ := json.Marshal(map[string]any{
		"repo_path": dir,
		"message":   "commit nothing",
		"paths":     []string{"nonexistent.txt"},
	})

	result, err := handleTryCommit(context.Background(), args, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsErr {
		t.Errorf("expected error result for nonexistent paths")
	}
}

func TestHandleTryCommitNothingToCommit(t *testing.T) {
	dir := setupTestRepo(t)
	initialCommit(t, dir)
	writeFile(t, dir, "a.txt", "aaa\n")

	// Create initial commit with a.txt
	cmd := exec.Command("git", "add", "a.txt")
	cmd.Dir = dir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "add a")
	cmd.Dir = dir
	cmd.Run()

	// Try to commit a.txt again with no changes — commit should fail
	args, _ := json.Marshal(map[string]any{
		"repo_path": dir,
		"message":   "empty commit",
		"paths":     []string{"a.txt"},
	})

	result, err := handleTryCommit(context.Background(), args, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Spec: commit failure returns JSONResult (not error) with empty Commit
	if result.IsErr {
		t.Fatalf("expected structured result, got error: %s", result.Text)
	}

	tcr, ok := result.JSON.(git.TryCommitResult)
	if !ok {
		t.Fatalf("unexpected result type: %T", result.JSON)
	}

	// Commit field should be zero-value (empty)
	if tcr.Commit.Status != "" {
		t.Errorf("commit.status = %q, want empty", tcr.Commit.Status)
	}

	// Status should still be populated
	if tcr.Status.Branch.Head == "" {
		t.Errorf("expected status.branch.head to be populated")
	}
}
```

- [ ] **Step 2: Verify the test file compiles (it won't — handler doesn't exist yet)**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/grit-commit-agent-flow-with-smaller-context --command go vet ./packages/grit/internal/tools/`
Expected: compile error referencing `handleTryCommit`

- [ ] **Step 3: Commit the test**

```
git add packages/grit/internal/tools/try_commit_test.go
git commit -m "test(grit): add try_commit handler tests"
```

### Task 3: Implement try_commit handler

**Files:**
- Create: `packages/grit/internal/tools/try_commit.go`
- Modify: `packages/grit/internal/tools/registry.go` (add registration call)

- [ ] **Step 1: Create the handler file**

Create `packages/grit/internal/tools/try_commit.go`:

```go
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	"github.com/friedenberg/grit/internal/git"
)

func registerTryCommitCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:  "try_commit",
		Title: "Try Commit",
		Description: command.Description{
			Short: "Stage, commit, and return context in a single call. Replaces the status, diff, log, add, commit multi-tool cycle. Use this instead of calling those tools individually when creating commits in independent agent loops.",
		},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(false),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(false),
			OpenWorldHint:   protocol.BoolPtr(false),
		},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "message", Type: command.String, Description: "Commit message", Required: true},
			{Name: "paths", Type: command.Array, Description: "File paths to stage before committing", Required: true},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git commit"}, UseWhen: "creating a new commit"},
		},
		Run: handleTryCommit,
	})
}

func handleTryCommit(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		RepoPath string   `json:"repo_path"`
		Message  string   `json:"message"`
		Paths    []string `json:"paths"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	// Stage files
	addArgs := []string{"add", "--"}
	addArgs = append(addArgs, params.Paths...)

	if _, err := git.Run(ctx, params.RepoPath, addArgs...); err != nil {
		return command.TextErrorResult(fmt.Sprintf("git add: %v", err)), nil
	}

	// Capture staged diff stats before committing
	numstatOut, err := git.Run(ctx, params.RepoPath, "diff", "--numstat", "--cached")
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("git diff --numstat: %v", err)), nil
	}

	staged := git.ParseDiffNumstat(numstatOut)

	// Commit
	commitOut, commitErr := git.Run(ctx, params.RepoPath, "commit", "-m", params.Message)

	// Capture post-commit (or post-failure) status
	statusOut, err := git.Run(ctx, params.RepoPath, "status", "--porcelain=v2", "--branch")
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("git status: %v", err)), nil
	}

	status := git.ParseStatus(statusOut)

	state, err := git.DetectInProgressState(ctx, params.RepoPath)
	if err == nil && state != nil {
		status.State = state
	}

	// If commit failed, return structured result with empty commit + status context
	if commitErr != nil {
		return command.JSONResult(git.TryCommitResult{
			Staged: staged,
			Status: status,
		}), nil
	}

	return command.JSONResult(git.TryCommitResult{
		Commit: git.ParseCommit(commitOut),
		Staged: staged,
		Status: status,
	}), nil
}
```

- [ ] **Step 2: Register in registry.go**

Add `registerTryCommitCommands(app)` to `RegisterAll()` in `packages/grit/internal/tools/registry.go`, after `registerCommitCommands(app)`.

- [ ] **Step 3: Run tests**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/grit-commit-agent-flow-with-smaller-context --command go test -v ./packages/grit/internal/tools/`
Expected: all four `TestHandleTryCommit*` tests pass

- [ ] **Step 4: Run full grit test suite**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/grit-commit-agent-flow-with-smaller-context --command go test ./packages/grit/...`
Expected: all tests pass

- [ ] **Step 5: Commit**

```
git add packages/grit/internal/tools/try_commit.go packages/grit/internal/tools/registry.go
git commit -m "feat(grit): add try_commit tool"
```

### Task 4: Build and verify MCP tool registration

- [ ] **Step 1: Build grit**

Run: `nix build /Users/sfriedenberg/eng/repos/purse-first/.worktrees/grit-commit-agent-flow-with-smaller-context#grit`
Expected: build succeeds

- [ ] **Step 2: Verify try_commit appears in tools/list**

Run: `echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"0.0.1"}}}' | /Users/sfriedenberg/eng/repos/purse-first/.worktrees/grit-commit-agent-flow-with-smaller-context/result/bin/grit 2>/dev/null | head -1 && echo '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' | timeout 2 /Users/sfriedenberg/eng/repos/purse-first/.worktrees/grit-commit-agent-flow-with-smaller-context/result/bin/grit 2>/dev/null`

Expected: `try_commit` appears in the tool list with the correct description and all three required parameters.

- [ ] **Step 3: No commit needed — this is a verification step**
