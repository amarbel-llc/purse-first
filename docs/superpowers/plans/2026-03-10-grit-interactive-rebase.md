# Grit Interactive Rebase Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two new MCP tools (`interactive_rebase_plan` and `interactive_rebase_execute`) to grit that enable programmatic interactive rebasing via `GIT_SEQUENCE_EDITOR`.

**Architecture:** Two-step workflow. `interactive_rebase_plan` is read-only — returns commits between upstream and HEAD as JSON. `interactive_rebase_execute` writes a temp shell script, sets `GIT_SEQUENCE_EDITOR` to it, and runs `git rebase -i`. Reword messages are handled via a second temp script set as `GIT_EDITOR`. Conflict handling reuses the existing `rebase` tool's continue/abort/skip flow.

**Tech Stack:** Go, `libs/go-mcp/command` framework, BATS integration tests.

---

## File Structure

| File | Responsibility |
|------|----------------|
| `packages/grit/internal/git/exec.go` | Add `RunWithEnv` — variant of `Run` accepting extra env vars |
| `packages/grit/internal/git/types.go` | Add `InteractiveRebasePlan` and `TodoEntry` types |
| `packages/grit/internal/tools/interactive_rebase.go` | Both tool registrations and handlers |
| `packages/grit/internal/tools/registry.go` | Wire up `registerInteractiveRebaseCommands` |
| `packages/grit/zz-tests_bats/interactive_rebase_mcp.bats` | Integration tests |

---

## Chunk 1: Foundation

### Task 1: Add `RunWithEnv` to git exec layer

**Files:**
- Modify: `packages/grit/internal/git/exec.go`

- [ ] **Step 1: Write test for `RunWithEnv`**

Create `packages/grit/internal/git/exec_test.go`:

