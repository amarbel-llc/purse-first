# MCP Library Migration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Consolidate all MCP packages onto the shared libraries (go-mcp, rust-mcp) so V1 protocol support becomes a one-line change per package.

**Architecture:** Two independent workstreams. Workstream 1 migrates get-hubbed from go-lib-mcp to go-mcp with command.App adoption. Workstream 2 migrates chix from manual JSON-RPC to rust-mcp's McpServerBuilder + Tool trait. Both stay on V0 protocol default.

**Tech Stack:** Go (go-mcp library, command.App), Rust (rust-mcp library, async-trait), Nix (build expressions)

---

## Workstream 1: get-hubbed Migration

### Task 1: Update go.mod to use go-mcp

**Files:**
- Modify: `packages/get-hubbed/go.mod`
- Modify: `packages/get-hubbed/go.work`

**Step 1: Update go.mod**

Replace the module dependency and remove the purse-first replace directive.
The new go.mod should be:

```
module github.com/friedenberg/get-hubbed

go 1.25.6

require (
	github.com/amarbel-llc/purse-first/libs/go-mcp v0.0.3
	github.com/amarbel-llc/purse-first v0.0.0-20260216133354-540c1e5ba995
)

replace github.com/amarbel-llc/purse-first => ./deps/purse-first
```

**Step 2: Update go.work**

The go.work already includes `../purse-first` and `./`. It also needs
`../../libs/go-mcp` if not already resolved via the workspace. Check that
the top-level `go.work` at the repo root already includes `./libs/go-mcp`
(it does), so local resolution should work.

**Step 3: Run `go mod tidy` to resolve**

Run: `cd packages/get-hubbed && go mod tidy`
Expected: go.mod and go.sum updated with go-mcp dependency resolved via workspace.

**Step 4: Verify it compiles (it won't yet, imports still reference go-lib-mcp)**

Run: `cd packages/get-hubbed && go build ./...`
Expected: Compile errors about `go-lib-mcp` imports — this is fine, we fix them next.

**Step 5: Commit**

```bash
git add packages/get-hubbed/go.mod packages/get-hubbed/go.sum
git commit -m "chore(get-hubbed): update go.mod to depend on go-mcp"
```

---

### Task 2: Convert registry.go to command.App

**Files:**
- Modify: `packages/get-hubbed/internal/tools/registry.go`

**Step 1: Rewrite registry.go**

```go
package tools

import "github.com/amarbel-llc/purse-first/libs/go-mcp/command"

func RegisterAll() *command.App {
	app := command.NewApp("get-hubbed", "GitHub MCP server wrapping the gh CLI")
	app.Version = "0.1.0"

	registerRepoCommands(app)
	registerIssueCommands(app)
	registerPRCommands(app)
	registerAPICommands(app)
	registerRunCommands(app)
	registerContentCommands(app)

	return app
}
```

Note: function names change from `registerXxxTools(r)` to
`registerXxxCommands(app)` to match grit's convention.

**Step 2: Commit**

```bash
git add packages/get-hubbed/internal/tools/registry.go
git commit -m "refactor(get-hubbed): convert registry to command.App"
```

---

### Task 3: Convert repo.go tools

**Files:**
- Modify: `packages/get-hubbed/internal/tools/repo.go`

**Step 1: Rewrite repo.go**

Convert from `r.Register(name, desc, schema, handler)` to
`app.AddCommand(&command.Command{...})`. Handler signature changes from
`func(ctx, args) (*protocol.ToolCallResult, error)` to
`func(ctx, args, Prompter) (*command.Result, error)`.

```go
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/friedenberg/get-hubbed/internal/gh"
)

func registerRepoCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "repo_view",
		Description: command.Description{Short: "View repository details"},
		Params: []command.Param{
			{Name: "repo", Type: command.String, Description: "Repository in OWNER/REPO format", Required: true},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"gh repo view"}, UseWhen: "viewing repository details"},
		},
		Run: handleRepoView,
	})

	app.AddCommand(&command.Command{
		Name:        "repo_list",
		Description: command.Description{Short: "List repositories for an owner"},
		Params: []command.Param{
			{Name: "owner", Type: command.String, Description: "GitHub user or organization", Required: true},
			{Name: "limit", Type: command.Int, Description: "Maximum number of repositories to list (default 30)"},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"gh repo list"}, UseWhen: "listing repositories"},
		},
		Run: handleRepoList,
	})
}

func handleRepoView(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		Repo string `json:"repo"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	out, err := gh.Run(ctx,
		"repo", "view", params.Repo,
		"--json", "name,owner,description,url,defaultBranchRef,stargazerCount,forkCount,isPrivate,createdAt,updatedAt",
	)
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("gh repo view: %v", err)), nil
	}

	return command.TextResult(out), nil
}

