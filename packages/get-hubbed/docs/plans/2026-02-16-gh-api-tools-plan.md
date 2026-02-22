# gh api Tools Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add three MCP tools (`api_get`, `graphql_query`, `graphql_mutation`) that expose the full GitHub API via `gh api`.

**Architecture:** New file `internal/tools/api.go` with three handlers following the existing pattern (typed params struct, build gh CLI args, return raw JSON). Registered via `registerAPITools(r)` in `registry.go`.

**Tech Stack:** Go, `go-lib-mcp` (server/protocol), `gh` CLI

---

### Task 1: Add `api_get` tool

**Files:**
- Create: `internal/tools/api.go`
- Modify: `internal/tools/registry.go`

**Step 1: Create `internal/tools/api.go` with `registerAPITools` and `handleAPIGet`**

```go
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/go-lib-mcp/protocol"
	"github.com/amarbel-llc/go-lib-mcp/server"
	"github.com/friedenberg/get-hubbed/internal/gh"
)

func registerAPITools(r *server.ToolRegistry) {
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
		handleAPIGet,
	)
}

func handleAPIGet(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
	var params struct {
		Endpoint string            `json:"endpoint"`
		Params   map[string]string `json:"params"`
		Headers  []string          `json:"headers"`
		Paginate bool              `json:"paginate"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	ghArgs := []string{"api", params.Endpoint, "--method", "GET"}

	for k, v := range params.Params {
		ghArgs = append(ghArgs, "-f", fmt.Sprintf("%s=%s", k, v))
	}

	for _, h := range params.Headers {
		ghArgs = append(ghArgs, "-H", h)
	}

	if params.Paginate {
		ghArgs = append(ghArgs, "--paginate")
	}

	out, err := gh.Run(ctx, ghArgs...)
	if err != nil {
		return protocol.ErrorResult(fmt.Sprintf("gh api: %v", err)), nil
	}

	return &protocol.ToolCallResult{
		Content: []protocol.Content{{Type: "text", Text: out}},
	}, nil
}
```

**Step 2: Add `registerAPITools(r)` to `RegisterAll` in `registry.go`**

Add after the existing `registerPRTools(r)` line:

```go
registerAPITools(r)
```

**Step 3: Verify it builds**

Run: `just build`
Expected: Success

**Step 4: Commit**

```
feat: add api_get tool for REST API GET requests
```

---

### Task 2: Add `graphql_query` tool

**Files:**
- Modify: `internal/tools/api.go`

**Step 1: Add `graphql_query` registration to `registerAPITools`**

Add to `registerAPITools` after the `api_get` registration:

```go
	r.Register(
		"graphql_query",
		"Execute a read-only GraphQL query against the GitHub API",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "The GraphQL query string"
				},
				"variables": {
					"type": "object",
					"description": "GraphQL variables as key-value pairs",
					"additionalProperties": {}
				},
				"paginate": {
					"type": "boolean",
					"description": "Auto-paginate results (requires endCursor/pageInfo in query)"
				}
			},
			"required": ["query"]
		}`),
		handleGraphQLQuery,
	)
```

**Step 2: Add `handleGraphQLQuery` handler**

```go
func handleGraphQLQuery(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
	var params struct {
		Query     string                 `json:"query"`
		Variables map[string]interface{} `json:"variables"`
		Paginate  bool                   `json:"paginate"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	ghArgs := []string{"api", "graphql", "-f", fmt.Sprintf("query=%s", params.Query)}

	for k, v := range params.Variables {
		ghArgs = append(ghArgs, "-F", fmt.Sprintf("%s=%v", k, v))
	}

	if params.Paginate {
		ghArgs = append(ghArgs, "--paginate")
	}

	out, err := gh.Run(ctx, ghArgs...)
	if err != nil {
		return protocol.ErrorResult(fmt.Sprintf("gh api graphql: %v", err)), nil
	}

	return &protocol.ToolCallResult{
		Content: []protocol.Content{{Type: "text", Text: out}},
	}, nil
}
```

**Step 3: Verify it builds**

Run: `just build`
Expected: Success

**Step 4: Commit**

```
feat: add graphql_query tool for read-only GraphQL queries
```

---

### Task 3: Add `graphql_mutation` tool

**Files:**
- Modify: `internal/tools/api.go`

**Step 1: Add `graphql_mutation` registration to `registerAPITools`**

Add after the `graphql_query` registration:

```go
	r.Register(
		"graphql_mutation",
		"Execute a GraphQL mutation against the GitHub API",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "The GraphQL mutation string"
				},
				"variables": {
					"type": "object",
					"description": "GraphQL variables as key-value pairs",
					"additionalProperties": {}
				}
			},
			"required": ["query"]
		}`),
		handleGraphQLMutation,
	)
```

**Step 2: Add `handleGraphQLMutation` handler**

```go
func handleGraphQLMutation(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
	var params struct {
		Query     string                 `json:"query"`
		Variables map[string]interface{} `json:"variables"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	ghArgs := []string{"api", "graphql", "-f", fmt.Sprintf("query=%s", params.Query)}

	for k, v := range params.Variables {
		ghArgs = append(ghArgs, "-F", fmt.Sprintf("%s=%v", k, v))
	}

	out, err := gh.Run(ctx, ghArgs...)
	if err != nil {
		return protocol.ErrorResult(fmt.Sprintf("gh api graphql mutation: %v", err)), nil
	}

	return &protocol.ToolCallResult{
		Content: []protocol.Content{{Type: "text", Text: out}},
	}, nil
}
```

**Step 3: Verify it builds**

Run: `just build`
Expected: Success

**Step 4: Commit**

```
feat: add graphql_mutation tool for GraphQL mutations
```

---

### Task 4: Verify full build and regenerate gomod2nix

**Step 1: Run full build**

Run: `just build`
Expected: Success with all three new tools registered

**Step 2: Verify nix build**

Run: `nix build`
Expected: Success
