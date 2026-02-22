# MCP V1 Tool Annotations Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add titles and behavior annotations (readOnly, destructive, idempotent, openWorld) to all 80 tools across chix, get-hubbed, and grit.

**Architecture:** Go packages get `Title` and `Annotations` fields on `command.Command`, passed through by `RegisterMCPToolsV1`. API tools in get-hubbed set annotations directly on `protocol.ToolV1`. Chix tools implement the `ToolV1` trait (extending `Tool`) with `title()` and `annotations()` methods, and registration switches from `.with_tool()` to `.with_tool_v1()`.

**Tech Stack:** Go (go-mcp, get-hubbed, grit), Rust (rust-mcp, chix), Nix (build validation)

---

### Task 1: Add Title and Annotations to command.Command

**Files:**
- Modify: `libs/go-mcp/protocol/tools_v1.go`
- Modify: `libs/go-mcp/command/command.go:72-90`
- Modify: `libs/go-mcp/command/mcp.go:38-59`
- Modify: `libs/go-mcp/command/mcp_test.go`

**Step 1: Add BoolPtr helper to protocol package**

Append to `libs/go-mcp/protocol/tools_v1.go`:

```go
// BoolPtr returns a pointer to b, for use with ToolAnnotations hint fields.
func BoolPtr(b bool) *bool { return &b }
```

**Step 2: Add Title and Annotations fields to Command**

In `libs/go-mcp/command/command.go`, add two fields to the `Command` struct after `Hidden bool`:

```go
type Command struct {
	Name        string
	Aliases     []string
	Description Description
	Hidden      bool

	// Title is a human-readable display name for the MCP tool (V1).
	Title string

	// Annotations provides V1 behavior hints (readOnly, destructive, etc.).
	Annotations *protocol.ToolAnnotations

	Params    []Param
	MapsTools []ToolMapping
	Examples  []Example
	// ...Run, RunCLI unchanged
}
```

Add `protocol` import: `"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"`

**Step 3: Update RegisterMCPToolsV1 to pass through Title and Annotations**

In `libs/go-mcp/command/mcp.go:45-50`, change the `protocol.ToolV1` literal:

```go
registry.Register(
	protocol.ToolV1{
		Name:        name,
		Title:       cmd.Title,
		Description: cmd.Description.Short,
		InputSchema: cmd.InputSchema(),
		Annotations: cmd.Annotations,
	},
	// handler unchanged
)
```

**Step 4: Write the test**

Add to `libs/go-mcp/command/mcp_test.go`:

```go
func TestRegisterMCPToolsV1Annotations(t *testing.T) {
	app := NewApp("test", "test")

	readOnly := true
	destructive := false
	idempotent := true
	openWorld := false

	app.AddCommand(&Command{
		Name:        "status",
		Title:       "Show Working Tree Status",
		Description: Description{Short: "Show status"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    &readOnly,
			DestructiveHint: &destructive,
			IdempotentHint:  &idempotent,
			OpenWorldHint:   &openWorld,
		},
		Params: []Param{
			{Name: "repo_path", Type: String, Required: true},
		},
		Run: func(ctx context.Context, args json.RawMessage, p Prompter) (*Result, error) {
			return TextResult("ok"), nil
		},
	})

	registry := server.NewToolRegistryV1()
	app.RegisterMCPToolsV1(registry)

	result, err := registry.ListToolsV1(context.Background(), "")
	if err != nil {
		t.Fatalf("ListToolsV1: %v", err)
	}

	tool := result.Tools[0]

	if tool.Title != "Show Working Tree Status" {
		t.Errorf("title = %q, want %q", tool.Title, "Show Working Tree Status")
	}

	if tool.Annotations == nil {
		t.Fatal("annotations is nil")
	}

	if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint {
		t.Error("readOnlyHint should be true")
	}

	if tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint {
		t.Error("destructiveHint should be false")
	}
}
```

**Step 5: Run tests**

Run: `cd libs/go-mcp && go test ./command/ -run TestRegisterMCPToolsV1Annotations -v`
Expected: PASS