func handleRepoList(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		Owner string `json:"owner"`
		Limit int    `json:"limit"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	ghArgs := []string{
		"repo", "list", params.Owner,
		"--json", "name,owner,description,url,isPrivate,stargazerCount,updatedAt",
	}

	if params.Limit > 0 {
		ghArgs = append(ghArgs, "--limit", fmt.Sprintf("%d", params.Limit))
	}

	out, err := gh.Run(ctx, ghArgs...)
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("gh repo list: %v", err)), nil
	}

	return command.TextResult(out), nil
}
```

Key changes:
- Import `command` instead of `protocol` and `server`
- `r *server.ToolRegistry` parameter becomes `app *command.App`
- Raw JSON schemas become typed `command.Param` slices
- Handler signature adds `_ command.Prompter` parameter
- Returns `*command.Result` via `command.TextResult()` / `command.TextErrorResult()`
  instead of `*protocol.ToolCallResult`
- Add `MapsTools` for `gh` CLI interception

**Step 2: Verify compilation**

Run: `cd packages/get-hubbed && go build ./...`
Expected: Errors from other unconverted files, but repo.go itself should be valid.

**Step 3: Commit**

```bash
git add packages/get-hubbed/internal/tools/repo.go
git commit -m "refactor(get-hubbed): convert repo tools to command.App"
```

---

### Task 4: Convert issue.go tools

**Files:**
- Modify: `packages/get-hubbed/internal/tools/issue.go`

**Step 1: Rewrite issue.go**

Same pattern as repo.go. Three tools: `issue_list`, `issue_view`,
`issue_create`. Convert raw JSON schemas to typed params.

Notable param types:
- `state` is a String with enum values — note that command.Param doesn't
  support enums, so the description should mention valid values. The handler
  already validates by passing through to `gh`.
- `labels` is an Array
- `number` is an Int

Handler changes:
- Signature: add `_ command.Prompter`
- Return: `command.TextResult(out)` / `command.TextErrorResult(...)`

Add `MapsTools` entries:
- `issue_list`: replaces `Bash` with prefix `gh issue list`
- `issue_view`: replaces `Bash` with prefix `gh issue view`
- `issue_create`: replaces `Bash` with prefix `gh issue create`

**Step 2: Commit**

```bash
git add packages/get-hubbed/internal/tools/issue.go
git commit -m "refactor(get-hubbed): convert issue tools to command.App"
```

---

### Task 5: Convert pr.go tools

**Files:**
- Modify: `packages/get-hubbed/internal/tools/pr.go`

**Step 1: Rewrite pr.go**

Two tools: `pr_list`, `pr_view`. Same pattern as issue tools.

Notable: `pr_list` has a `state` param with enum `open, closed, merged, all` —
include valid values in the description string.

Add `MapsTools` entries:
- `pr_list`: replaces `Bash` with prefix `gh pr list`
- `pr_view`: replaces `Bash` with prefix `gh pr view`

**Step 2: Commit**

```bash
git add packages/get-hubbed/internal/tools/pr.go
git commit -m "refactor(get-hubbed): convert pr tools to command.App"
```

---

### Task 6: Convert api.go tools

**Files:**
- Modify: `packages/get-hubbed/internal/tools/api.go`

**Step 1: Assess the object-type param limitation**

Three tools here use `object`-typed params that `command.Param` doesn't
support:
- `api_get`: `params` field is `map[string]string`
- `graphql_query`: `variables` field is `map[string]interface{}`
- `graphql_mutation`: `variables` field is `map[string]interface{}`

Two options:
1. Register these 3 tools directly on the ToolRegistry (bypassing command.App)
2. Encode `params`/`variables` as JSON strings in a String param

**Recommended: option 1** — register directly on ToolRegistry. This keeps the
handler logic unchanged and avoids forcing callers to double-serialize JSON.
These 3 tools are the escape hatch for advanced API access, so raw schemas
are appropriate.

**Step 2: Rewrite api.go**

```go
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/server"
	"github.com/friedenberg/get-hubbed/internal/gh"
)

// registerAPICommands registers API tools. These use object-typed params
// that command.Param doesn't support, so they register directly on a
// ToolRegistry passed via the app's extra registry.
func registerAPICommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "api_get",
		Description: command.Description{Short: "Make an authenticated GET request to the GitHub REST API"},
		Params: []command.Param{
			{Name: "endpoint", Type: command.String, Description: "REST API path, e.g. /repos/{owner}/{repo}/actions/runs", Required: true},
			{Name: "paginate", Type: command.Bool, Description: "Auto-paginate results"},
		},
		Run: handleAPIGet,
	})

	app.AddCommand(&command.Command{
		Name:        "graphql_query",
		Description: command.Description{Short: "Execute a read-only GraphQL query against the GitHub API"},
		Params: []command.Param{
			{Name: "query", Type: command.String, Description: "The GraphQL query string", Required: true},
			{Name: "paginate", Type: command.Bool, Description: "Auto-paginate results (requires endCursor/pageInfo in query)"},
		},
		Run: handleGraphQLQuery,
	})

	app.AddCommand(&command.Command{
		Name:        "graphql_mutation",
		Description: command.Description{Short: "Execute a GraphQL mutation against the GitHub API"},
		Params: []command.Param{
			{Name: "query", Type: command.String, Description: "The GraphQL mutation string", Required: true},
		},
		Run: handleGraphQLMutation,
	})
}

// RegisterAPIToolsRaw registers API tools that need object-typed params
// directly on the ToolRegistry with raw JSON schemas.
func RegisterAPIToolsRaw(r *server.ToolRegistry) {
	// Override the command.App-generated schemas with full schemas
	// that include the object-typed params.
	r.Register(
		"api_get",
		"Make an authenticated GET request to the GitHub REST API",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"endpoint": {
					"type": "string",
					"description": "REST API path, e.g. /repos/{owner}/{repo}/actions/runs"
				},
				"params": {
					"type": "object",
					"description": "Query string parameters as key-value pairs",
					"additionalProperties": {"type": "string"}
				},
				"headers": {
					"type": "array",
					"items": {"type": "string"},
					"description": "Additional headers in key:value format"
				},
				"paginate": {
					"type": "boolean",
					"description": "Auto-paginate results"
				}
			},
			"required": ["endpoint"]
		}`),
		func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
			r, err := handleAPIGet(ctx, args, command.StubPrompter{})
			if err != nil {
				return nil, err
			}
			return command.ResultToMCP(r), nil
		},
	)

	// ... same pattern for graphql_query and graphql_mutation
}
```

Wait — this approach creates a conflict: the tool gets registered twice
(once via command.App, once via raw registry). And it requires exposing
`ResultToMCP` from the command package.

**Revised approach:** Don't register api_get, graphql_query, graphql_mutation
via command.App at all. Keep them as direct ToolRegistry registrations with
the existing handler pattern. This means api.go keeps its current structure
but imports from go-mcp instead of go-lib-mcp.

```go
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/server"
	"github.com/friedenberg/get-hubbed/internal/gh"
)