```go
package git

import (
	"context"
	"strings"
	"testing"
)

func TestRunWithEnvPassesExtraVars(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	// Init a repo so git commands work
	if _, err := Run(ctx, dir, "init"); err != nil {
		t.Fatal(err)
	}

	// GIT_AUTHOR_NAME is an env var git respects
	out, err := RunWithEnv(ctx, dir, []string{"GIT_AUTHOR_NAME=TestBot"}, "var", "GIT_AUTHOR_IDENT")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "TestBot") {
		t.Errorf("expected output to contain TestBot, got: %s", out)
	}
}

func TestRunWithEnvPreservesBaseEnv(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	if _, err := Run(ctx, dir, "init"); err != nil {
		t.Fatal(err)
	}

	// RunWithEnv with no extra vars should behave like Run
	out, err := RunWithEnv(ctx, dir, nil, "status")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "On branch") {
		t.Errorf("expected status output, got: %s", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `nix develop --command go test -run TestRunWithEnv ./packages/grit/internal/git/...`
Expected: FAIL — `RunWithEnv` undefined.

- [ ] **Step 3: Implement `RunWithEnv`**

In `packages/grit/internal/git/exec.go`, refactor `Run` to delegate to `RunWithEnv`:

```go
func RunWithEnv(ctx context.Context, dir string, extraEnv []string, args ...string) (string, error) {
	if strings.ContainsRune(dir, 0) {
		return "", fmt.Errorf("dir contains null byte")
	}

	for _, arg := range args {
		if strings.ContainsRune(arg, 0) {
			return "", fmt.Errorf("argument contains null byte")
		}
	}

	// Build set of env var names that extraEnv overrides
	overridden := make(map[string]bool)
	for _, env := range extraEnv {
		if k, _, ok := strings.Cut(env, "="); ok {
			overridden[k] = true
		}
	}

	// Base defaults — skip any that extraEnv overrides
	baseDefaults := []string{
		"GIT_TERMINAL_PROMPT=0",
		"GIT_EDITOR=true",
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	for _, d := range baseDefaults {
		if k, _, ok := strings.Cut(d, "="); ok && overridden[k] {
			continue
		}
		cmd.Env = append(cmd.Env, d)
	}
	cmd.Env = append(cmd.Env, extraEnv...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		limited := output.LimitStderr(stderr.String())
		return "", fmt.Errorf("git %v: %w: %s", args, err, limited.Content)
	}

	return stdout.String(), nil
}

func Run(ctx context.Context, dir string, args ...string) (string, error) {
	return RunWithEnv(ctx, dir, nil, args...)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `nix develop --command go test -run TestRunWithEnv ./packages/grit/internal/git/...`
Expected: PASS

- [ ] **Step 5: Run existing grit tests to verify no regressions**

Run: `just test-grit`
Expected: All existing tests pass.

- [ ] **Step 6: Commit**

```bash
git add packages/grit/internal/git/exec.go packages/grit/internal/git/exec_test.go
git commit -m "feat(grit): add RunWithEnv for custom environment variables"
```

---

### Task 2: Add types for interactive rebase

**Files:**
- Modify: `packages/grit/internal/git/types.go`

- [ ] **Step 1: Add types**

Append to `packages/grit/internal/git/types.go`:

```go
type TodoEntry struct {
	Action  string `json:"action"`
	Hash    string `json:"hash"`
	Message string `json:"message,omitempty"`
}

type InteractiveRebasePlan struct {
	Status   string     `json:"status"`
	Branch   string     `json:"branch,omitempty"`
	Upstream string     `json:"upstream,omitempty"`
	Commits  []LogEntry `json:"commits"`
}
```

Note: `Commits` reuses the existing `LogEntry` type (has `Hash` and `Subject`).

- [ ] **Step 2: Verify compilation**

Run: `nix develop --command go build ./packages/grit/...`
Expected: Compiles without errors.

- [ ] **Step 3: Commit**

```bash
git add packages/grit/internal/git/types.go
git commit -m "feat(grit): add TodoEntry and InteractiveRebasePlan types"
```

---

## Chunk 2: Plan Tool

### Task 3: Implement `interactive_rebase_plan` tool

**Files:**
- Create: `packages/grit/internal/tools/interactive_rebase.go`
- Modify: `packages/grit/internal/tools/registry.go`

- [ ] **Step 1: Write BATS test for plan tool**

Create `packages/grit/zz-tests_bats/interactive_rebase_mcp.bats`:

```bash
#! /usr/bin/env bats

setup() {
  load "$(dirname "$BATS_TEST_FILE")/common.bash"
  export output
}

teardown() {
  teardown_test_home
}

# Helper: create a repo with 3 commits on feature branch ahead of main
setup_multi_commit_scenario() {
  setup_test_repo

  # Create feature branch with multiple commits
  git -C "$TEST_REPO" checkout -b feature
  echo "first" > "$TEST_REPO/first.txt"
  git -C "$TEST_REPO" add first.txt
  git -C "$TEST_REPO" commit -m "feature: add first"

  echo "second" > "$TEST_REPO/second.txt"
  git -C "$TEST_REPO" add second.txt
  git -C "$TEST_REPO" commit -m "feature: add second"

  echo "third" > "$TEST_REPO/third.txt"
  git -C "$TEST_REPO" add third.txt
  git -C "$TEST_REPO" commit -m "feature: add third"
}

function plan_returns_commit_list { # @test
  setup_multi_commit_scenario
  run run_grit_mcp "interactive_rebase_plan" "$(printf '{"repo_path":"%s","upstream":"main"}' "$TEST_REPO")"
  assert_success

  local status
  status=$(echo "$output" | jq -r '.status')
  assert_equal "$status" "plan"

  local count
  count=$(echo "$output" | jq '.commits | length')
  assert_equal "$count" "3"

  # Commits should be in chronological order (oldest first)
  local first_subject
  first_subject=$(echo "$output" | jq -r '.commits[0].subject')
  assert_equal "$first_subject" "feature: add first"
}

function plan_up_to_date { # @test
  setup_test_repo
  git -C "$TEST_REPO" checkout -b feature
  run run_grit_mcp "interactive_rebase_plan" "$(printf '{"repo_path":"%s","upstream":"main"}' "$TEST_REPO")"
  assert_success

  local status
  status=$(echo "$output" | jq -r '.status')
  assert_equal "$status" "up_to_date"

  local count
  count=$(echo "$output" | jq '.commits | length')
  assert_equal "$count" "0"
}

function plan_blocked_on_main { # @test
  setup_test_repo
  run run_grit_mcp "interactive_rebase_plan" "$(printf '{"repo_path":"%s","upstream":"HEAD~1"}' "$TEST_REPO")"
  assert_success
  assert_output --partial "blocked"
}
```

- [ ] **Step 2: Create `interactive_rebase.go` with plan handler**

Create `packages/grit/internal/tools/interactive_rebase.go`:

```go
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	"github.com/friedenberg/grit/internal/git"
)

func registerInteractiveRebaseCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "interactive_rebase_plan",
		Title:       "Plan Interactive Rebase",
		Description: command.Description{Short: "Get the commit list for an interactive rebase (blocked on main/master for safety)"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(true),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(true),
			OpenWorldHint:   protocol.BoolPtr(false),
		},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "upstream", Type: command.String, Description: "Ref to rebase onto (branch, tag, commit)", Required: true},
		},
		Run: handleInteractiveRebasePlan,
	})
}

func handleInteractiveRebasePlan(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		RepoPath string `json:"repo_path"`
		Upstream string `json:"upstream"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	// Determine current branch
	branchOut, err := git.Run(ctx, params.RepoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("failed to determine current branch: %v", err)), nil
	}
	branch := strings.TrimSpace(branchOut)

	// Safety: block on main/master
	if branch == "main" || branch == "master" {
		return command.TextErrorResult("interactive rebase on main/master is blocked for safety"), nil
	}

	// Get commits between upstream and HEAD in chronological order
	out, err := git.Run(ctx, params.RepoPath,
		"log", "--reverse", "--format=%H%x00%s",
		fmt.Sprintf("%s..HEAD", params.Upstream),
	)
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("git log: %v", err)), nil
	}

	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return command.JSONResult(git.InteractiveRebasePlan{
			Status:   "up_to_date",
			Branch:   branch,
			Upstream: params.Upstream,
			Commits:  []git.LogEntry{},
		}), nil
	}

	lines := strings.Split(trimmed, "\n")
	commits := make([]git.LogEntry, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(line, "\x00", 2)
		if len(parts) != 2 {
			continue
		}
		commits = append(commits, git.LogEntry{
			Hash:    parts[0],
			Subject: parts[1],
		})
	}

	return command.JSONResult(git.InteractiveRebasePlan{
		Status:   "plan",
		Branch:   branch,
		Upstream: params.Upstream,
		Commits:  commits,
	}), nil
}
```

- [ ] **Step 3: Register the new tools in `registry.go`**

In `packages/grit/internal/tools/registry.go`, add `registerInteractiveRebaseCommands(app)` after `registerRebaseCommands(app)`.

- [ ] **Step 4: Run Go tests to verify compilation**

Run: `just test-grit`
Expected: PASS — no new Go tests, but confirms compilation.

- [ ] **Step 5: Build grit and run BATS tests**

Run: `just test-grit-bats`
Expected: Existing rebase tests pass. New interactive rebase tests pass.

- [ ] **Step 6: Commit**

```bash
git add packages/grit/internal/tools/interactive_rebase.go \
       packages/grit/internal/tools/registry.go \
       packages/grit/zz-tests_bats/interactive_rebase_mcp.bats
