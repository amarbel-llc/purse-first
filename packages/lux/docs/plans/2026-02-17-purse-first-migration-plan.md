# Purse-First Migration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Migrate lux from go-lib-mcp + custom MCP infrastructure to purse-first's go-mcp library and command framework.

**Architecture:** Replace `go-lib-mcp` dependency with `purse-first/libs/go-mcp`. Define all 10 LSP tools as `command.Command` structs. Use purse-first's `server.Server` and registries instead of custom Handler/ToolRegistry/ResourceRegistry/PromptRegistry. Keep Cobra for CLI dispatch. Add `generate-plugin` for build-time artifact generation.

**Tech Stack:** Go 1.22+, purse-first/libs/go-mcp (command, server, protocol, transport), Cobra, Nix flakes

---

### Task 1: Swap go-lib-mcp dependency to purse-first go-mcp

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

**Step 1: Update go.mod**

Replace the `go-lib-mcp` dependency with `purse-first/libs/go-mcp`:

```
# Remove old dependency
go mod edit -droprequire github.com/amarbel-llc/go-lib-mcp

# Add new dependency (use the same version grit uses)
go mod edit -require github.com/amarbel-llc/purse-first/libs/go-mcp@v0.0.0-20260217222858-cd512e5ef8b7
```

**Step 2: Update all import paths**

Replace all occurrences of `github.com/amarbel-llc/go-lib-mcp` with `github.com/amarbel-llc/purse-first/libs/go-mcp` across the codebase. Files to update:

- `internal/mcp/tools.go`: `go-lib-mcp/protocol` → `purse-first/libs/go-mcp/protocol`
- `internal/mcp/bridge.go`: `go-lib-mcp/protocol` → `purse-first/libs/go-mcp/protocol`
- `internal/mcp/server.go`: `go-lib-mcp/jsonrpc`, `go-lib-mcp/transport` → `purse-first/libs/go-mcp/jsonrpc`, `purse-first/libs/go-mcp/transport`
- `internal/mcp/handler.go`: `go-lib-mcp/jsonrpc`, `go-lib-mcp/protocol` → `purse-first/libs/go-mcp/jsonrpc`, `purse-first/libs/go-mcp/protocol`
- `internal/mcp/resources.go`: `go-lib-mcp/protocol` → `purse-first/libs/go-mcp/protocol`
- `internal/mcp/prompts.go`: `go-lib-mcp/protocol` → `purse-first/libs/go-mcp/protocol`
- `cmd/lux/main.go`: `go-lib-mcp/transport` → `purse-first/libs/go-mcp/transport`
- Any other files importing from `go-lib-mcp`

**Step 3: Tidy and verify compilation**

```bash
cd /Users/sfriedenberg/eng/worktrees/lux/purse
go mod tidy
go build ./...
```

Expected: Build succeeds. The APIs are compatible.

**Step 4: Commit**

```bash
git add go.mod go.sum $(grep -rl 'go-lib-mcp' --include='*.go' .)
git commit -m "chore: migrate from go-lib-mcp to purse-first/libs/go-mcp"
```

---

### Task 2: Create internal/tools package with Bridge

The Bridge needs to move to `internal/tools/` so it can be used by the command handlers without creating a circular dependency with `internal/mcp/`.

**Files:**
- Create: `internal/tools/bridge.go`
- Modify: `internal/mcp/bridge.go` (will be deleted after server.go is updated)

