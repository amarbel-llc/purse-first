# Grit Hard Reset Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Add a `hard_reset` command to grit that runs `git reset --hard <ref>`, blocked on main/master.

**Architecture:** New command in its own file, registered alongside the existing staging commands. Follows the same safety pattern as force push (resolve current branch, block main/master). Uses existing `MutationResult` type.

**Tech Stack:** Go, go-mcp command framework

**Rollback:** N/A — purely additive.

---

### Task 1: Create hard_reset command

**Files:**
- Create: `packages/grit/internal/tools/hard_reset.go`

**Step 1: Write the command file**

```go
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"code.linenisgreat.com/purse-first/libs/go-mcp/command"
	"code.linenisgreat.com/purse-first/libs/go-mcp/protocol"
	"github.com/friedenberg/grit/internal/git"
)

func registerHardResetCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "hard_reset",
		Title:       "Hard Reset",
		Description: command.Description{Short: "Discard all changes and reset HEAD, index, and working tree to a ref (blocked on main/master for safety)"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(false),
			DestructiveHint: protocol.BoolPtr(true),
			IdempotentHint:  protocol.BoolPtr(true),
			OpenWorldHint:   protocol.BoolPtr(false),
		},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "ref", Type: command.String, Description: "Target ref (e.g. HEAD, origin/main, HEAD~3, a commit SHA)", Required: true},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git reset --hard"}, UseWhen: "discarding all changes and resetting to a ref"},
		},
		Run: handleGitHardReset,
	})
}

func handleGitHardReset(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		RepoPath string `json:"repo_path"`
		Ref      string `json:"ref"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	branchOut, err := git.Run(ctx, params.RepoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err == nil {
		branch := strings.TrimSpace(branchOut)
		if branch == "main" || branch == "master" {
			return command.TextErrorResult("hard reset on main/master is blocked for safety"), nil
		}
	}

	if _, err := git.Run(ctx, params.RepoPath, "reset", "--hard", params.Ref); err != nil {
		return command.TextErrorResult(fmt.Sprintf("git reset --hard: %v", err)), nil
	}

	return command.JSONResult(git.MutationResult{
		Status: "hard_reset",
		Ref:    params.Ref,
	}), nil
}
```

**Step 2: Verify it compiles**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/keen-aspen --command go build ./packages/grit/...`
Expected: Build succeeds (file exists but isn't registered yet, so the function is unused — this step just checks syntax). Actually, Go will complain about unused functions. Skip this step and verify in Task 2.

---

### Task 2: Register the command and update server instructions

**Files:**
- Modify: `packages/grit/internal/tools/registry.go:17` (add registration call)
- Modify: `packages/grit/cmd/grit/main.go:81` (update Instructions string)

**Step 1: Add registration call to registry.go**

Add `registerHardResetCommands(app)` after `registerRebaseCommands(app)` on line 17:

```go
func RegisterAll() *command.App {
	app := command.NewApp("grit", "MCP server exposing git operations")
	app.Version = "0.1.0"

	registerStatusCommands(app)
	registerLogCommands(app)
	registerStagingCommands(app)
	registerCommitCommands(app)
	registerBranchCommands(app)
	registerRemoteCommands(app)
	registerRevParseCommands(app)
	registerRebaseCommands(app)
	registerHardResetCommands(app)

	return app
}
```

**Step 2: Update server instructions string in main.go**

Change line 81 from:
```go
Instructions:  "Git MCP server exposing repository operations. Provides tools for status, diff, log, show, blame, staging, commits, branches, remotes, fetch, pull, push, and rebase. Force push is blocked on main/master.",
```
to:
```go
Instructions:  "Git MCP server exposing repository operations. Provides tools for status, diff, log, show, blame, staging, commits, branches, remotes, fetch, pull, push, rebase, and hard reset. Force push and hard reset are blocked on main/master.",
```

**Step 3: Verify build**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/keen-aspen --command go build ./packages/grit/...`
Expected: Build succeeds with no errors.

**Step 4: Run existing grit tests**

Run: `nix develop /Users/sfriedenberg/eng/repos/purse-first/.worktrees/keen-aspen --command go test ./packages/grit/...`
Expected: All existing tests pass.

**Step 5: Commit**

```
feat(grit): add hard_reset command with main/master protection
```
