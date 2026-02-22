# MCP V1 Upgrade Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable V1 (2025-11-25) MCP protocol negotiation across chix, get-hubbed, and grit with server instructions.

**Architecture:** Both shared libraries already have full V1 types, registries, and negotiation. The Go packages switch from `ToolRegistry` to `ToolRegistryV1` via a new `RegisterMCPToolsV1` method on `command.App`. The Rust package adds `.instructions()` to the builder. V0 clients continue to get V0 responses.

**Tech Stack:** Go (go-mcp, get-hubbed, grit), Rust (rust-mcp, chix), Nix (build validation)

---

### Task 1: Add RegisterMCPToolsV1 to command.App

**Files:**
- Modify: `libs/go-mcp/command/mcp.go`

**Step 1: Write the failing test**

Create `libs/go-mcp/command/mcp_test.go`:

```go
package command

import (
	"testing"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/server"
)

func TestRegisterMCPToolsV1(t *testing.T) {
	app := NewApp("test", "Test app")
	app.AddCommand(&Command{
		Name:        "echo",
		Description: Description{Short: "Echo a message"},
		Params: []Param{
			{Name: "message", Type: String, Required: true, Description: "The message"},
		},
		Run: func(ctx __context.Context, args __json.RawMessage, p Prompter) (*Result, error) {
			return &Result{Text: "ok"}, nil
		},
	})

	registry := server.NewToolRegistryV1()
	app.RegisterMCPToolsV1(registry)

	tools, err := registry.ListToolsV1(__context.Background(), "")
	if err != nil {
		t.Fatalf("ListToolsV1: %v", err)
	}
	if len(tools.Tools) != 1 {
		t.Fatalf("tool count = %d, want 1", len(tools.Tools))
	}
	tool := tools.Tools[0]
	if tool.Name != "echo" {
		t.Errorf("name = %q, want %q", tool.Name, "echo")
	}
	if tool.Description != "Echo a message" {
		t.Errorf("description = %q, want %q", tool.Description, "Echo a message")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd libs/go-mcp && go test ./command/ -run TestRegisterMCPToolsV1 -v`
Expected: FAIL — `RegisterMCPToolsV1` not defined

**Step 3: Add RegisterMCPToolsV1 and resultToMCPV1**

Append to `libs/go-mcp/command/mcp.go`:

```go
// RegisterMCPToolsV1 registers all non-hidden commands as V1 MCP tools
// in the given ToolRegistryV1.
func (a *App) RegisterMCPToolsV1(registry *server.ToolRegistryV1) {
	for name, cmd := range a.AllCommands() {
		if cmd.Hidden || cmd.Run == nil {
			continue
		}

		run := cmd.Run // capture for closure
		registry.Register(
			protocol.ToolV1{
				Name:        name,
				Description: cmd.Description.Short,
				InputSchema: cmd.InputSchema(),
			},
			func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResultV1, error) {
				result, err := run(ctx, args, StubPrompter{})
				if err != nil {
					return nil, err
				}
				return resultToMCPV1(result), nil
			},
		)
	}
}

func resultToMCPV1(r *Result) *protocol.ToolCallResultV1 {
	var text string
	if r.JSON != nil {
		data, _ := json.Marshal(r.JSON)
		text = string(data)
	} else {
		text = r.Text
	}
	return &protocol.ToolCallResultV1{
		Content: []protocol.ContentBlockV1{protocol.TextContentV1(text)},
		IsError: r.IsErr,
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd libs/go-mcp && go test ./command/ -run TestRegisterMCPToolsV1 -v`
Expected: PASS

**Step 5: Commit**

```
feat(go-mcp): add RegisterMCPToolsV1 to command.App
```

---

### Task 2: Add V1 negotiation test with ToolRegistryV1

**Files:**
- Modify: `libs/go-mcp/server/handler_test.go`

**Step 1: Write the test**

Add to `libs/go-mcp/server/handler_test.go`:

```go
func TestVersionNegotiationV1WithToolRegistryV1(t *testing.T) {
	registry := NewToolRegistryV1()
	registry.Register(protocol.ToolV1{
		Name:        "echo",
		Description: "Echo",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}, func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResultV1, error) {
		return &protocol.ToolCallResultV1{
			Content: []protocol.ContentBlockV1{protocol.TextContentV1("ok")},
		}, nil
	})

	s := &Server{
		opts: Options{
			ServerName:    "test",
			ServerVersion: "1.0",
			Instructions:  "Test server instructions",
			Tools:         registry,
		},
	}
	s.handler = NewHandler(s)

	initMsg := makeInitialize(t, protocol.ProtocolVersionV1)
	resp, err := s.handler.Handle(context.Background(), initMsg)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	var result protocol.InitializeResultV1
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal V1 result: %v", err)
	}

	if result.ProtocolVersion != protocol.ProtocolVersionV1 {
		t.Errorf("negotiated version = %q, want V1", result.ProtocolVersion)
	}
	if result.Instructions != "Test server instructions" {
		t.Errorf("instructions = %q, want %q", result.Instructions, "Test server instructions")
	}
}
```