**Step 1: Copy bridge.go to internal/tools/**

```bash
mkdir -p internal/tools
cp internal/mcp/bridge.go internal/tools/bridge.go
```

**Step 2: Update the package declaration and imports**

Change `package mcp` to `package tools` in `internal/tools/bridge.go`. Update the import paths — the Bridge depends on:
- `github.com/amarbel-llc/purse-first/libs/go-mcp/protocol`
- `github.com/amarbel-llc/lux/internal/lsp`
- `github.com/amarbel-llc/lux/internal/server`
- `github.com/amarbel-llc/lux/internal/subprocess`

These imports stay the same, only the package declaration changes.

**Step 3: Verify compilation**

```bash
go build ./internal/tools/
```

Expected: Build succeeds.

**Step 4: Commit**

```bash
git add internal/tools/bridge.go
git commit -m "refactor: move Bridge to internal/tools package"
```

---

### Task 3: Create command.App with all 10 tool definitions

**Files:**
- Create: `internal/tools/registry.go`
- Create: `internal/tools/result.go`

**Step 1: Create result.go helper**

Create `internal/tools/result.go`:

```go
package tools

import (
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
)

func jsonResult(v any) (*protocol.ToolCallResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}

	return &protocol.ToolCallResult{
		Content: []protocol.ContentBlock{
			protocol.TextContent(string(data)),
		},
	}, nil
}
```

**Step 2: Create registry.go with all 10 tool definitions**

Create `internal/tools/registry.go`. Each tool maps from the current inline JSON schema + handler in `internal/mcp/tools.go` to a `command.Command` with typed `Param` declarations. The `RunMCP` closures capture the bridge and call its methods.

```go
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	"github.com/amarbel-llc/lux/internal/lsp"
)

func RegisterAll(bridge *Bridge) *command.App {
	app := command.NewApp("lux", "MCP server exposing LSP capabilities as tools")
	app.Version = "0.1.0"
	app.MCPArgs = []string{"mcp", "stdio"}

	registerPositionTools(app, bridge)
	registerDocumentTools(app, bridge)
	registerRefactoringTools(app, bridge)

	return app
}

func registerPositionTools(app *command.App, bridge *Bridge) {
	app.AddCommand(&command.Command{
		Name: "hover",
		Description: command.Description{
			Short: "Get type information, documentation, and signatures for a symbol. Agents MUST use this tool instead of reading source files when you need to understand what a function/type does, its parameters, return types, or documentation. Unlike grep/read which show raw text, hover provides semantically-parsed information from the language server. DO NOT read files just to check function signatures or types - use this tool instead.",
		},
		Params: []command.Param{
			{Name: "uri", Type: command.String, Required: true, Description: "File URI (e.g., file:///path/to/file.go)"},
			{Name: "line", Type: command.Int, Required: true, Description: "0-indexed line number"},
			{Name: "character", Type: command.Int, Required: true, Description: "0-indexed character offset"},
		},
		RunMCP: makePositionHandler(bridge, func(b *Bridge, ctx context.Context, uri lsp.DocumentURI, line, char int) (*protocol.ToolCallResult, error) {
			return b.Hover(ctx, uri, line, char)
		}),
	})

	app.AddCommand(&command.Command{
		Name: "definition",
		Description: command.Description{
			Short: "Jump to the definition of any symbol (function, type, variable). Agents MUST use this tool instead of grep/search when you know a symbol name and need to find its definition or implementation. Uses semantic analysis to find the actual definition, not just string matches. DO NOT use grep or file searches to locate function/type definitions - this tool handles cross-file navigation, interface implementations, and import sources accurately.",
		},
		Params: []command.Param{
			{Name: "uri", Type: command.String, Required: true, Description: "File URI (e.g., file:///path/to/file.go)"},
			{Name: "line", Type: command.Int, Required: true, Description: "0-indexed line number"},
			{Name: "character", Type: command.Int, Required: true, Description: "0-indexed character offset"},
		},
		RunMCP: makePositionHandler(bridge, func(b *Bridge, ctx context.Context, uri lsp.DocumentURI, line, char int) (*protocol.ToolCallResult, error) {
			return b.Definition(ctx, uri, line, char)
		}),
	})

	app.AddCommand(&command.Command{
		Name: "references",
		Description: command.Description{
			Short: "Find ALL usages of a symbol throughout the codebase. Agents MUST use this tool instead of grep/search for finding where functions/types/variables are used - it understands scope and semantics, finding actual references not just string matches. DO NOT use grep to find usages of symbols - grep finds false positives (comments, strings, similar names). Critical for impact analysis before refactoring, understanding how functions are called, tracing data flow.",
		},
		Params: []command.Param{
			{Name: "uri", Type: command.String, Required: true, Description: "File URI (e.g., file:///path/to/file.go)"},
			{Name: "line", Type: command.Int, Required: true, Description: "0-indexed line number"},
			{Name: "character", Type: command.Int, Required: true, Description: "0-indexed character offset"},
			{Name: "include_declaration", Type: command.Bool, Description: "Include the declaration in results", Default: true},
		},
		RunMCP: handleReferences(bridge),
	})

	app.AddCommand(&command.Command{
		Name: "completion",
		Description: command.Description{
			Short: "Get context-aware code completions at a cursor position. Agents should use this tool instead of reading documentation or source files when exploring available methods on a type, discovering struct fields, finding imported symbols, or understanding API surfaces. Shows only valid symbols, methods, and fields actually available in scope - more accurate than guessing from source.",
		},
		Params: []command.Param{
			{Name: "uri", Type: command.String, Required: true, Description: "File URI (e.g., file:///path/to/file.go)"},
			{Name: "line", Type: command.Int, Required: true, Description: "0-indexed line number"},
			{Name: "character", Type: command.Int, Required: true, Description: "0-indexed character offset"},
		},
		RunMCP: makePositionHandler(bridge, func(b *Bridge, ctx context.Context, uri lsp.DocumentURI, line, char int) (*protocol.ToolCallResult, error) {
			return b.Completion(ctx, uri, line, char)
		}),
	})
}

func registerDocumentTools(app *command.App, bridge *Bridge) {
	app.AddCommand(&command.Command{
		Name: "format",
		Description: command.Description{
			Short: "Get formatting edits for a document according to language-standard style. Uses external formatters (configured in formatters.toml) when available, falling back to LSP formatting. Agents should use this tool to get proper formatting instead of manually adjusting whitespace or running external formatters. Returns text edits needed to properly format the file. Note: returns edits but does not apply them - use Edit tool to apply the returned changes.",
		},
		Params: []command.Param{
			{Name: "uri", Type: command.String, Required: true, Description: "File URI (e.g., file:///path/to/file.go)"},
		},
		RunMCP: handleURI(bridge, func(b *Bridge, ctx context.Context, uri lsp.DocumentURI) (*protocol.ToolCallResult, error) {
			return b.Format(ctx, uri)
		}),
	})

	app.AddCommand(&command.Command{
		Name: "document_symbols",
		Description: command.Description{
			Short: "Get a structured outline of all symbols in a file. Agents MUST use this tool instead of reading entire files when you need to understand file structure or find what functions/types exist in a file. Returns hierarchical symbols: function/method names, type definitions, nested structures, top-level constants. DO NOT read and parse files manually to find symbol names - this tool is faster and more accurate.",
		},
		Params: []command.Param{
			{Name: "uri", Type: command.String, Required: true, Description: "File URI (e.g., file:///path/to/file.go)"},
		},
		RunMCP: handleURI(bridge, func(b *Bridge, ctx context.Context, uri lsp.DocumentURI) (*protocol.ToolCallResult, error) {
			return b.DocumentSymbols(ctx, uri)
		}),
	})

	app.AddCommand(&command.Command{
		Name: "diagnostics",
		Description: command.Description{
			Short: "Get compiler/linter diagnostics (errors, warnings, hints) for a file. Agents should use this tool instead of running build commands when checking for errors in a specific file. Provides precise error locations and messages. Use to understand issues before making edits or to verify changes are correct without running a full build.",
		},
		Params: []command.Param{
			{Name: "uri", Type: command.String, Required: true, Description: "File URI (e.g., file:///path/to/file.go)"},
		},
		RunMCP: handleURI(bridge, func(b *Bridge, ctx context.Context, uri lsp.DocumentURI) (*protocol.ToolCallResult, error) {
			return b.Diagnostics(ctx, uri)
		}),
	})

	app.AddCommand(&command.Command{
		Name: "workspace_symbols",
		Description: command.Description{
			Short: "Search for symbols (functions, types, constants) across the entire workspace by name pattern. Agents MUST use this tool instead of grep/glob when searching for symbol definitions by name. DO NOT use grep to find function or type definitions - grep returns all text matches including usages, comments, and strings. This tool returns only actual symbol definitions with their locations.",
		},
		Params: []command.Param{
			{Name: "query", Type: command.String, Required: true, Description: "Symbol name pattern to search for"},
			{Name: "uri", Type: command.String, Required: true, Description: "Any file URI in the workspace (used to identify which LSP to query)"},
		},
		RunMCP: handleWorkspaceSymbols(bridge),
	})
}

func registerRefactoringTools(app *command.App, bridge *Bridge) {
	app.AddCommand(&command.Command{
		Name: "code_action",
		Description: command.Description{
			Short: "Get suggested fixes, refactorings, and improvements for code at a range. Agents should use this tool to get language-server suggested fixes instead of manually writing fixes for common issues. Provides quick fixes for errors, refactoring operations (extract function, inline variable), import organization, and code generation (implement interface). Use after diagnostics to get fixes for reported issues.",
		},
		Params: []command.Param{
			{Name: "uri", Type: command.String, Required: true, Description: "File URI (e.g., file:///path/to/file.go)"},
			{Name: "start_line", Type: command.Int, Required: true, Description: "0-indexed start line"},
			{Name: "start_character", Type: command.Int, Required: true, Description: "0-indexed start character"},
			{Name: "end_line", Type: command.Int, Required: true, Description: "0-indexed end line"},
			{Name: "end_character", Type: command.Int, Required: true, Description: "0-indexed end character"},
		},
		RunMCP: handleCodeAction(bridge),
	})

	app.AddCommand(&command.Command{
		Name: "rename",
		Description: command.Description{
			Short: "Rename a symbol across the entire codebase with semantic accuracy. Agents MUST use this tool instead of find-and-replace or manual editing when renaming functions, types, variables, or other symbols. Only renames actual references (not comments, strings, or similar names), handles scoping correctly, and updates imports appropriately. DO NOT use grep+edit or find-and-replace for renaming - it will miss references or change unrelated text.",
		},
		Params: []command.Param{
			{Name: "uri", Type: command.String, Required: true, Description: "File URI (e.g., file:///path/to/file.go)"},
			{Name: "line", Type: command.Int, Required: true, Description: "0-indexed line number"},
			{Name: "character", Type: command.Int, Required: true, Description: "0-indexed character offset"},
			{Name: "new_name", Type: command.String, Required: true, Description: "New name for the symbol"},
		},
		RunMCP: handleRename(bridge),
	})
}

// Handler factories to reduce boilerplate

func makePositionHandler(bridge *Bridge, fn func(*Bridge, context.Context, lsp.DocumentURI, int, int) (*protocol.ToolCallResult, error)) func(context.Context, json.RawMessage) (*protocol.ToolCallResult, error) {
	return func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
		var a struct {
			URI       string `json:"uri"`
			Line      int    `json:"line"`
			Character int    `json:"character"`
		}
		if err := json.Unmarshal(args, &a); err != nil {
			return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
		}
		return fn(bridge, ctx, lsp.DocumentURI(a.URI), a.Line, a.Character)
	}
}

func handleURI(bridge *Bridge, fn func(*Bridge, context.Context, lsp.DocumentURI) (*protocol.ToolCallResult, error)) func(context.Context, json.RawMessage) (*protocol.ToolCallResult, error) {
	return func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
		var a struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(args, &a); err != nil {
			return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
		}
		return fn(bridge, ctx, lsp.DocumentURI(a.URI))
	}
}

func handleReferences(bridge *Bridge) func(context.Context, json.RawMessage) (*protocol.ToolCallResult, error) {
	return func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
		var a struct {
			URI                string `json:"uri"`
			Line               int    `json:"line"`
			Character          int    `json:"character"`
			IncludeDeclaration bool   `json:"include_declaration"`
		}
		a.IncludeDeclaration = true
		if err := json.Unmarshal(args, &a); err != nil {
			return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
		}
		return bridge.References(ctx, lsp.DocumentURI(a.URI), a.Line, a.Character, a.IncludeDeclaration)
	}
}

func handleWorkspaceSymbols(bridge *Bridge) func(context.Context, json.RawMessage) (*protocol.ToolCallResult, error) {
	return func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
		var a struct {
			Query string `json:"query"`
			URI   string `json:"uri"`
		}
		if err := json.Unmarshal(args, &a); err != nil {
			return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
		}
		return bridge.WorkspaceSymbols(ctx, lsp.DocumentURI(a.URI), a.Query)
	}
}

func handleCodeAction(bridge *Bridge) func(context.Context, json.RawMessage) (*protocol.ToolCallResult, error) {
	return func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
		var a struct {
			URI            string `json:"uri"`
			StartLine      int    `json:"start_line"`
			StartCharacter int    `json:"start_character"`
			EndLine        int    `json:"end_line"`
			EndCharacter   int    `json:"end_character"`
		}
		if err := json.Unmarshal(args, &a); err != nil {
			return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
		}
		return bridge.CodeAction(ctx, lsp.DocumentURI(a.URI),
			a.StartLine, a.StartCharacter, a.EndLine, a.EndCharacter)
	}
}

func handleRename(bridge *Bridge) func(context.Context, json.RawMessage) (*protocol.ToolCallResult, error) {
	return func(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
		var a struct {
			URI       string `json:"uri"`
			Line      int    `json:"line"`
			Character int    `json:"character"`
			NewName   string `json:"new_name"`
		}
		if err := json.Unmarshal(args, &a); err != nil {
			return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
		}
		return bridge.Rename(ctx, lsp.DocumentURI(a.URI), a.Line, a.Character, a.NewName)
	}
}
```

**Step 3: Verify compilation**

```bash
go build ./internal/tools/
```

Expected: Build succeeds.

**Step 4: Commit**

```bash
git add internal/tools/registry.go internal/tools/result.go
git commit -m "feat: define LSP tools as purse-first command.Command structs"
```

---

### Task 4: Rewrite MCP server to use purse-first server.Server

**Files:**
- Modify: `internal/mcp/server.go`
- Modify: `internal/mcp/resources.go`
- Modify: `internal/mcp/prompts.go`
- Delete: `internal/mcp/handler.go`
- Delete: `internal/mcp/tools.go`

**Step 1: Rewrite server.go**

Replace `internal/mcp/server.go` with a thin wrapper around purse-first's server:

```go
package mcp

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	mcpserver "github.com/amarbel-llc/purse-first/libs/go-mcp/server"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/transport"
	"github.com/amarbel-llc/lux/internal/config"
	luxserver "github.com/amarbel-llc/lux/internal/server"
	"github.com/amarbel-llc/lux/internal/subprocess"
	"github.com/amarbel-llc/lux/internal/tools"
)

type Server struct {
	inner *mcpserver.Server
	pool  *subprocess.Pool
}

func New(cfg *config.Config, t transport.Transport) (*Server, error) {
	router, err := luxserver.NewRouter(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating router: %w", err)
	}

	executor := subprocess.NewNixExecutor()
	pool := subprocess.NewPool(executor, lspNotificationHandler())

	for _, l := range cfg.LSPs {
		var capOverrides *subprocess.CapabilityOverride
		if l.Capabilities != nil {
			capOverrides = &subprocess.CapabilityOverride{
				Disable: l.Capabilities.Disable,
				Enable:  l.Capabilities.Enable,
			}
		}
		pool.Register(l.Name, l.Flake, l.Binary, l.Args, l.Env, l.InitOptions, capOverrides)
	}

	bridge := tools.NewBridge(pool, router)

	app := tools.RegisterAll(bridge)

	toolRegistry := mcpserver.NewToolRegistry()
	app.RegisterMCPTools(toolRegistry)

	resourceRegistry := mcpserver.NewResourceRegistry()
	registerResources(resourceRegistry, pool, bridge, cfg)

	promptRegistry := mcpserver.NewPromptRegistry()
	registerPrompts(promptRegistry)

	srv, err := mcpserver.New(t, mcpserver.Options{
		ServerName:    "lux",
		ServerVersion: app.Version,
		Tools:         toolRegistry,
		Resources:     resourceRegistry,
		Prompts:       promptRegistry,
	})
	if err != nil {
		return nil, err
	}

	return &Server{inner: srv, pool: pool}, nil
}

func (s *Server) Run(ctx context.Context) error {
	defer s.pool.StopAll()
	return s.inner.Run(ctx)
}

func (s *Server) Close() {
	s.inner.Close()
}

func lspNotificationHandler() func(ctx context.Context, msg interface{}) {
	// Placeholder: LSP notifications are currently ignored.
	// The actual type depends on the jsonrpc.Handler type from the pool.
	return nil
}
```

Note: The `lspNotificationHandler` return type needs to match what `subprocess.NewPool` expects. Check the actual signature of `subprocess.NewPool` and adjust accordingly.

**Step 2: Rewrite resources.go**

Replace `internal/mcp/resources.go` to use purse-first's `ResourceRegistry`. The key difference: purse-first's `ResourceRegistry.ReadResource` does exact URI matching, but lux's symbols template uses prefix matching (`lux://symbols/{uri}`). Handle this by implementing `ResourceProvider` directly for a wrapper that handles both static resources and the template.

```go
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	mcpserver "github.com/amarbel-llc/purse-first/libs/go-mcp/server"
	"github.com/amarbel-llc/lux/internal/config"
	"github.com/amarbel-llc/lux/internal/lsp"
	"github.com/amarbel-llc/lux/internal/subprocess"
	"github.com/amarbel-llc/lux/internal/tools"
	"github.com/amarbel-llc/lux/pkg/filematch"
)

func registerResources(registry *mcpserver.ResourceRegistry, pool *subprocess.Pool, bridge *tools.Bridge, cfg *config.Config) {
	cwd, _ := os.Getwd()
	matcher := filematch.NewMatcherSet()
	for _, l := range cfg.LSPs {
		matcher.Add(l.Name, l.Extensions, l.Patterns, l.LanguageIDs)
	}

	registry.RegisterResource(
		protocol.Resource{
			URI:         "lux://status",
			Name:        "LSP Status",
			Description: "Current status of configured language servers including which are running",
			MimeType:    "application/json",
		},
		readStatusFunc(pool, cfg),
	)

	registry.RegisterResource(
		protocol.Resource{
			URI:         "lux://languages",
			Name:        "Supported Languages",
			Description: "Languages supported by lux with their file extensions and patterns",
			MimeType:    "application/json",
		},
		readLanguagesFunc(cfg),
	)

	registry.RegisterResource(
		protocol.Resource{
			URI:         "lux://files",
			Name:        "Project Files",
			Description: "Files in the current directory that match configured LSP extensions/patterns",
			MimeType:    "application/json",
		},
		readFilesFunc(cwd, matcher),
	)

	registry.RegisterTemplate(
		protocol.ResourceTemplate{
			URITemplate: "lux://symbols/{uri}",
			Name:        "File Symbols",
			Description: "All symbols (functions, types, constants, etc.) in a file as reported by the LSP",
			MimeType:    "application/json",
		},
		readSymbolsFunc(bridge),
	)
}

// Resource reader implementations — same logic as current resources.go,
// just wrapped as mcpserver.ResourceReader functions.

// (The existing read functions are refactored into closures that return
// mcpserver.ResourceReader compatible functions. See current resources.go
// for the JSON structures: statusResponse, lspStatus, languagesResponse,
// languageInfo, filesResponse, filesStats, symbolsResponse.)
```

The actual reader function implementations stay the same — they just need to be
wrapped as `func(ctx context.Context, uri string) (*protocol.ResourceReadResult, error)`.
The existing struct types (`statusResponse`, `lspStatus`, etc.) and the logic inside
`readStatus()`, `readLanguages()`, `readFiles()`, `readSymbols()` are preserved.

**Important:** The purse-first `ResourceRegistry.ReadResource` does exact URI lookup
and won't match template URIs like `lux://symbols/file:///path/to/file.go`.
You need to implement a custom `ResourceProvider` wrapper that delegates static URIs
to the registry but handles the symbols prefix match separately:

```go
type resourceProvider struct {
	registry *mcpserver.ResourceRegistry
	bridge   *tools.Bridge
}

func (p *resourceProvider) ListResources(ctx context.Context) ([]protocol.Resource, error) {
	return p.registry.ListResources(ctx)
}

func (p *resourceProvider) ReadResource(ctx context.Context, uri string) (*protocol.ResourceReadResult, error) {
	if strings.HasPrefix(uri, "lux://symbols/") {
		fileURI := strings.TrimPrefix(uri, "lux://symbols/")
		return readSymbols(ctx, p.bridge, uri, fileURI)
	}
	return p.registry.ReadResource(ctx, uri)
}

func (p *resourceProvider) ListResourceTemplates(ctx context.Context) ([]protocol.ResourceTemplate, error) {
	return p.registry.ListResourceTemplates(ctx)
}
```

Then pass this wrapper as `Resources` in the server options instead of the registry directly.

**Step 3: Rewrite prompts.go**

Replace `internal/mcp/prompts.go` to use purse-first's `PromptRegistry`:

```go
package mcp

import (
	"context"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	mcpserver "github.com/amarbel-llc/purse-first/libs/go-mcp/server"
)

const codeExplorationPrompt = `When exploring an unfamiliar codebase, use lux LSP tools strategically:
... (same content as current prompts.go)
`

const refactoringGuidePrompt = `For safe, comprehensive refactoring using lux:
... (same content as current prompts.go)
`

func registerPrompts(registry *mcpserver.PromptRegistry) {
	registry.Register(
		protocol.Prompt{
			Name:        "code-exploration",
			Description: "Best practices for exploring and understanding code using LSP tools",
		},
		func(ctx context.Context, args map[string]string) (*protocol.PromptGetResult, error) {
			return &protocol.PromptGetResult{
				Description: "Best practices for exploring and understanding code using LSP tools",
				Messages: []protocol.PromptMessage{
					{
						Role:    "user",
						Content: protocol.TextContent(codeExplorationPrompt),
					},
				},
			}, nil
		},
	)

	registry.Register(
		protocol.Prompt{
			Name:        "refactoring-guide",
			Description: "How to safely refactor code using LSP-assisted tools",
		},
		func(ctx context.Context, args map[string]string) (*protocol.PromptGetResult, error) {
			return &protocol.PromptGetResult{
				Description: "How to safely refactor code using LSP-assisted tools",
				Messages: []protocol.PromptMessage{
					{
						Role:    "user",
						Content: protocol.TextContent(refactoringGuidePrompt),
					},
				},
			}, nil
		},
	)
}
```

**Step 4: Delete handler.go and tools.go**

```bash
rm internal/mcp/handler.go internal/mcp/tools.go
```

Also delete `internal/mcp/bridge.go` since it was moved to `internal/tools/`:

```bash
rm internal/mcp/bridge.go
```

**Step 5: Fix the lspNotificationHandler**

Check what `subprocess.NewPool` expects for the notification handler parameter.
The current code passes a `jsonrpc.Handler` — verify the type signature and ensure
the new server.go matches. The handler type from purse-first's jsonrpc package is:

```go
type Handler func(ctx context.Context, msg *Message) (*Message, error)
```

Use the purse-first import path for this type.

**Step 6: Verify compilation**

```bash
go build ./...
```

Expected: Build succeeds. Fix any compilation errors from type mismatches.

**Step 7: Commit**

```bash
git add internal/mcp/ internal/tools/
git commit -m "feat: replace custom MCP server with purse-first server.Server"
```

---

### Task 5: Update cmd/lux/main.go

**Files:**
- Modify: `cmd/lux/main.go`

**Step 1: Add generate-plugin subcommand**

Add a hidden `generate-plugin` command to Cobra:

```go
var generatePluginCmd = &cobra.Command{
	Use:    "generate-plugin [output-dir]",
	Short:  "Generate purse-first plugin artifacts",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app := tools.RegisterAll(nil)
		return app.GenerateAll(args[0])
	},
}
```

Add the import for `tools` package and register the command in `init()`:

```go
rootCmd.AddCommand(generatePluginCmd)
```

**Step 2: Update MCP server construction imports**

The MCP commands (`mcpStdioCmd`, `mcpSSECmd`, `mcpHTTPCmd`) call `mcp.New(cfg, t)`.
This function signature stays the same, so the only change needed is verifying the
transport import path is updated (should already be done in Task 1).

**Step 3: Verify compilation**

```bash
go build ./cmd/lux/
```

**Step 4: Commit**

```bash
git add cmd/lux/main.go
git commit -m "feat: add generate-plugin subcommand for purse-first artifacts"
```

---

### Task 6: Update flake.nix with postInstall

**Files:**
- Modify: `flake.nix`

**Step 1: Add postInstall to the lux derivation**

Add `postInstall` to the `buildGoApplication` call, matching grit's pattern:

```nix
lux = pkgs.buildGoApplication {
  pname = "lux";
  inherit version;
  src = ./.;
  modules = ./gomod2nix.toml;
  subPackages = [ "cmd/lux" ];

  postInstall = ''
    $out/bin/lux generate-plugin $out
  '';

  meta = with pkgs.lib; {
    description = "LSP Multiplexer that routes requests to language servers based on file type";
    homepage = "https://github.com/amarbel-llc/lux";
    license = licenses.mit;
  };
};
```

**Step 2: Update gomod2nix.toml**

Run gomod2nix to update the dependency hashes:

```bash
gomod2nix
```

**Step 3: Build with nix**

```bash
nix build --show-trace
```

Expected: Build succeeds and `result/share/purse-first/lux/plugin.json` exists.

**Step 4: Verify generated artifacts**

```bash
ls result/share/purse-first/lux/
cat result/share/purse-first/lux/plugin.json
ls result/share/man/man1/
ls result/share/bash-completion/completions/
```

**Step 5: Commit**

```bash
git add flake.nix gomod2nix.toml
git commit -m "feat: add purse-first artifact generation to nix build"
```

---

### Task 7: Verify end-to-end functionality

**Files:** None (testing only)

**Step 1: Build and test MCP stdio mode**

```bash
nix build --show-trace
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.1.0"}}}' | ./result/bin/lux mcp stdio
```

Expected: Returns an initialize response with tools, resources, and prompts capabilities.

**Step 2: Test tools/list**

Send an initialize followed by tools/list to verify all 10 tools are registered.

**Step 3: Test generate-plugin output**

```bash
./result/bin/lux generate-plugin /tmp/lux-test
cat /tmp/lux-test/share/purse-first/lux/plugin.json
```

Expected: Valid plugin.json with mcpServers configuration.

**Step 4: Test CLI commands still work**

```bash
./result/bin/lux --help
./result/bin/lux list
./result/bin/lux serve --help
```

Expected: All Cobra CLI commands work as before.

**Step 5: Final commit if any fixes needed**

```bash
git add -A
git commit -m "fix: resolve issues found during end-to-end testing"
```
