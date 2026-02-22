# Migrate grit to purse-first command framework — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace `go-lib-mcp` with `purse-first/libs/go-mcp` as the MCP framework and convert all 17 tools to declarative `command.Command` structs, giving grit a single source of truth for tool schemas, bash mappings, and handlers.

**Architecture:** The `command.App` type replaces both `server.ToolRegistry` (for MCP tool registration) and the `purse.PluginBuilder` (for plugin/mapping generation). Each tool becomes a `command.Command` with declarative `Params`, `MapsBash`, and `RunMCP` fields. `app.GenerateAll()` produces plugin.json, mappings.json, manpages, and shell completions at build time.

**Tech Stack:** Go, `github.com/amarbel-llc/purse-first/libs/go-mcp` (server, transport, protocol, command packages), Nix with gomod2nix

---

## Import Path Reference

All `go-lib-mcp` imports change to `purse-first/libs/go-mcp`:

| Old | New |
|-----|-----|
| `github.com/amarbel-llc/go-lib-mcp/server` | `github.com/amarbel-llc/purse-first/libs/go-mcp/server` |
| `github.com/amarbel-llc/go-lib-mcp/protocol` | `github.com/amarbel-llc/purse-first/libs/go-mcp/protocol` |
| `github.com/amarbel-llc/go-lib-mcp/transport` | `github.com/amarbel-llc/purse-first/libs/go-mcp/transport` |
| `github.com/amarbel-llc/go-lib-mcp/jsonrpc` | `github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc` |
| `github.com/amarbel-llc/purse-first/purse` | REMOVE (replaced by `command` package) |
| (new) | `github.com/amarbel-llc/purse-first/libs/go-mcp/command` |

---

### Task 1: Update go.mod and gomod2nix

**Files:**
- Modify: `go.mod`
- Modify: `gomod2nix.toml` (regenerated)

**Step 1: Update go.mod**

Replace go-lib-mcp with purse-first/libs/go-mcp. Remove the go-lib-mcp require line, add the libs/go-mcp require line, and keep purse-first for the purse package (it will be removed in a later task when main.go is updated):

```
module github.com/friedenberg/grit

go 1.25.6

require github.com/amarbel-llc/purse-first/libs/go-mcp v0.0.0-<latest>

require github.com/amarbel-llc/purse-first v0.0.0-<latest>
```

Run: `go get github.com/amarbel-llc/purse-first/libs/go-mcp@latest`
Then: `go mod tidy`

**Step 2: Regenerate gomod2nix.toml**

Run: `just deps`

**Step 3: Verify it compiles (it won't yet — that's expected)**

The code still references old import paths. This task just sets up the dependency.

**Step 4: Commit**

```bash
git add go.mod go.sum gomod2nix.toml
git commit -m "deps: add purse-first/libs/go-mcp dependency"
```

---

### Task 2: Migrate internal/tools/result.go

**Files:**
- Modify: `internal/tools/result.go`

**Step 1: Update import path**

Change:
```go
import (
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/go-lib-mcp/protocol"
)
```

To:
```go
import (
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
)
```

The `jsonResult` function body is unchanged — same types, new import path.

**Step 2: Commit**

```bash
git add internal/tools/result.go
git commit -m "refactor: migrate result.go to purse-first/libs/go-mcp"
```

---

### Task 3: Migrate internal/transport/sse.go

**Files:**
- Modify: `internal/transport/sse.go`

**Step 1: Update import path**

Change:
```go
"github.com/amarbel-llc/go-lib-mcp/jsonrpc"
```

To:
```go
"github.com/amarbel-llc/purse-first/libs/go-mcp/jsonrpc"
```

The SSE implementation is unchanged — same `jsonrpc.Message` type, new import path.

**Step 2: Commit**

```bash
git add internal/transport/sse.go
git commit -m "refactor: migrate SSE transport to purse-first/libs/go-mcp"
```

---