func registerAPITools(r *server.ToolRegistry) {
	// Identical to current code, just with updated import paths.
	// ... (keep existing registrations unchanged)
}

// handlers unchanged
```

Then `registry.go` passes both the App and a ToolRegistry:

```go
package tools

import (
	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/server"
)

func RegisterAll() *command.App {
	app := command.NewApp("get-hubbed", "GitHub MCP server wrapping the gh CLI")
	app.Version = "0.1.0"

	registerRepoCommands(app)
	registerIssueCommands(app)
	registerPRCommands(app)
	registerRunCommands(app)
	registerContentCommands(app)

	return app
}

func RegisterAPITools(r *server.ToolRegistry) {
	registerAPITools(r)
}
```

And `main.go` calls both:

```go
registry := server.NewToolRegistry()
app.RegisterMCPTools(registry)
tools.RegisterAPITools(registry) // adds the 3 API tools with raw schemas
```

**Step 3: Rewrite api.go with updated imports only**

Keep the existing `registerAPITools(r *server.ToolRegistry)` pattern.
Just change imports from `go-lib-mcp` to `go-mcp`.

**Step 4: Commit**

```bash
git add packages/get-hubbed/internal/tools/api.go
git commit -m "refactor(get-hubbed): update api tools imports to go-mcp"
```

---

### Task 7: Convert run.go tools

**Files:**
- Modify: `packages/get-hubbed/internal/tools/run.go`

**Step 1: Rewrite run.go**

Three tools: `run_list`, `run_view`, `run_log`. Standard conversion.

Read the current file first to identify all params and their types. Convert
raw JSON schemas to `command.Param` declarations.

Add `MapsTools` entries:
- `run_list`: replaces `Bash` with prefix `gh run list`
- `run_view`: replaces `Bash` with prefix `gh run view`
- `run_log`: replaces `Bash` with prefix `gh run view --log`

**Step 2: Commit**

```bash
git add packages/get-hubbed/internal/tools/run.go
git commit -m "refactor(get-hubbed): convert run tools to command.App"
```

---

### Task 8: Convert content.go tools

**Files:**
- Modify: `packages/get-hubbed/internal/tools/content.go`

**Step 1: Rewrite content.go**

Six tools: `content_tree`, `content_read`, `content_blame`,
`content_commits`, `content_compare`, `content_search`.

This is the largest file (~680 lines). All tools use standard param types
(String, Int, Bool) so command.Param works for all of them.

Convert each tool's raw JSON schema to typed params. Handler signature
changes as in other files.

**Step 2: Commit**

```bash
git add packages/get-hubbed/internal/tools/content.go
git commit -m "refactor(get-hubbed): convert content tools to command.App"
```

---

### Task 9: Update main.go

**Files:**
- Modify: `packages/get-hubbed/cmd/get-hubbed/main.go`

**Step 1: Rewrite main.go**

Follow grit's pattern. Key changes:
- Import go-mcp instead of go-lib-mcp
- Call `tools.RegisterAll()` to get `*command.App`
- Use `app.RegisterMCPTools(registry)` to populate the ToolRegistry
- Call `tools.RegisterAPITools(registry)` for the 3 raw-schema API tools
- Use `app.GenerateAll()` for the generate-plugin subcommand

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/server"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/transport"
	"github.com/friedenberg/get-hubbed/internal/tools"
)

func main() {
	app := tools.RegisterAll()

	if len(os.Args) >= 3 && os.Args[1] == "generate-plugin" {
		if err := app.GenerateAll(os.Args[2]); err != nil {
			log.Fatalf("generating plugin: %v", err)
		}
		return
	}

	for _, arg := range os.Args[1:] {
		if arg == "-h" || arg == "--help" {
			fmt.Println("get-hubbed - a GitHub MCP server wrapping the gh CLI")
			fmt.Println()
			fmt.Println("Usage: get-hubbed")
			fmt.Println()
			fmt.Println("Runs an MCP server over stdio that exposes GitHub operations as tools.")
			os.Exit(0)
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	t := transport.NewStdio(os.Stdin, os.Stdout)

	registry := server.NewToolRegistry()
	app.RegisterMCPTools(registry)
	tools.RegisterAPITools(registry)

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

**Step 2: Verify full compilation**

Run: `cd packages/get-hubbed && go build ./cmd/get-hubbed/`
Expected: Clean build.

**Step 3: Commit**

```bash
git add packages/get-hubbed/cmd/get-hubbed/main.go
git commit -m "refactor(get-hubbed): update main.go for go-mcp + command.App"
```

---

### Task 10: Update go.mod, go.sum, and remove go-lib-mcp

**Files:**
- Modify: `packages/get-hubbed/go.mod`
- Modify: `packages/get-hubbed/go.sum`

**Step 1: Run go mod tidy**

Run: `cd packages/get-hubbed && go mod tidy`
Expected: go-lib-mcp dependency removed, go-mcp added.

**Step 2: Verify no go-lib-mcp references remain**

Run: `grep -r "go-lib-mcp" packages/get-hubbed/`
Expected: No matches.

**Step 3: Commit**

```bash
git add packages/get-hubbed/go.mod packages/get-hubbed/go.sum
git commit -m "chore(get-hubbed): remove go-lib-mcp dependency"
```

---

### Task 11: Update Nix build and verify

**Files:**
- Modify: `lib/packages/get-hubbed.nix`
- Regenerate: `packages/get-hubbed/gomod2nix.toml`

**Step 1: Regenerate gomod2nix.toml**

Run: `cd packages/get-hubbed && gomod2nix`

**Step 2: Update get-hubbed.nix**

The Nix build currently copies purse-first into `deps/purse-first` for the
`replace` directive. After migration, get-hubbed may still need this for the
purse-first/purse import, but now also needs `libs/go-mcp`. Check whether
go.work is stripped (it is, in the Nix build) and whether the go.mod replace
directives handle this correctly.

The current Nix build strips `go.work` and provides purse-first via `deps/`.
After migration, go.mod will reference `go-mcp` as a real module dependency
(not a replace), so it should resolve via the module proxy or via
gomod2nix.toml.

**Step 3: Build with Nix**

Run: `nix build .#get-hubbed --show-trace`
Expected: Clean build.