Run: `cd libs/go-mcp && go test ./...`
Expected: All PASS

**Step 6: Commit**

```
feat(go-mcp): add Title and Annotations to command.Command for V1
```

---

### Task 2: Add annotations to all grit tools

**Files:**
- Modify: `packages/grit/internal/tools/status.go`
- Modify: `packages/grit/internal/tools/log.go`
- Modify: `packages/grit/internal/tools/staging.go`
- Modify: `packages/grit/internal/tools/commit.go`
- Modify: `packages/grit/internal/tools/branch.go`
- Modify: `packages/grit/internal/tools/remote.go`
- Modify: `packages/grit/internal/tools/rev_parse.go`
- Modify: `packages/grit/internal/tools/rebase.go`

**Annotation values for all 17 grit tools:**

| Tool | Title | readOnly | destructive | idempotent | openWorld |
|------|-------|----------|-------------|------------|-----------|
| status | Show Working Tree Status | true | false | true | false |
| diff | Show Changes | true | false | true | false |
| log | Show Commit History | true | false | true | false |
| show | Show Git Object | true | false | true | false |
| blame | Show Line Authorship | true | false | true | false |
| add | Stage Files | false | false | true | false |
| reset | Unstage Files | false | false | true | false |
| commit | Create Commit | false | false | false | false |
| branch_list | List Branches | true | false | true | false |
| branch_create | Create Branch | false | false | false | false |
| checkout | Switch Branches | false | false | true | false |
| fetch | Fetch from Remote | false | false | true | true |
| pull | Pull from Remote | false | false | false | true |
| push | Push to Remote | false | true | false | true |
| remote_list | List Remotes | true | false | true | false |
| git_rev_parse | Resolve Git Revision | true | false | true | false |
| rebase | Rebase Branch | false | true | false | false |

**Step 1: Add import to each file**

Every file that registers commands needs:
```go
import "github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
```

**Step 2: Add Title and Annotations to each command**

Pattern (showing `status.go` as example):

```go
app.AddCommand(&command.Command{
	Name:        "status",
	Title:       "Show Working Tree Status",
	Description: command.Description{Short: "Show working tree status with machine-readable output"},
	Annotations: &protocol.ToolAnnotations{
		ReadOnlyHint:    protocol.BoolPtr(true),
		DestructiveHint: protocol.BoolPtr(false),
		IdempotentHint:  protocol.BoolPtr(true),
		OpenWorldHint:   protocol.BoolPtr(false),
	},
	// Params, MapsTools, Run unchanged
})
```

Apply the pattern to all 17 tools using the table above. Each `app.AddCommand(...)` call gets `Title:` and `Annotations:` fields added right after the existing `Description:` field.

**Step 3: Build and test**

Run: `cd packages/grit && go build ./cmd/grit`
Expected: Compiles without errors

Run: `cd packages/grit && go test ./...`
Expected: All PASS

**Step 4: Commit**

```
feat(grit): add V1 tool annotations and titles
```

---

### Task 3: Add annotations to all get-hubbed tools

**Files:**
- Modify: `packages/get-hubbed/internal/tools/repo.go`
- Modify: `packages/get-hubbed/internal/tools/issue.go`
- Modify: `packages/get-hubbed/internal/tools/pr.go`
- Modify: `packages/get-hubbed/internal/tools/run.go`
- Modify: `packages/get-hubbed/internal/tools/content.go`
- Modify: `packages/get-hubbed/internal/tools/api.go`

**Annotation values for all 20 get-hubbed tools:**

**Command-based tools (17):**

| Tool | Title | readOnly | destructive | idempotent | openWorld |
|------|-------|----------|-------------|------------|-----------|
| repo_view | View Repository | true | false | true | true |
| repo_list | List Repositories | true | false | true | true |
| issue_list | List Issues | true | false | true | true |
| issue_view | View Issue | true | false | true | true |
| issue_create | Create Issue | false | false | false | true |
| pr_list | List Pull Requests | true | false | true | true |
| pr_view | View Pull Request | true | false | true | true |
| run_list | List Workflow Runs | true | false | true | true |
| run_view | View Workflow Run | true | false | true | true |
| run_log | View Run Logs | true | false | true | true |
| content_tree | List Directory Contents | true | false | true | true |
| content_read | Read File Contents | true | false | true | true |
| content_blame | Show File Blame | true | false | true | true |
| content_commits | List File Commits | true | false | true | true |
| content_compare | Compare Refs | true | false | true | true |
| content_search | Search Code | true | false | true | true |