git commit -m "feat(grit): add interactive_rebase_plan tool"
```

---

## Chunk 3: Execute Tool

### Task 4: Implement `interactive_rebase_execute` tool

**Files:**
- Modify: `packages/grit/internal/tools/interactive_rebase.go`

- [ ] **Step 1: Add BATS tests for execute tool**

Append to `packages/grit/zz-tests_bats/interactive_rebase_mcp.bats`:

```bash
function execute_squash_commits { # @test
  setup_multi_commit_scenario

  # Get the commit hashes
  local hash1 hash2 hash3
  hash1=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '1p')
  hash2=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '2p')
  hash3=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '3p')

  # Squash second into first, pick third
  run run_grit_mcp "interactive_rebase_execute" "$(printf '{"repo_path":"%s","upstream":"main","todo":[{"action":"pick","hash":"%s"},{"action":"squash","hash":"%s"},{"action":"pick","hash":"%s"}]}' "$TEST_REPO" "$hash1" "$hash2" "$hash3")"
  assert_success

  local status
  status=$(echo "$output" | jq -r '.status')
  assert_equal "$status" "completed"

  # Should have 2 commits after squash (was 3)
  local count
  count=$(git -C "$TEST_REPO" log --oneline main..HEAD | wc -l | tr -d ' ')
  assert_equal "$count" "2"
}