**Step 4: Smoke test**

Run:
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.1"}}}' | nix run .#get-hubbed
```
Expected: JSON response with `protocolVersion: "2024-11-05"` and tool capabilities.

**Step 5: Run any existing tests**

Run: `just test`
Expected: Existing tests pass.

**Step 6: Commit**

```bash
git add packages/get-hubbed/gomod2nix.toml lib/packages/get-hubbed.nix
git commit -m "chore(get-hubbed): update Nix build for go-mcp migration"
```

---

## Workstream 2: chix Migration

### Task 12: Create Tool trait implementations for a representative tool

**Files:**
- Create: `packages/chix/src/tools/build_tool.rs`
- Modify: `packages/chix/src/tools/mod.rs`

**Step 1: Create the first Tool trait impl**

Pick `build` as the representative tool. Create a new file that wraps the
existing `nix_build` function with the `Tool` trait:

```rust
use async_trait::async_trait;
use mcp_server::tools::{Tool, ToolError, ToolResult};
use mcp_server::server::Context;
use serde_json::Value;

use super::{NixBuildParams, list_tools};

pub struct BuildTool;

#[async_trait]
impl Tool for BuildTool {
    fn name(&self) -> &str {
        "build"
    }

    fn description(&self) -> &str {
        // Get from the existing ToolInfo in list_tools()
        "Build a nix flake package. Returns store paths on success. Agents MUST use this tool over running `nix build` directly - it provides validated inputs, structured output, and proper error handling."
    }