### Task 4: Convert internal/tools/registry.go to command.App

**Files:**
- Modify: `internal/tools/registry.go`

**Step 1: Rewrite registry.go**

Change from:
```go
package tools

import "github.com/amarbel-llc/go-lib-mcp/server"

func RegisterAll() *server.ToolRegistry {
	r := server.NewToolRegistry()
	registerStatusTools(r)
	registerLogTools(r)
	registerStagingTools(r)
	registerCommitTools(r)
	registerBranchTools(r)
	registerRemoteTools(r)
	registerRevParseTools(r)
	registerRebaseTools(r)
	return r
}
```

To:
```go
package tools

import "github.com/amarbel-llc/purse-first/libs/go-mcp/command"

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

	return app
}
```

This will not compile yet — the category functions still have old signatures. They get converted in tasks 5-12.

**Step 2: Commit**

```bash
git add internal/tools/registry.go
git commit -m "refactor: convert RegisterAll to return command.App"
```

---

### Task 5: Convert status.go to command.Command

**Files:**
- Modify: `internal/tools/status.go`

**Step 1: Rewrite registration function**

Change `registerStatusTools(r *server.ToolRegistry)` to `registerStatusCommands(app *command.App)`. Replace the two `r.Register()` calls with `app.AddCommand()` calls. Handler functions stay identical except for the import path change.

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

func registerStatusCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "status",
		Description: command.Description{Short: "Show working tree status with machine-readable output"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
		},
		MapsBash: []command.BashMapping{
			{Prefixes: []string{"git status"}, UseWhen: "checking repository status"},
		},
		RunMCP: handleGitStatus,
	})

	app.AddCommand(&command.Command{
		Name:        "diff",
		Description: command.Description{Short: "Show changes in the working tree or between commits"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "staged", Type: command.Bool, Description: "Show staged changes (--cached)"},
			{Name: "ref", Type: command.String, Description: "Diff against a specific ref (commit, branch, tag)"},
			{Name: "paths", Type: command.Array, Description: "Limit diff to specific paths"},
			{Name: "stat_only", Type: command.Bool, Description: "Show only diffstat summary"},
			{Name: "context_lines", Type: command.Int, Description: "Number of context lines around each change (git --unified=N, default 3)"},
			{Name: "max_patch_lines", Type: command.Int, Description: "Maximum number of patch output lines. Output is truncated with a truncated flag when exceeded."},
		},
		MapsBash: []command.BashMapping{
			{Prefixes: []string{"git diff"}, UseWhen: "viewing changes"},
		},
		RunMCP: handleGitDiff,
	})
}