function execute_drop_commit { # @test
  setup_multi_commit_scenario

  local hash1 hash3
  hash1=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '1p')
  hash3=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '3p')

  # Pick first and third, implicitly drop second
  run run_grit_mcp "interactive_rebase_execute" "$(printf '{"repo_path":"%s","upstream":"main","todo":[{"action":"pick","hash":"%s"},{"action":"pick","hash":"%s"}]}' "$TEST_REPO" "$hash1" "$hash3")"
  assert_success

  local status
  status=$(echo "$output" | jq -r '.status')
  assert_equal "$status" "completed"

  # Should have 2 commits (dropped one)
  local count
  count=$(git -C "$TEST_REPO" log --oneline main..HEAD | wc -l | tr -d ' ')
  assert_equal "$count" "2"

  # second.txt should not exist (commit was dropped)
  assert [ ! -f "$TEST_REPO/second.txt" ]
}

function execute_reorder_commits { # @test
  setup_multi_commit_scenario

  local hash1 hash2 hash3
  hash1=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '1p')
  hash2=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '2p')
  hash3=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '3p')

  # Reverse the order: third, second, first
  run run_grit_mcp "interactive_rebase_execute" "$(printf '{"repo_path":"%s","upstream":"main","todo":[{"action":"pick","hash":"%s"},{"action":"pick","hash":"%s"},{"action":"pick","hash":"%s"}]}' "$TEST_REPO" "$hash3" "$hash2" "$hash1")"
  assert_success

  local status
  status=$(echo "$output" | jq -r '.status')
  assert_equal "$status" "completed"

  # First commit should now be "feature: add third"
  local first_subject
  first_subject=$(git -C "$TEST_REPO" log --reverse --format=%s main..HEAD | head -1)
  assert_equal "$first_subject" "feature: add third"
}

function execute_reword_commit { # @test
  setup_multi_commit_scenario

  local hash1 hash2 hash3
  hash1=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '1p')
  hash2=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '2p')
  hash3=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '3p')

  # Reword the first commit
  run run_grit_mcp "interactive_rebase_execute" "$(printf '{"repo_path":"%s","upstream":"main","todo":[{"action":"reword","hash":"%s","message":"renamed first commit"},{"action":"pick","hash":"%s"},{"action":"pick","hash":"%s"}]}' "$TEST_REPO" "$hash1" "$hash2" "$hash3")"
  assert_success

  local status
  status=$(echo "$output" | jq -r '.status')
  assert_equal "$status" "completed"

  # First commit should have the new message
  local first_subject
  first_subject=$(git -C "$TEST_REPO" log --reverse --format=%s main..HEAD | head -1)
  assert_equal "$first_subject" "renamed first commit"
}

function execute_validates_squash_not_first { # @test
  setup_multi_commit_scenario

  local hash1 hash2
  hash1=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '1p')
  hash2=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '2p')

  # squash as first action should fail validation
  run run_grit_mcp "interactive_rebase_execute" "$(printf '{"repo_path":"%s","upstream":"main","todo":[{"action":"squash","hash":"%s"},{"action":"pick","hash":"%s"}]}' "$TEST_REPO" "$hash1" "$hash2")"
  assert_success
  assert_output --partial "cannot be the first"
}