    fn input_schema(&self) -> Value {
        // Get from the existing list_tools() entry for "build"
        serde_json::json!({
            "type": "object",
            "properties": {
                "installable": {
                    "type": "string",
                    "description": "Flake installable (e.g., '.#default', 'nixpkgs#hello'). Defaults to '.#default'."
                },
                "print_build_logs": {
                    "type": "boolean",
                    "description": "Whether to print build logs (-L flag). Defaults to true."
                },
                "flake_dir": {
                    "type": "string",
                    "description": "Directory containing the flake. Defaults to current directory."
                },
                "max_log_bytes": {
                    "type": "integer",
                    "description": "Maximum bytes of build log output to return. Defaults to config value (100KB)."
                },
                "log_tail": {
                    "type": "integer",
                    "description": "Only return the last N lines of build log. Takes precedence over max_log_bytes."
                }
            }
        })
    }

    async fn execute(&self, arguments: Value, _ctx: &Context) -> Result<ToolResult, ToolError> {
        let params: NixBuildParams = serde_json::from_value(arguments)
            .map_err(|e| ToolError::InvalidArguments(e.to_string()))?;

        match super::build::nix_build(params).await {
            Ok(result) => {
                let json = serde_json::to_string_pretty(&result)
                    .map_err(|e| ToolError::ExecutionFailed(e.to_string()))?;
                Ok(ToolResult::text(json))
            }
            Err(e) => Ok(ToolResult::error(e)),
        }
    }
}
```

**Step 2: Verify it compiles in isolation**

Run: `cd packages/chix && cargo check`
Expected: May have import issues to resolve, but the pattern validates.

**Step 3: Commit**

```bash
git add packages/chix/src/tools/build_tool.rs
git commit -m "feat(chix): add Tool trait impl for build"
```

---

### Task 13: Create Tool trait implementations for all remaining tools

**Files:**
- Create: one `*_tool.rs` file per tool module (or group small tools into
  one file per module)

**Step 1: Create trait impls for each tool category**

Follow the same pattern as Task 12 for all 43 tools. Group by module:

- `flake_tools.rs`: flake_show, flake_check, flake_metadata, flake_update,
  flake_lock, flake_init (6 tools)
- `run_tools.rs`: run, develop_run (2 tools)
- `eval_tool.rs`: eval (1 tool)
- `log_tool.rs`: log (1 tool)
- `search_tool.rs`: search (1 tool)
- `store_tools.rs`: store_path_info, store_gc, store_ls, store_cat (4 tools)
- `derivation_tool.rs`: derivation_show (1 tool)
- `hash_tools.rs`: hash_path, hash_file (2 tools)
- `copy_tool.rs`: copy (1 tool)
- `flakehub_tools.rs`: fh_search, fh_add, fh_list_flakes, fh_list_releases,
  fh_list_versions, fh_resolve, fh_status, fh_fetch, fh_login (9 tools)
- `cachix_tools.rs`: cachix_push, cachix_use, cachix_status (3 tools)
- `task_tool.rs`: task_status (1 tool)
- `lsp_tools.rs`: nil_diagnostics, nil_completions, nil_hover,
  nil_definition (4 tools)

Each impl follows the exact same pattern:
1. Struct definition (e.g., `pub struct FlakeShowTool;`)
2. `#[async_trait] impl Tool for FlakeShowTool { ... }`
3. `name()` returns the tool name from existing `list_tools()`
4. `description()` returns the description from existing `list_tools()`
5. `input_schema()` returns the schema from existing `list_tools()`
6. `execute()` deserializes params, calls the existing async function,
   serializes the result