**Step 2: Run test to verify it passes**

Run: `cd libs/go-mcp && go test ./server/ -run TestVersionNegotiationV1WithToolRegistryV1 -v`
Expected: PASS (negotiation logic and ToolRegistryV1 already exist)

**Step 3: Run all go-mcp tests**

Run: `cd libs/go-mcp && go test ./...`
Expected: All PASS

**Step 4: Commit**

```
test(go-mcp): add V1 negotiation test with ToolRegistryV1
```

---

### Task 3: Switch get-hubbed to ToolRegistryV1

**Files:**
- Modify: `packages/get-hubbed/cmd/get-hubbed/main.go`
- Modify: `packages/get-hubbed/internal/tools/registry.go`
- Modify: `packages/get-hubbed/internal/tools/api.go`

**Step 1: Update registry.go**

Change `RegisterAPITools` to accept `*server.ToolRegistryV1`:

```go
func RegisterAPITools(r *server.ToolRegistryV1) {
	registerAPITools(r)
}
```

**Step 2: Update api.go**

Change `registerAPITools` signature and all handler return types from V0 to V1:

```go
func registerAPITools(r *server.ToolRegistryV1) {
	r.Register(
		protocol.ToolV1{
			Name:        "api_get",
			Description: "Make an authenticated GET request to the GitHub REST API",
			InputSchema: json.RawMessage(`{...}`), // same schema as current
		},
		handleAPIGet,
	)
	// same for graphql_query and graphql_mutation
}
```

Update all three handler signatures:

```go
func handleAPIGet(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResultV1, error) {
```

Replace `protocol.ErrorResult(...)` with `protocol.ErrorResultV1(...)`.
Replace `protocol.TextContent(...)` with `protocol.TextContentV1(...)`.
Replace `protocol.ContentBlock` with `protocol.ContentBlockV1`.
Replace `*protocol.ToolCallResult` with `*protocol.ToolCallResultV1`.

**Step 3: Update main.go**

```go
registry := server.NewToolRegistryV1()
app.RegisterMCPToolsV1(registry)
tools.RegisterAPITools(registry)

srv, err := server.New(t, server.Options{
	ServerName:    app.Name,
	ServerVersion: app.Version,
	Instructions:  "GitHub MCP server wrapping the gh CLI. Provides tools for repositories, issues, pull requests, workflow runs, file content, and the GitHub API.",
	Tools:         registry,
})
```

**Step 4: Build and test**

Run: `cd packages/get-hubbed && go build ./cmd/get-hubbed`
Expected: Compiles without errors

Run: `cd packages/get-hubbed && go test ./...`
Expected: All PASS

**Step 5: Commit**

```
feat(get-hubbed): switch to ToolRegistryV1 for V1 negotiation
```

---

### Task 4: Switch grit to ToolRegistryV1

**Files:**
- Modify: `packages/grit/cmd/grit/main.go:68-75`

**Step 1: Update main.go**

```go
registry := server.NewToolRegistryV1()
app.RegisterMCPToolsV1(registry)

srv, err := server.New(t, server.Options{
	ServerName:    app.Name,
	ServerVersion: app.Version,
	Instructions:  "Git MCP server exposing repository operations. Provides tools for status, diff, log, show, blame, staging, commits, branches, remotes, fetch, pull, push, and rebase. Force push is blocked on main/master.",
	Tools:         registry,
})
```

**Step 2: Build and test**

Run: `cd packages/grit && go build ./cmd/grit`
Expected: Compiles without errors

Run: `cd packages/grit && go test ./...`
Expected: All PASS

**Step 3: Commit**

```
feat(grit): switch to ToolRegistryV1 for V1 negotiation
```

---

### Task 5: Add instructions to chix

**Files:**
- Modify: `packages/chix/src/main.rs:61`

**Step 1: Add .instructions() to builder**

```rust
let server = McpServerBuilder::new("chix", "0.1.0")
    .instructions("Nix MCP server providing tools for building, evaluating, and managing Nix flakes, packages, and store paths. Includes FlakeHub and Cachix integration, Nix language diagnostics via nil LSP, and background task management.")
    // Tools
    .with_tool(tools::BuildTool)
    // ... rest unchanged
```

**Step 2: Build and test**

Run: `cd packages/chix && cargo build`
Expected: Compiles without errors

Run: `cd packages/chix && cargo test`
Expected: All PASS

**Step 3: Commit**

```
feat(chix): add server instructions for V1 negotiation
```

---

### Task 6: Nix build validation

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

**Step 5: Final nix build**

Run: `nix build --show-trace`
Expected: Builds successfully

**Step 6: Commit any build artifacts**

```
chore: update go.work.sum and gomod2nix after V1 upgrade
```