function execute_validates_reword_needs_message { # @test
  setup_multi_commit_scenario

  local hash1
  hash1=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '1p')

  # reword without message should fail validation
  run run_grit_mcp "interactive_rebase_execute" "$(printf '{"repo_path":"%s","upstream":"main","todo":[{"action":"reword","hash":"%s"}]}' "$TEST_REPO" "$hash1")"
  assert_success
  assert_output --partial "message"
}

function execute_blocked_on_main { # @test
  setup_test_repo
  run run_grit_mcp "interactive_rebase_execute" "$(printf '{"repo_path":"%s","upstream":"HEAD~1","todo":[{"action":"pick","hash":"abc"}]}' "$TEST_REPO")"
  assert_success
  assert_output --partial "blocked"
}

function execute_conflict_returns_conflict_status { # @test
  setup_test_repo
  git -C "$TEST_REPO" checkout -b feature
  echo "feature change" > "$TEST_REPO/file.txt"
  git -C "$TEST_REPO" add file.txt
  git -C "$TEST_REPO" commit -m "feature: modify file"

  echo "more" > "$TEST_REPO/more.txt"
  git -C "$TEST_REPO" add more.txt
  git -C "$TEST_REPO" commit -m "feature: add more"

  git -C "$TEST_REPO" checkout main
  echo "main change" > "$TEST_REPO/file.txt"
  git -C "$TEST_REPO" add file.txt
  git -C "$TEST_REPO" commit -m "main: modify file"
  git -C "$TEST_REPO" checkout feature

  local hash1 hash2
  hash1=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '1p')
  hash2=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '2p')

  run run_grit_mcp "interactive_rebase_execute" "$(printf '{"repo_path":"%s","upstream":"main","todo":[{"action":"pick","hash":"%s"},{"action":"pick","hash":"%s"}]}' "$TEST_REPO" "$hash1" "$hash2")"
  assert_success

  local status
  status=$(echo "$output" | jq -r '.status')
  assert_equal "$status" "conflict"
}

function execute_empty_todo_rejected { # @test
  setup_multi_commit_scenario
  run run_grit_mcp "interactive_rebase_execute" "$(printf '{"repo_path":"%s","upstream":"main","todo":[]}' "$TEST_REPO")"
  assert_success
  assert_output --partial "must not be empty"
}