**Step 2: Update tools/mod.rs to export the new tool structs**

Add `mod build_tool;`, `mod flake_tools;`, etc. and `pub use` the structs.

**Step 3: Verify compilation**

Run: `cd packages/chix && cargo check`
Expected: Clean check. The old server.rs still exists but new code compiles.

**Step 4: Commit**

```bash
git add packages/chix/src/tools/
git commit -m "feat(chix): add Tool trait impls for all tools"
```

---

### Task 14: Create Resource trait implementations

**Files:**
- Modify: `packages/chix/src/resources/` (existing resource files)

**Step 1: Check existing resources**

Read `packages/chix/src/resources/` to understand what resources chix exposes.
Implement the `Resource` trait for each.

The `Resource` trait requires:
- `uri_template()` — URI pattern
- `name()` — human name
- `description()` — description
- `mime_type()` — MIME type
- `read(uri, ctx)` — returns `ResourceContent`

**Step 2: Create implementations and commit**

```bash
git add packages/chix/src/resources/
git commit -m "feat(chix): add Resource trait impls"
```

---

### Task 15: Replace main.rs and server.rs with McpServerBuilder

**Files:**
- Modify: `packages/chix/src/main.rs`
- Delete or gut: `packages/chix/src/server.rs`

**Step 1: Rewrite main.rs**