// handleGitStatus and handleGitDiff stay identical — only the protocol import path changes
```

**Step 2: Commit**

```bash
git add internal/tools/status.go
git commit -m "refactor: convert status tools to command.Command"
```

---

### Task 6: Convert log.go to command.Command

**Files:**
- Modify: `internal/tools/log.go`

**Step 1: Rewrite registration function**

Convert `registerLogTools` to `registerLogCommands`. Three commands: `log`, `show`, `blame`.

```go
func registerLogCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "log",
		Description: command.Description{Short: "Show commit history as structured JSON"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "max_count", Type: command.Int, Description: "Maximum number of commits to show (default 10)"},
			{Name: "ref", Type: command.String, Description: "Starting ref (commit, branch, tag)"},
			{Name: "paths", Type: command.Array, Description: "Limit to commits affecting these paths"},
			{Name: "all", Type: command.Bool, Description: "Show commits from all branches"},
		},
		MapsBash: []command.BashMapping{
			{Prefixes: []string{"git log"}, UseWhen: "viewing commit history"},
		},
		RunMCP: handleGitLog,
	})

	app.AddCommand(&command.Command{
		Name:        "show",
		Description: command.Description{Short: "Show a commit, tag, or other git object"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "ref", Type: command.String, Description: "Ref to show (commit hash, tag, branch, etc.)", Required: true},
			{Name: "context_lines", Type: command.Int, Description: "Number of context lines around each change (git --unified=N, default 3)"},
			{Name: "max_patch_lines", Type: command.Int, Description: "Maximum number of patch output lines. Output is truncated with a truncated flag when exceeded."},
		},
		MapsBash: []command.BashMapping{
			{Prefixes: []string{"git show"}, UseWhen: "inspecting commits or objects"},
		},
		RunMCP: handleGitShow,
	})

	app.AddCommand(&command.Command{
		Name:        "blame",
		Description: command.Description{Short: "Show line-by-line authorship of a file"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "path", Type: command.String, Description: "File path to blame (relative to repo root)", Required: true},
			{Name: "ref", Type: command.String, Description: "Blame at a specific ref"},
			{Name: "line_range", Type: command.String, Description: "Line range in format START,END (e.g. '10,20')"},
		},
		MapsBash: []command.BashMapping{
			{Prefixes: []string{"git blame"}, UseWhen: "viewing line-by-line authorship"},
		},
		RunMCP: handleGitBlame,
	})
}
```

Import changes: replace `go-lib-mcp/{protocol,server}` with `purse-first/libs/go-mcp/{command,protocol}`.

**Step 2: Commit**

```bash
git add internal/tools/log.go
git commit -m "refactor: convert log tools to command.Command"
```

---

### Task 7: Convert staging.go to command.Command

**Files:**
- Modify: `internal/tools/staging.go`

**Step 1: Rewrite registration function**

```go
func registerStagingCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "add",
		Description: command.Description{Short: "Stage files for commit"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "paths", Type: command.Array, Description: "File paths to stage (relative to repo root)", Required: true},
		},
		MapsBash: []command.BashMapping{
			{Prefixes: []string{"git add"}, UseWhen: "staging files for commit"},
		},
		RunMCP: handleGitAdd,
	})

	app.AddCommand(&command.Command{
		Name:        "reset",
		Description: command.Description{Short: "Unstage files (soft reset only, does not modify working tree)"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "paths", Type: command.Array, Description: "File paths to unstage (relative to repo root)", Required: true},
		},
		MapsBash: []command.BashMapping{
			{Prefixes: []string{"git reset"}, UseWhen: "unstaging files"},
		},
		RunMCP: handleGitReset,
	})
}
```

**Step 2: Commit**

```bash
git add internal/tools/staging.go
git commit -m "refactor: convert staging tools to command.Command"
```

---

### Task 8: Convert commit.go to command.Command

**Files:**
- Modify: `internal/tools/commit.go`

**Step 1: Rewrite registration function**

```go
func registerCommitCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "commit",
		Description: command.Description{Short: "Create a new commit with staged changes"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "message", Type: command.String, Description: "Commit message", Required: true},
		},
		MapsBash: []command.BashMapping{
			{Prefixes: []string{"git commit"}, UseWhen: "creating a new commit"},
		},
		RunMCP: handleGitCommit,
	})
}
```

**Step 2: Commit**

```bash
git add internal/tools/commit.go
git commit -m "refactor: convert commit tool to command.Command"
```

---

### Task 9: Convert branch.go to command.Command

**Files:**
- Modify: `internal/tools/branch.go`

**Step 1: Rewrite registration function**

```go
func registerBranchCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "branch_list",
		Description: command.Description{Short: "List branches"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "remote", Type: command.Bool, Description: "List remote-tracking branches"},
			{Name: "all", Type: command.Bool, Description: "List both local and remote-tracking branches"},
		},
		MapsBash: []command.BashMapping{
			{Prefixes: []string{"git branch"}, UseWhen: "listing branches"},
		},
		RunMCP: handleGitBranchList,
	})

	app.AddCommand(&command.Command{
		Name:        "branch_create",
		Description: command.Description{Short: "Create a new branch"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "name", Type: command.String, Description: "Name for the new branch", Required: true},
			{Name: "start_point", Type: command.String, Description: "Starting point for the new branch (commit, branch, tag)"},
		},
		RunMCP: handleGitBranchCreate,
	})

	app.AddCommand(&command.Command{
		Name:        "checkout",
		Description: command.Description{Short: "Switch branches or restore working tree files"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "ref", Type: command.String, Description: "Branch name or ref to check out", Required: true},
			{Name: "create", Type: command.Bool, Description: "Create a new branch and check it out (-b)"},
		},
		MapsBash: []command.BashMapping{
			{Prefixes: []string{"git checkout", "git switch"}, UseWhen: "switching branches"},
		},
		RunMCP: handleGitCheckout,
	})
}
```

Note: `branch_create` has no `MapsBash` — the `git branch` prefix maps to `branch_list` (matching current behavior). The catch-all mapping at the end of main.go currently covers branch_create; that catch-all will be handled in Task 13.

**Step 2: Commit**

```bash
git add internal/tools/branch.go
git commit -m "refactor: convert branch tools to command.Command"
```

---

### Task 10: Convert remote.go to command.Command

**Files:**
- Modify: `internal/tools/remote.go`

**Step 1: Rewrite registration function**

```go
func registerRemoteCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "fetch",
		Description: command.Description{Short: "Fetch from a remote repository"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "remote", Type: command.String, Description: "Remote name (default origin)"},
			{Name: "prune", Type: command.Bool, Description: "Prune remote-tracking branches no longer on remote"},
			{Name: "all", Type: command.Bool, Description: "Fetch from all remotes"},
		},
		MapsBash: []command.BashMapping{
			{Prefixes: []string{"git fetch"}, UseWhen: "fetching from a remote"},
		},
		RunMCP: handleGitFetch,
	})

	app.AddCommand(&command.Command{
		Name:        "pull",
		Description: command.Description{Short: "Pull changes from a remote repository"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "remote", Type: command.String, Description: "Remote name (default origin)"},
			{Name: "branch", Type: command.String, Description: "Remote branch to pull"},
			{Name: "rebase", Type: command.Bool, Description: "Rebase instead of merge"},
		},
		MapsBash: []command.BashMapping{
			{Prefixes: []string{"git pull"}, UseWhen: "pulling changes from a remote"},
		},
		RunMCP: handleGitPull,
	})

	app.AddCommand(&command.Command{
		Name:        "push",
		Description: command.Description{Short: "Push commits to a remote repository (force push blocked on main/master)"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "remote", Type: command.String, Description: "Remote name (default origin)"},
			{Name: "branch", Type: command.String, Description: "Branch to push"},
			{Name: "set_upstream", Type: command.Bool, Description: "Set upstream tracking reference (-u)"},
			{Name: "force", Type: command.Bool, Description: "Force push (blocked on main/master branches)"},
		},
		MapsBash: []command.BashMapping{
			{Prefixes: []string{"git push"}, UseWhen: "pushing commits to a remote"},
		},
		RunMCP: handleGitPush,
	})

	app.AddCommand(&command.Command{
		Name:        "remote_list",
		Description: command.Description{Short: "List remotes with their URLs"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
		},
		MapsBash: []command.BashMapping{
			{Prefixes: []string{"git remote"}, UseWhen: "listing remotes"},
		},
		RunMCP: handleGitRemoteList,
	})
}
```

**Step 2: Commit**

```bash
git add internal/tools/remote.go
git commit -m "refactor: convert remote tools to command.Command"
```

---

### Task 11: Convert rev_parse.go to command.Command

**Files:**
- Modify: `internal/tools/rev_parse.go`

**Step 1: Rewrite registration function**

```go
func registerRevParseCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "git_rev_parse",
		Description: command.Description{Short: "Resolve a git revision to its full SHA, or resolve special names like HEAD, branch names, tags, and relative refs (e.g. HEAD~3, main^2)"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "ref", Type: command.String, Description: "Ref to resolve (e.g. HEAD, main, v1.0, HEAD~3, abc1234)", Required: true},
		},
		MapsBash: []command.BashMapping{
			{Prefixes: []string{"git rev-parse"}, UseWhen: "resolving a git revision to its full SHA"},
		},
		RunMCP: handleGitRevParse,
	})
}
```

**Step 2: Commit**

```bash
git add internal/tools/rev_parse.go
git commit -m "refactor: convert rev_parse tool to command.Command"
```

---

### Task 12: Convert rebase.go to command.Command

**Files:**
- Modify: `internal/tools/rebase.go`

**Step 1: Rewrite registration function**

```go
func registerRebaseCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "rebase",
		Description: command.Description{Short: "Rebase current branch onto another ref (blocked on main/master for safety)"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "upstream", Type: command.String, Description: "Ref to rebase onto (branch, tag, commit)"},
			{Name: "branch", Type: command.String, Description: "Branch to rebase (defaults to current branch)"},
			{Name: "autostash", Type: command.Bool, Description: "Automatically stash/unstash uncommitted changes"},
			{Name: "continue", Type: command.Bool, Description: "Continue rebase after resolving conflicts"},
			{Name: "abort", Type: command.Bool, Description: "Abort current rebase operation"},
			{Name: "skip", Type: command.Bool, Description: "Skip current commit and continue rebase"},
		},
		MapsBash: []command.BashMapping{
			{Prefixes: []string{"git rebase"}, UseWhen: "rebasing a branch"},
		},
		RunMCP: handleGitRebase,
	})
}
```

Note: rebase didn't have a specific bash mapping in the old code (only the catch-all `git ` prefix covered it). Adding a specific mapping is an improvement.

**Step 2: Commit**

```bash
git add internal/tools/rebase.go
git commit -m "refactor: convert rebase tool to command.Command"
```

---

### Task 13: Rewrite cmd/grit/main.go

**Files:**
- Modify: `cmd/grit/main.go`

**Step 1: Rewrite main.go**

Replace the entire file. Key changes:
- Import `purse-first/libs/go-mcp/{server,transport}` instead of `go-lib-mcp/{server,transport}`
- Remove `purse-first/purse` import
- Replace 100-line `generate-plugin` block with `app.GenerateAll(dir)`
- Use `app.RegisterMCPTools(registry)` to bridge commands to ToolRegistry

```go
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/server"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/transport"
	"github.com/friedenberg/grit/internal/tools"
	intTransport "github.com/friedenberg/grit/internal/transport"
)