function execute_rejects_when_rebase_in_progress { # @test
  setup_conflict_scenario
  # Start a regular rebase that will conflict
  git -C "$TEST_REPO" rebase main || true

  local hash1
  hash1=$(git -C "$TEST_REPO" log --reverse --format=%H main..ORIG_HEAD | sed -n '1p')

  run run_grit_mcp "interactive_rebase_execute" "$(printf '{"repo_path":"%s","upstream":"main","todo":[{"action":"pick","hash":"%s"}]}' "$TEST_REPO" "$hash1")"
  assert_success
  assert_output --partial "already in progress"
}
```

- [ ] **Step 2: Implement the execute handler**

Add to `packages/grit/internal/tools/interactive_rebase.go` — register the second command in `registerInteractiveRebaseCommands` and add the handler:

```go
// In registerInteractiveRebaseCommands, add after the plan command:
app.AddCommand(&command.Command{
    Name:        "interactive_rebase_execute",
    Title:       "Execute Interactive Rebase",
    Description: command.Description{Short: "Execute an interactive rebase with a structured todo list (blocked on main/master for safety)"},
    Annotations: &protocol.ToolAnnotations{
        ReadOnlyHint:    protocol.BoolPtr(false),
        DestructiveHint: protocol.BoolPtr(true),
        IdempotentHint:  protocol.BoolPtr(false),
        OpenWorldHint:   protocol.BoolPtr(false),
    },
    Params: []command.Param{
        {Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
        {Name: "upstream", Type: command.String, Description: "Ref to rebase onto", Required: true},
        {Name: "todo", Type: command.Array, Description: "Ordered list of {action, hash, message?} objects. Actions: pick, reword, squash, fixup, drop", Required: true},
        {Name: "autostash", Type: command.Bool, Description: "Automatically stash/unstash uncommitted changes"},
    },
    // Note: the existing rebase tool maps "git rebase" which also prefix-matches
    // "git rebase -i". This overlap is benign — both mappings deny the Bash
    // invocation and redirect to an MCP tool. The UseWhen message directs the
    // caller to the correct tool.
    MapsTools: []command.ToolMapping{
        {Replaces: "Bash", CommandPrefixes: []string{"git rebase -i", "git rebase --interactive"}, UseWhen: "performing an interactive rebase"},
    },
    Run: handleInteractiveRebaseExecute,
})
```

Handler implementation:

```go
var validActions = map[string]bool{
	"pick": true, "reword": true, "squash": true, "fixup": true, "drop": true,
}

func handleInteractiveRebaseExecute(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		RepoPath  string          `json:"repo_path"`
		Upstream  string          `json:"upstream"`
		Todo      []git.TodoEntry `json:"todo"`
		Autostash bool            `json:"autostash"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	// Determine current branch
	branchOut, err := git.Run(ctx, params.RepoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("failed to determine current branch: %v", err)), nil
	}
	branch := strings.TrimSpace(branchOut)

	// Safety: block on main/master
	if branch == "main" || branch == "master" {
		return command.TextErrorResult("interactive rebase on main/master is blocked for safety"), nil
	}

	// Check for existing rebase state
	state, err := git.DetectInProgressState(ctx, params.RepoPath)
	if err == nil && state != nil && state.Operation == "rebase" {
		return command.TextErrorResult("a rebase operation is already in progress; use rebase tool with continue, abort, or skip"), nil
	}

	// Validate todo entries
	if len(params.Todo) == 0 {
		return command.TextErrorResult("todo list must not be empty"), nil
	}

	// Collect reword messages keyed by short hash for the editor script
	rewordMessages := make(map[int]string) // index -> message

	for i, entry := range params.Todo {
		if !validActions[entry.Action] {
			return command.TextErrorResult(fmt.Sprintf("invalid action %q at index %d; must be one of: pick, reword, squash, fixup, drop", entry.Action, i)), nil
		}

		if entry.Hash == "" {
			return command.TextErrorResult(fmt.Sprintf("missing hash at index %d", i)), nil
		}

		if (entry.Action == "squash" || entry.Action == "fixup") && i == 0 {
			return command.TextErrorResult(fmt.Sprintf("%s cannot be the first action", entry.Action)), nil
		}

		if entry.Action == "reword" {
			if entry.Message == "" {
				return command.TextErrorResult(fmt.Sprintf("reword at index %d requires a message", i)), nil
			}
			rewordMessages[i] = entry.Message
		}

		// Validate hash exists
		_, err := git.Run(ctx, params.RepoPath, "rev-parse", "--verify", entry.Hash)
		if err != nil {
			return command.TextErrorResult(fmt.Sprintf("invalid commit hash %q at index %d", entry.Hash, i)), nil
		}
	}

	// Build the todo file content
	var todoContent strings.Builder
	for _, entry := range params.Todo {
		// Resolve to short hash for the todo file
		shortOut, err := git.Run(ctx, params.RepoPath, "rev-parse", "--short", entry.Hash)
		if err != nil {
			return command.TextErrorResult(fmt.Sprintf("failed to resolve hash %q: %v", entry.Hash, err)), nil
		}
		shortHash := strings.TrimSpace(shortOut)
		fmt.Fprintf(&todoContent, "%s %s\n", entry.Action, shortHash)
	}

	// Write the sequence editor script
	seqScript, err := os.CreateTemp("", "grit-rebase-seq-*.sh")
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("failed to create temp script: %v", err)), nil
	}
	defer os.Remove(seqScript.Name())

	todoStr := todoContent.String()
	// The script receives the todo file path as $1 and overwrites it
	scriptContent := fmt.Sprintf("#!/bin/sh\ncat > \"$1\" << 'GRIT_EOF'\n%sGRIT_EOF\n", todoStr)
	if _, err := seqScript.WriteString(scriptContent); err != nil {
		return command.TextErrorResult(fmt.Sprintf("failed to write temp script: %v", err)), nil
	}
	seqScript.Close()
	os.Chmod(seqScript.Name(), 0o755)

	// Build env vars
	extraEnv := []string{
		fmt.Sprintf("GIT_SEQUENCE_EDITOR=%s", seqScript.Name()),
	}

	// Handle reword messages via GIT_EDITOR
	var editorScript *os.File
	if len(rewordMessages) > 0 {
		editorScript, err = os.CreateTemp("", "grit-rebase-editor-*.sh")
		if err != nil {
			return command.TextErrorResult(fmt.Sprintf("failed to create editor script: %v", err)), nil
		}
		defer os.Remove(editorScript.Name())

		// Build a script that writes the next reword message each time it's called.
		// Uses a counter file to track which reword we're on.
		counterFile := editorScript.Name() + ".counter"
		defer os.Remove(counterFile)

		var editorContent strings.Builder
		editorContent.WriteString("#!/bin/sh\n")
		editorContent.WriteString(fmt.Sprintf("COUNTER_FILE='%s'\n", counterFile))
		editorContent.WriteString("if [ ! -f \"$COUNTER_FILE\" ]; then echo 0 > \"$COUNTER_FILE\"; fi\n")
		editorContent.WriteString("COUNT=$(cat \"$COUNTER_FILE\")\n")
		editorContent.WriteString("NEXT=$((COUNT + 1))\n")
		editorContent.WriteString("echo $NEXT > \"$COUNTER_FILE\"\n")

		// Build case statement for each reword message
		rewordIndex := 0
		editorContent.WriteString("case $COUNT in\n")
		for i, entry := range params.Todo {
			if entry.Action == "reword" {
				msg := rewordMessages[i]
				// Escape single quotes in message
				escaped := strings.ReplaceAll(msg, "'", "'\\''")
				editorContent.WriteString(fmt.Sprintf("  %d) printf '%%s' '%s' > \"$1\" ;;\n", rewordIndex, escaped))
				rewordIndex++
			}
		}
		editorContent.WriteString("esac\n")

		if _, err := editorScript.WriteString(editorContent.String()); err != nil {
			return command.TextErrorResult(fmt.Sprintf("failed to write editor script: %v", err)), nil
		}
		editorScript.Close()
		os.Chmod(editorScript.Name(), 0o755)

		extraEnv = append(extraEnv, fmt.Sprintf("GIT_EDITOR=%s", editorScript.Name()))
	}

	// Build git args
	gitArgs := []string{"rebase", "-i"}
	if params.Autostash {
		gitArgs = append(gitArgs, "--autostash")
	}
	gitArgs = append(gitArgs, params.Upstream)

	// Execute the rebase
	out, err := git.RunWithEnv(ctx, params.RepoPath, extraEnv, gitArgs...)
	if err != nil {
		// Check for conflicts
		if strings.Contains(err.Error(), "CONFLICT") || strings.Contains(err.Error(), "could not apply") {
			conflicts := extractConflictFiles(ctx, params.RepoPath)
			return command.JSONResult(git.RebaseResult{
				Status:    "conflict",
				Branch:    branch,
				Upstream:  params.Upstream,
				Conflicts: conflicts,
			}), nil
		}
		return command.TextErrorResult(fmt.Sprintf("git rebase -i: %v", err)), nil
	}

	result := git.RebaseResult{
		Status:   "completed",
		Branch:   branch,
		Upstream: params.Upstream,
		Summary:  strings.TrimSpace(out),
	}

	if strings.Contains(out, "is up to date") {
		result.Status = "up_to_date"
		result.Summary = ""
	}

	return command.JSONResult(result), nil
}
```

Add the `os` import to the file's import block.

- [ ] **Step 3: Run Go tests to verify compilation**

Run: `just test-grit`
Expected: PASS

- [ ] **Step 4: Build grit and run BATS tests**

Run: `just test-grit-bats`
Expected: All tests pass, including new interactive rebase tests.

- [ ] **Step 5: Commit**

```bash
git add packages/grit/internal/tools/interactive_rebase.go \
       packages/grit/zz-tests_bats/interactive_rebase_mcp.bats
git commit -m "feat(grit): add interactive_rebase_execute tool"
```

---

## Chunk 4: Explicit Drop Action Test

### Task 5: Add test for explicit `drop` action

**Files:**
- Modify: `packages/grit/zz-tests_bats/interactive_rebase_mcp.bats`

- [ ] **Step 1: Add test for explicit drop action**

Append to `packages/grit/zz-tests_bats/interactive_rebase_mcp.bats`:

```bash
function execute_explicit_drop { # @test
  setup_multi_commit_scenario

  local hash1 hash2 hash3
  hash1=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '1p')
  hash2=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '2p')
  hash3=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '3p')

  # Explicitly drop the second commit
  run run_grit_mcp "interactive_rebase_execute" "$(printf '{"repo_path":"%s","upstream":"main","todo":[{"action":"pick","hash":"%s"},{"action":"drop","hash":"%s"},{"action":"pick","hash":"%s"}]}' "$TEST_REPO" "$hash1" "$hash2" "$hash3")"
  assert_success

  local status
  status=$(echo "$output" | jq -r '.status')
  assert_equal "$status" "completed"

  # second.txt should not exist
  assert [ ! -f "$TEST_REPO/second.txt" ]

  # Should have 2 commits
  local count
  count=$(git -C "$TEST_REPO" log --oneline main..HEAD | wc -l | tr -d ' ')
  assert_equal "$count" "2"
}

function execute_fixup_commits { # @test
  setup_multi_commit_scenario

  local hash1 hash2 hash3
  hash1=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '1p')
  hash2=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '2p')
  hash3=$(git -C "$TEST_REPO" log --reverse --format=%H main..HEAD | sed -n '3p')

  # Fixup second into first (like squash but discard second's message)
  run run_grit_mcp "interactive_rebase_execute" "$(printf '{"repo_path":"%s","upstream":"main","todo":[{"action":"pick","hash":"%s"},{"action":"fixup","hash":"%s"},{"action":"pick","hash":"%s"}]}' "$TEST_REPO" "$hash1" "$hash2" "$hash3")"
  assert_success

  local status
  status=$(echo "$output" | jq -r '.status')
  assert_equal "$status" "completed"

  # Should have 2 commits
  local count
  count=$(git -C "$TEST_REPO" log --oneline main..HEAD | wc -l | tr -d ' ')
  assert_equal "$count" "2"

  # First commit message should be the original (not combined)
  local first_subject
  first_subject=$(git -C "$TEST_REPO" log --reverse --format=%s main..HEAD | head -1)
  assert_equal "$first_subject" "feature: add first"
}
```

- [ ] **Step 2: Build and run BATS tests**

Run: `just test-grit-bats`
Expected: All tests pass.

- [ ] **Step 3: Commit**

```bash
git add packages/grit/zz-tests_bats/interactive_rebase_mcp.bats
git commit -m "test(grit): add drop and fixup tests for interactive rebase"
```

---

## Chunk 5: Verify Full Suite

### Task 6: Run full test suite and verify no regressions

- [ ] **Step 1: Run Go unit tests**

Run: `just test-grit`
Expected: All pass.

- [ ] **Step 2: Run all grit BATS tests**

Run: `just test-grit-bats`
Expected: All pass, including existing rebase tests and new interactive rebase tests.

- [ ] **Step 3: Build the full marketplace**

Run: `just build`
Expected: Build succeeds. The new tools appear in the grit plugin manifest.

- [ ] **Step 4: Verify new tools appear in plugin manifest**

Run: `jq '.tools[] | select(.name | startswith("interactive_rebase"))' result/share/purse-first/grit/plugin.json`
Expected: Both `interactive_rebase_plan` and `interactive_rebase_execute` appear with correct schemas.