Replace the manual IO loop with McpServerBuilder:

```rust
use mcp_server::McpServer;
use clap::{Parser, Subcommand};

#[derive(Parser)]
#[command(name = "chix")]
struct Cli {
    #[command(subcommand)]
    command: Option<Commands>,
}

#[derive(Subcommand)]
enum Commands {
    InstallClaude,
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    let cli = Cli::parse();

    if let Some(Commands::InstallClaude) = cli.command {
        // existing install logic
        return Ok(());
    }

    let mut builder = McpServer::builder("chix", env!("CARGO_PKG_VERSION"));

    // Register all tools
    builder = builder
        .with_tool(tools::BuildTool)
        .with_tool(tools::FlakeShowTool)
        .with_tool(tools::FlakeCheckTool)
        // ... all 43 tools
        ;

    // Register all resources
    // builder = builder.with_resource(...);

    let server = builder.build();
    server.run_stdio().await?;

    Ok(())
}
```

**Step 2: Delete or empty server.rs**

The manual JSON-RPC types and dispatch logic are no longer needed. Delete
`server.rs` entirely or replace it with a `// Migrated to rust-mcp library`
comment.

**Step 3: Verify compilation**

Run: `cd packages/chix && cargo check`
Expected: Clean check.

**Step 4: Commit**

```bash
git add packages/chix/src/main.rs
git rm packages/chix/src/server.rs  # or git add if gutted
git commit -m "refactor(chix): replace manual JSON-RPC with rust-mcp McpServerBuilder"
```

---

### Task 16: Clean up unused code

**Files:**
- Modify: `packages/chix/src/tools/mod.rs`

**Step 1: Remove the old list_tools() function**

The `list_tools()` function and its `ToolInfo` struct are no longer needed —
each Tool trait impl provides its own name, description, and schema. Remove
the ~900-line function.

Keep the parameter structs (`NixBuildParams`, `NixFlakeShowParams`, etc.) as
they're still used by the Tool trait impls for deserialization.

**Step 2: Remove unused imports**

Run: `cd packages/chix && cargo check 2>&1 | grep "unused"`
Fix any unused import warnings.

**Step 3: Commit**

```bash
git add packages/chix/src/tools/mod.rs
git commit -m "refactor(chix): remove manual tool registry"
```

---

### Task 17: Verify Nix build and smoke test

**Files:**
- Possibly modify: `lib/packages/chix.nix`

**Step 1: Build with Nix**

Run: `nix build .#chix --show-trace`
Expected: Clean build. The Nix expression already vendors rust-mcp into the
chix source, so no Nix changes should be needed.

**Step 2: Smoke test**

Run:
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.1"}}}' | nix run .#chix
```
Expected: JSON response with `protocolVersion: "2024-11-05"` and tools +
resources capabilities.

**Step 3: Test tools/list**

Run:
```bash
printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.1"}}}\n{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}\n' | nix run .#chix
```
Expected: Second response lists all 43 tools with correct names and schemas.

**Step 4: Run full test suite**

Run: `just test`
Expected: All tests pass.

**Step 5: Commit any fixes**

```bash
git add -A
git commit -m "fix(chix): address issues found in smoke testing"
```

---

### Task 18: Final verification

**Step 1: Full Nix flake check**

Run: `nix flake check`
Expected: All checks pass for all packages.

**Step 2: Verify no go-lib-mcp references remain anywhere**

Run: `grep -r "go-lib-mcp" .`
Expected: No matches.

**Step 3: Verify chix server.rs is gone**

Run: `ls packages/chix/src/server.rs`
Expected: File not found.

**Step 4: Final commit if needed**

```bash
git commit -m "chore: complete MCP library migration"
```