**API tools (3) — set directly on `protocol.ToolV1`:**

| Tool | Title | readOnly | destructive | idempotent | openWorld |
|------|-------|----------|-------------|------------|-----------|
| api_get | GitHub API GET | true | false | true | true |
| graphql_query | GitHub GraphQL Query | true | false | true | true |
| graphql_mutation | GitHub GraphQL Mutation | false | true | false | true |

**Step 1: Add annotations to command-based tools**

Same pattern as grit. Add `protocol` import and set `Title:` + `Annotations:` on each `command.Command`.

**Step 2: Add annotations to API tools**

In `packages/get-hubbed/internal/tools/api.go`, add `Title` and `Annotations` to each `protocol.ToolV1` struct:

```go
r.Register(
	protocol.ToolV1{
		Name:        "api_get",
		Title:       "GitHub API GET",
		Description: "Make an authenticated GET request to the GitHub REST API",
		InputSchema: json.RawMessage(`{...}`), // unchanged
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(true),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(true),
			OpenWorldHint:   protocol.BoolPtr(true),
		},
	},
	handleAPIGet,
)
```

Same pattern for `graphql_query` (readOnly) and `graphql_mutation` (destructive).

**Step 3: Build and test**

Run: `cd packages/get-hubbed && go build ./cmd/get-hubbed`
Expected: Compiles without errors

Run: `cd packages/get-hubbed && go test ./...`
Expected: All PASS

**Step 4: Commit**

```
feat(get-hubbed): add V1 tool annotations and titles
```

---

### Task 4: Add annotations to all chix tools

**Files:**
- Modify: `packages/chix/src/tools/build_tool.rs`
- Modify: `packages/chix/src/tools/flake_tools.rs`
- Modify: `packages/chix/src/tools/run_tools.rs`
- Modify: `packages/chix/src/tools/log_tool.rs`
- Modify: `packages/chix/src/tools/search_tool.rs`
- Modify: `packages/chix/src/tools/store_tools.rs`
- Modify: `packages/chix/src/tools/derivation_tool.rs`
- Modify: `packages/chix/src/tools/hash_tools.rs`
- Modify: `packages/chix/src/tools/copy_tool.rs`
- Modify: `packages/chix/src/tools/eval_tool.rs`
- Modify: `packages/chix/src/tools/flakehub_tools.rs`
- Modify: `packages/chix/src/tools/cachix_tools.rs`
- Modify: `packages/chix/src/tools/lsp_tools.rs`
- Modify: `packages/chix/src/tools/task_tool.rs`
- Modify: `packages/chix/src/main.rs`

**Annotation values for all 37 chix tools:**