func main() {
	sseMode := flag.Bool("sse", false, "Use HTTP/SSE transport instead of stdio")
	port := flag.Int("port", 8080, "Port for HTTP/SSE transport")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "grit — an MCP server exposing git operations\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  grit [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Starts an MCP server on stdio (default) or HTTP/SSE.\n")
		fmt.Fprintf(os.Stderr, "Intended to be launched by an MCP client such as Claude Code.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  grit                     # stdio transport\n")
		fmt.Fprintf(os.Stderr, "  grit --sse --port 8080   # HTTP/SSE transport\n")
	}

	flag.Parse()

	app := tools.RegisterAll()

	if flag.NArg() == 2 && flag.Arg(0) == "generate-plugin" {
		if err := app.GenerateAll(flag.Arg(1)); err != nil {
			log.Fatalf("generating plugin: %v", err)
		}
		return
	}

	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "grit: unexpected arguments: %v\n", flag.Args())
		flag.Usage()
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	var t transport.Transport

	if *sseMode {
		sse := intTransport.NewSSE(fmt.Sprintf(":%d", *port))
		if err := sse.Start(ctx); err != nil {
			log.Fatalf("starting SSE transport: %v", err)
		}
		defer sse.Close()
		log.Printf("SSE transport listening on %s", sse.Addr())
		t = sse
	} else {
		t = transport.NewStdio(os.Stdin, os.Stdout)
	}

	registry := server.NewToolRegistry()
	app.RegisterMCPTools(registry)

	srv, err := server.New(t, server.Options{
		ServerName:    app.Name,
		ServerVersion: app.Version,
		Tools:         registry,
	})
	if err != nil {
		log.Fatalf("creating server: %v", err)
	}

	if err := srv.Run(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
```

**Step 2: Commit**

```bash
git add cmd/grit/main.go
git commit -m "refactor: replace PluginBuilder with command.App in main"
```

---

### Task 14: Update go.mod, remove go-lib-mcp, regenerate gomod2nix

**Files:**
- Modify: `go.mod`
- Modify: `gomod2nix.toml`

**Step 1: Remove go-lib-mcp dependency**

Run: `go mod tidy`

This should drop `go-lib-mcp` since no code imports it anymore. Verify:

Run: `grep go-lib-mcp go.mod`
Expected: no output

**Step 2: Regenerate gomod2nix.toml**

Run: `just deps`

**Step 3: Verify go build compiles**

Run: `go build ./cmd/grit/`
Expected: successful build, binary produced

**Step 4: Run tests**

Run: `just test`
Expected: all tests pass

**Step 5: Commit**

```bash
git add go.mod go.sum gomod2nix.toml
git commit -m "deps: remove go-lib-mcp, use purse-first/libs/go-mcp exclusively"
```

---

### Task 15: Update flake.nix postInstall

**Files:**
- Modify: `flake.nix`

**Step 1: Update postInstall path**

Change:
```nix
postInstall = ''
  $out/bin/grit generate-plugin $out/share/purse-first
'';
```

To:
```nix
postInstall = ''
  $out/bin/grit generate-plugin $out
'';
```

`GenerateAll(dir)` writes to `{dir}/share/purse-first/{name}/...` internally, so passing `$out` is correct.

**Step 2: Nix build and verify artifacts**

Run: `just build`
Expected: successful build

Run: `ls ./result/share/purse-first/grit/`
Expected: `plugin.json` and `mappings.json`

Run: `ls ./result/share/man/man1/`
Expected: manpage files (`grit.1`, `grit-status.1`, etc.)

Run: `ls ./result/share/bash-completion/completions/`
Expected: `grit` completion file

**Step 3: Verify plugin.json content**

Run: `cat ./result/share/purse-first/grit/plugin.json`
Expected:
```json
{
  "name": "grit",
  "mcpServers": {
    "grit": {
      "type": "stdio",
      "command": "grit"
    }
  }
}
```

**Step 4: Verify mappings.json has all expected entries**

Run: `cat ./result/share/purse-first/grit/mappings.json | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d['mappings']), 'mappings')"`
Expected: mapping count matches the number of commands with MapsBash (should be ~16)

**Step 5: Commit**

```bash
git add flake.nix
git commit -m "build: update postInstall for GenerateAll output path"
```

---

### Task 16: Final verification

**Step 1: Clean build from scratch**

Run: `just clean && just build`
Expected: successful nix build

**Step 2: Run the binary to verify it starts**

Run: `echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | ./result/bin/grit`
Expected: JSON response with `"serverInfo":{"name":"grit","version":"0.1.0"}` and tools capability

**Step 3: Run tests**

Run: `just test`
Expected: all tests pass

**Step 4: Verify generate-plugin subcommand**

Run: `mkdir -p /tmp/grit-test && ./result/bin/grit generate-plugin /tmp/grit-test && ls /tmp/grit-test/share/purse-first/grit/`
Expected: `plugin.json`, `mappings.json`

Run: `ls /tmp/grit-test/share/man/man1/ | head -5`
Expected: manpage files

Run: `ls /tmp/grit-test/share/bash-completion/completions/`
Expected: `grit`

Run: `rm -rf /tmp/grit-test`