| Tool | Title | readOnly | destructive | idempotent | openWorld |
|------|-------|----------|-------------|------------|-----------|
| build | Build Nix Package | false | false | true | true |
| flake_show | Show Flake Outputs | true | false | true | false |
| flake_check | Check Flake | true | false | true | true |
| flake_metadata | Show Flake Metadata | true | false | true | false |
| flake_update | Update Flake Lock | false | true | false | true |
| flake_lock | Lock Flake Inputs | false | true | false | true |
| flake_init | Initialize Flake | false | true | false | false |
| run | Run Flake App | false | true | false | true |
| develop_run | Run in Dev Shell | false | true | false | true |
| log | Show Build Log | true | false | true | false |
| search | Search Packages | true | false | true | true |
| store_path_info | Show Store Path Info | true | false | true | false |
| store_gc | Garbage Collect Store | false | true | true | false |
| store_ls | List Store Path | true | false | true | false |
| store_cat | Read Store File | true | false | true | false |
| derivation_show | Show Derivation | true | false | true | false |
| hash_path | Hash Path | true | false | true | false |
| hash_file | Hash File | true | false | true | false |
| copy | Copy Store Paths | false | false | true | true |
| eval | Evaluate Nix Expression | true | false | true | false |
| fh_search | Search FlakeHub | true | false | true | true |
| fh_add | Add FlakeHub Input | false | true | false | true |
| fh_list_flakes | List FlakeHub Flakes | true | false | true | true |
| fh_list_releases | List Flake Releases | true | false | true | true |
| fh_list_versions | List Flake Versions | true | false | true | true |
| fh_resolve | Resolve FlakeHub Ref | true | false | true | true |
| cachix_push | Push to Cachix | false | false | true | true |
| cachix_use | Configure Cachix | false | true | true | false |
| cachix_status | Check Cachix Status | true | false | true | false |
| fh_status | Check FlakeHub Status | true | false | true | true |
| fh_fetch | Fetch from FlakeHub | false | false | true | true |
| fh_login | Login to FlakeHub | false | false | true | true |
| task_status | Check Task Status | true | false | true | false |
| nil_diagnostics | Nix Diagnostics | true | false | true | false |
| nil_completions | Nix Completions | true | false | true | false |
| nil_hover | Nix Hover Info | true | false | true | false |
| nil_definition | Nix Go to Definition | true | false | true | false |

**Step 1: Add ToolV1 impl to each tool**

Pattern (showing `build_tool.rs` as example). Add import and impl block after the existing `impl Tool for BuildTool`:

```rust
use mcp_server::tools::{ToolAnnotations, ToolV1};

#[async_trait]
impl ToolV1 for BuildTool {
    fn title(&self) -> Option<&str> {
        Some("Build Nix Package")
    }

    fn annotations(&self) -> Option<ToolAnnotations> {
        Some(ToolAnnotations {
            title: None,
            read_only_hint: Some(false),
            destructive_hint: Some(false),
            idempotent_hint: Some(true),
            open_world_hint: Some(true),
        })
    }
}
```

Apply this pattern to all 37 tools using the table above. Each tool file gets:
1. `use mcp_server::tools::{ToolAnnotations, ToolV1};` (add to existing imports)
2. An `impl ToolV1 for XxxTool { ... }` block after each `impl Tool for XxxTool`

**Step 2: Switch main.rs from `.with_tool()` to `.with_tool_v1()`**

In `packages/chix/src/main.rs`, change every `.with_tool(tools::XxxTool)` to `.with_tool_v1(tools::XxxTool)` for all 37 tools. The 3 resource registrations (`.with_resource()`) stay unchanged.

**Step 3: Build and test**

Run: `cd packages/chix && cargo build`
Expected: Compiles without errors

Run: `cd packages/chix && cargo test`
Expected: All PASS

**Step 4: Commit**

```
feat(chix): add V1 tool annotations and titles
```

---

### Task 5: Nix build validation

**Files:** None modified — build validation only.

**Step 1: Build all packages with Nix**

Run: `nix build --show-trace` (from repo root)
Expected: Builds successfully

**Step 2: Run BATS integration tests**

Run: `just test` (from repo root)
Expected: All tests pass

**Step 3: Update go.work.sum if needed**

Run: `cd /home/sasha/eng/repos/purse-first && go work sync`

**Step 4: Regenerate gomod2nix.toml files if go.mod changed**

Run: `env GOWORK=off gomod2nix --dir packages/get-hubbed --outdir packages/get-hubbed`
Run: `env GOWORK=off gomod2nix --dir packages/grit --outdir packages/grit`

**Important:** If the go-mcp dependency version changed, you must first push the go-mcp commits to origin, then run `GOWORK=off go get github.com/amarbel-llc/purse-first/libs/go-mcp@<commit-sha>` in each package directory before regenerating gomod2nix.toml.

**Step 5: Final nix build**

Run: `nix build --show-trace`
Expected: Builds successfully

**Step 6: Commit any build artifacts**

```
chore: update go.work.sum and gomod2nix after V1 annotations
```
