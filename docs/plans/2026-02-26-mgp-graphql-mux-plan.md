# mgp GraphQL Mux Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable mgp to spawn a local GraphQL server over stdio, discover tools
via introspection, forward `query` calls to it, and route `exec` calls to its
paired MCP server.

**Architecture:** New `--graphql-server` flag spawns a GraphQL subprocess at
startup. mgp communicates via newline-delimited JSON (one request line, one
response line). Tools discovered from this source supplement the plugin.json
catalog. The `query` tool forwards all queries to the remote when configured.
`exec` routes to the paired MCP server (same binary, default mode).

**Tech Stack:** Go, `graphql-go/graphql`, `go-mcp` library, newline-delimited
JSON over stdio

---

### Task 1: Add ServerSource to Catalog

**Files:**
- Modify: `packages/mgp/internal/catalog/catalog.go`
- Test: `packages/mgp/internal/catalog/catalog_test.go`

**Step 1: Write the failing test**

Create `packages/mgp/internal/catalog/catalog_test.go`:

```go
package catalog

import "testing"

func TestServerEntry_DefaultSourceIsPlugin(t *testing.T) {
	entry := ServerEntry{
		Name:    "grit",
		Command: "/bin/grit",
	}
	if entry.Source != SourcePlugin {
		t.Errorf("expected SourcePlugin (0), got %d", entry.Source)
	}
}

func TestServerEntry_GraphQLSource(t *testing.T) {
	entry := ServerEntry{
		Name:    "remote-tool",
		Command: "/bin/graphql-server",
		Source:  SourceGraphQL,
	}
	if entry.Source != SourceGraphQL {
		t.Errorf("expected SourceGraphQL (1), got %d", entry.Source)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first/.worktrees/mgp-mux && go test ./packages/mgp/internal/catalog/ -run TestServerEntry -v`

Expected: FAIL — `SourcePlugin` and `SourceGraphQL` undefined.

**Step 3: Write minimal implementation**

Add to the top of `packages/mgp/internal/catalog/catalog.go`, after the import
block:

```go
type ServerSource int

const (
	SourcePlugin  ServerSource = iota // discovered via plugin.json
	SourceGraphQL                     // discovered via GraphQL server
)
```

Add `Source` field to `ServerEntry`:

```go
type ServerEntry struct {
	Name    string
	Command string
	Args    []string
	Source  ServerSource
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first/.worktrees/mgp-mux && go test ./packages/mgp/internal/catalog/ -v`

Expected: All tests PASS (existing resource tests + new catalog tests).

**Step 5: Commit**

```
git add packages/mgp/internal/catalog/catalog.go packages/mgp/internal/catalog/catalog_test.go
git commit -m "feat(mgp): add ServerSource type to catalog for origin tracking"
```

---

### Task 2: GraphQL Client — Spawn and Close

**Files:**
- Create: `packages/mgp/internal/graphqlclient/client.go`
- Test: `packages/mgp/internal/graphqlclient/client_test.go`

**Step 1: Write the failing test**

Create `packages/mgp/internal/graphqlclient/client_test.go`:

```go
package graphqlclient

import (
	"context"
	"testing"
)

func TestSpawn_InvalidCommand(t *testing.T) {
	ctx := context.Background()
	_, err := Spawn(ctx, "/nonexistent/binary")
	if err == nil {
		t.Fatal("expected error for nonexistent binary")
	}
}

func TestSpawn_EchoAndClose(t *testing.T) {
	ctx := context.Background()
	// cat will echo stdin to stdout — proves pipes work
	client, err := Spawn(ctx, "cat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close error: %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first/.worktrees/mgp-mux && go test ./packages/mgp/internal/graphqlclient/ -run TestSpawn -v`

Expected: FAIL — package does not exist.

**Step 3: Write minimal implementation**

Create `packages/mgp/internal/graphqlclient/client.go`:

```go
package graphqlclient

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
)

type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
}

func Spawn(ctx context.Context, command string, args ...string) (*Client, error) {
	cmd := exec.CommandContext(ctx, command, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting process %s: %w", command, err)
	}

	return &Client{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewScanner(stdout),
	}, nil
}

func (c *Client) Close() error {
	c.stdin.Close()
	if c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}
	return c.cmd.Wait()
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first/.worktrees/mgp-mux && go test ./packages/mgp/internal/graphqlclient/ -v`

Expected: PASS.

**Step 5: Commit**

```
git add packages/mgp/internal/graphqlclient/
git commit -m "feat(mgp): add graphqlclient package with Spawn and Close"
```

---

### Task 3: GraphQL Client — Query Method

**Files:**
- Modify: `packages/mgp/internal/graphqlclient/client.go`
- Modify: `packages/mgp/internal/graphqlclient/client_test.go`

**Step 1: Write the failing test**

The test needs a fake GraphQL server. Use a small shell script via `bash -c`
that reads a JSON line from stdin and writes a JSON response line to stdout.

Add to `client_test.go`:

```go
func TestQuery_RoundTrip(t *testing.T) {
	ctx := context.Background()

	// Fake GraphQL server: reads one JSON line, responds with a fixed JSON line
	script := `read line; echo '{"data":{"tools":[{"name":"test-tool"}]}}'`
	client, err := Spawn(ctx, "bash", "-c", script)
	if err != nil {
		t.Fatalf("spawn error: %v", err)
	}
	defer client.Close()

	result, err := client.Query(ctx, "{ tools { name } }", nil)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}

	// Verify we got valid JSON back with the expected structure
	var parsed struct {
		Data struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"data"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if len(parsed.Data.Tools) != 1 || parsed.Data.Tools[0].Name != "test-tool" {
		t.Errorf("unexpected response: %s", string(result))
	}
}

func TestQuery_WithVariables(t *testing.T) {
	ctx := context.Background()

	// Echo back the request so we can verify variables were sent
	script := `read line; echo "$line"`
	client, err := Spawn(ctx, "bash", "-c", script)
	if err != nil {
		t.Fatalf("spawn error: %v", err)
	}
	defer client.Close()

	vars := map[string]any{"package": "grit"}
	result, err := client.Query(ctx, "{ tools(package: $package) { name } }", vars)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}

	// The response is the echoed request — verify it contains our variables
	var req struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	if err := json.Unmarshal(result, &req); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if req.Variables["package"] != "grit" {
		t.Errorf("expected variable package=grit, got %v", req.Variables)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first/.worktrees/mgp-mux && go test ./packages/mgp/internal/graphqlclient/ -run TestQuery -v`

Expected: FAIL — `Query` method not defined.

**Step 3: Write minimal implementation**

Add to `client.go`:

```go
import "encoding/json"

type graphqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

func (c *Client) Query(ctx context.Context, query string, variables map[string]any) (json.RawMessage, error) {
	req := graphqlRequest{
		Query:     query,
		Variables: variables,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	data = append(data, '\n')
	if _, err := c.stdin.Write(data); err != nil {
		return nil, fmt.Errorf("writing request: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if !c.stdout.Scan() {
		if err := c.stdout.Err(); err != nil {
			return nil, fmt.Errorf("reading response: %w", err)
		}
		return nil, fmt.Errorf("reading response: unexpected EOF")
	}

	return json.RawMessage(c.stdout.Bytes()), nil
}
```

Note: `bufio.Scanner.Bytes()` returns the line without the trailing newline.

**Step 4: Run test to verify it passes**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first/.worktrees/mgp-mux && go test ./packages/mgp/internal/graphqlclient/ -v`

Expected: PASS.

**Step 5: Commit**

```
git add packages/mgp/internal/graphqlclient/
git commit -m "feat(mgp): add Query method to graphqlclient"
```

---

### Task 4: GraphQL Discovery — DiscoverGraphQL Function

**Files:**
- Modify: `packages/mgp/internal/catalog/discover.go`
- Test: `packages/mgp/internal/catalog/discover_test.go`

This task adds a `DiscoverGraphQL` function that spawns the GraphQL server,
sends an introspection query, then queries for tools to populate the catalog.

**Step 1: Write the failing test**

Create `packages/mgp/internal/catalog/discover_test.go`:

```go
package catalog

import (
	"context"
	"testing"
)

func TestDiscoverGraphQL_PopulatesCatalog(t *testing.T) {
	ctx := context.Background()

	// Fake GraphQL server that:
	// 1. Responds to introspection with a schema containing a "tools" query
	// 2. Responds to tools query with two tools from different packages
	script := `
while IFS= read -r line; do
  if echo "$line" | grep -q '__schema'; then
    echo '{"data":{"__schema":{"queryType":{"name":"Query"},"types":[{"name":"Query","kind":"OBJECT","fields":[{"name":"tools","type":{"name":null,"kind":"NON_NULL","ofType":{"name":null,"kind":"LIST"}}}]}]}}}'
  else
    echo '{"data":{"tools":[{"name":"status","package":"grit","description":"Show status"},{"name":"repo_view","package":"get-hubbed","description":"View repo"}]}}'
  fi
done
`
	cat := NewCatalog()
	err := DiscoverGraphQL(ctx, cat, "bash", "-c", script)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cat.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(cat.Tools))
	}

	// Verify server entries were created with GraphQL source
	grit, ok := cat.FindServer("grit")
	if !ok {
		t.Fatal("grit server not found")
	}
	if grit.Source != SourceGraphQL {
		t.Errorf("expected SourceGraphQL, got %d", grit.Source)
	}
	if grit.Command != "bash" {
		t.Errorf("expected command 'bash', got %q", grit.Command)
	}

	gh, ok := cat.FindServer("get-hubbed")
	if !ok {
		t.Fatal("get-hubbed server not found")
	}
	if gh.Source != SourceGraphQL {
		t.Errorf("expected SourceGraphQL, got %d", gh.Source)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first/.worktrees/mgp-mux && go test ./packages/mgp/internal/catalog/ -run TestDiscoverGraphQL -v`

Expected: FAIL — `DiscoverGraphQL` undefined.

**Step 3: Write minimal implementation**

Add to `packages/mgp/internal/catalog/discover.go`:

```go
import "github.com/amarbel-llc/mgp/internal/graphqlclient"

// DiscoverGraphQL spawns a GraphQL server, introspects its schema, queries for
// tools, and adds them to the catalog. Server entries are created with
// SourceGraphQL and the command set to the GraphQL server binary.
func DiscoverGraphQL(ctx context.Context, cat *Catalog, command string, args ...string) error {
	client, err := graphqlclient.Spawn(ctx, command, args...)
	if err != nil {
		return fmt.Errorf("spawning graphql server: %w", err)
	}

	// Send introspection query to verify the server is alive and has a schema
	_, err = client.Query(ctx, introspectionQuery, nil)
	if err != nil {
		client.Close()
		return fmt.Errorf("introspecting graphql server: %w", err)
	}

	// Query for tools
	result, err := client.Query(ctx, toolsQuery, nil)
	if err != nil {
		client.Close()
		return fmt.Errorf("querying tools: %w", err)
	}

	tools, err := parseToolsResponse(result)
	if err != nil {
		client.Close()
		return fmt.Errorf("parsing tools response: %w", err)
	}

	// Track which packages we've seen to create server entries
	seenPackages := make(map[string]bool)

	for _, tool := range tools {
		cat.AddTool(tool)

		if !seenPackages[tool.Package] {
			seenPackages[tool.Package] = true
			cat.AddServer(ServerEntry{
				Name:    tool.Package,
				Command: command,
				Args:    args,
				Source:  SourceGraphQL,
			})
		}
	}

	// Store the client on the catalog for query forwarding
	cat.GraphQLClient = client

	return nil
}

const introspectionQuery = `{ __schema { queryType { name } types { name kind fields { name type { name kind ofType { name kind } } } } } }`

const toolsQuery = `{ tools { name title description package inputSchema readOnly destructive idempotent openWorld } }`

func parseToolsResponse(data json.RawMessage) ([]CatalogTool, error) {
	var resp struct {
		Data struct {
			Tools []CatalogTool `json:"tools"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %s", resp.Errors[0].Message)
	}

	return resp.Data.Tools, nil
}
```

Also add to `catalog.go` in the `Catalog` struct:

```go
import "github.com/amarbel-llc/mgp/internal/graphqlclient"

type Catalog struct {
	Tools         []CatalogTool
	Servers       map[string]ServerEntry
	GraphQLClient *graphqlclient.Client
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first/.worktrees/mgp-mux && go test ./packages/mgp/internal/catalog/ -v`

Expected: All tests PASS.

**Step 5: Commit**

```
git add packages/mgp/internal/catalog/
git commit -m "feat(mgp): add DiscoverGraphQL for tool discovery via GraphQL server"
```

---

### Task 5: Query Tool — Forward to Remote GraphQL Server

**Files:**
- Modify: `packages/mgp/internal/tools/query.go`
- Modify: `packages/mgp/internal/tools/registry.go`

**Step 1: Write the failing test**

No separate test file needed — this is a wiring change. The behavior is: if
`cat.GraphQLClient` is non-nil, forward the query to it instead of executing
locally.

We'll verify via the integration test in Task 7. For now, modify the code.

**Step 2: Modify query.go**

Update `registerQueryCommand` to accept the catalog and check for a GraphQL
client:

```go
func registerQueryCommand(app *command.App, cat *catalog.Catalog, schema graphql.Schema) {
	app.AddCommand(&command.Command{
		Name:        "query",
		Title:       "Query Tool Catalog",
		Description: command.Description{Short: "Query the purse-first tool catalog using GraphQL"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:   protocol.BoolPtr(true),
			IdempotentHint: protocol.BoolPtr(true),
		},
		Params: []command.Param{
			{Name: "query", Type: command.String, Description: "GraphQL query string", Required: true},
		},
		Run: func(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
			var params struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
			}

			// Forward to remote GraphQL server if available
			if cat.GraphQLClient != nil {
				result, err := cat.GraphQLClient.Query(ctx, params.Query, nil)
				if err != nil {
					return command.TextErrorResult(fmt.Sprintf("graphql query error: %v", err)), nil
				}
				return command.TextResult(string(result)), nil
			}

			result, err := graphqlschema.Execute(schema, params.Query)
			if err != nil {
				return command.TextErrorResult(fmt.Sprintf("graphql execution error: %v", err)), nil
			}

			return command.TextResult(string(result)), nil
		},
	})
}
```

**Step 3: Run tests to verify nothing is broken**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first/.worktrees/mgp-mux && go test ./packages/mgp/... -v`

Expected: PASS — no behavioral change when `GraphQLClient` is nil (which it is
in all existing tests).

**Step 4: Commit**

```
git add packages/mgp/internal/tools/query.go
git commit -m "feat(mgp): forward query tool to remote GraphQL server when available"
```

---

### Task 6: Main — Wire Up --graphql-server Flag

**Files:**
- Modify: `packages/mgp/cmd/mgp/main.go`

**Step 1: Add the flag and discovery call**

Add `--graphql-server` flag to `main.go`. When provided, call
`catalog.DiscoverGraphQL` after the plugin.json discovery:

```go
func main() {
	pluginsDir := flag.String("plugins-dir", "", "path to share/purse-first/ directory containing plugin.json files")
	binDir := flag.String("bin-dir", "", "path to bin/ directory containing MCP server binaries")
	graphqlServer := flag.String("graphql-server", "", "command to spawn as GraphQL server (newline-delimited JSON over stdio)")

	// ... (existing flag.Usage, flag.Parse, subcommand handling unchanged)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	cat := discoverCatalog(ctx, *pluginsDir, *binDir)

	if *graphqlServer != "" {
		if err := catalog.DiscoverGraphQL(ctx, cat, *graphqlServer); err != nil {
			log.Fatalf("graphql server discovery failed: %v", err)
		}
	}

	app := tools.RegisterAll(cat)
	// ... (rest unchanged)
}
```

Also add cleanup for the GraphQL client on shutdown. After `srv.Run` returns:

```go
if cat.GraphQLClient != nil {
	cat.GraphQLClient.Close()
}
```

**Step 2: Build to verify compilation**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first/.worktrees/mgp-mux && go build ./packages/mgp/cmd/mgp/`

Expected: Compiles successfully.

**Step 3: Run all tests**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first/.worktrees/mgp-mux && go test ./packages/mgp/... -v`

Expected: PASS.

**Step 4: Commit**

```
git add packages/mgp/cmd/mgp/main.go
git commit -m "feat(mgp): add --graphql-server flag for GraphQL mux support"
```

---

### Task 7: Integration Smoke Test

**Files:**
- Create: `packages/mgp/internal/graphqlclient/integration_test.go`

**Step 1: Write integration test**

This test verifies the full flow: spawn a fake GraphQL server, discover tools,
verify catalog, and query forwarding.

```go
package graphqlclient

import (
	"context"
	"encoding/json"
	"testing"
)

func TestQuery_MultipleRoundTrips(t *testing.T) {
	ctx := context.Background()

	// Fake server that handles multiple requests
	script := `
while IFS= read -r line; do
  if echo "$line" | grep -q '__schema'; then
    echo '{"data":{"__schema":{"queryType":{"name":"Query"}}}}'
  elif echo "$line" | grep -q 'tools'; then
    echo '{"data":{"tools":[{"name":"test","package":"fake"}]}}'
  else
    echo '{"data":null}'
  fi
done
`
	client, err := Spawn(ctx, "bash", "-c", script)
	if err != nil {
		t.Fatalf("spawn error: %v", err)
	}
	defer client.Close()

	// First query
	r1, err := client.Query(ctx, "{ __schema { queryType { name } } }", nil)
	if err != nil {
		t.Fatalf("query 1 error: %v", err)
	}

	var schema struct {
		Data struct {
			Schema struct {
				QueryType struct {
					Name string `json:"name"`
				} `json:"queryType"`
			} `json:"__schema"`
		} `json:"data"`
	}
	if err := json.Unmarshal(r1, &schema); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if schema.Data.Schema.QueryType.Name != "Query" {
		t.Errorf("expected Query, got %s", schema.Data.Schema.QueryType.Name)
	}

	// Second query
	r2, err := client.Query(ctx, "{ tools { name } }", nil)
	if err != nil {
		t.Fatalf("query 2 error: %v", err)
	}

	var tools struct {
		Data struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"data"`
	}
	if err := json.Unmarshal(r2, &tools); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(tools.Data.Tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(tools.Data.Tools))
	}
}

func TestQuery_ServerExit(t *testing.T) {
	ctx := context.Background()

	// Server that exits immediately
	client, err := Spawn(ctx, "bash", "-c", "exit 0")
	if err != nil {
		t.Fatalf("spawn error: %v", err)
	}
	defer client.Close()

	_, err = client.Query(ctx, "{ tools { name } }", nil)
	if err == nil {
		t.Fatal("expected error when server exits")
	}
}
```

**Step 2: Run the integration tests**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first/.worktrees/mgp-mux && go test ./packages/mgp/internal/graphqlclient/ -v`

Expected: PASS.

**Step 3: Run all mgp tests**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first/.worktrees/mgp-mux && go test ./packages/mgp/... -v`

Expected: PASS.

**Step 4: Commit**

```
git add packages/mgp/internal/graphqlclient/integration_test.go
git commit -m "test(mgp): add integration tests for graphqlclient"
```

---

### Task 8: Scanner Bytes Copy Fix

**Note:** `bufio.Scanner.Bytes()` returns a slice that may be overwritten on the
next `Scan()` call. Since `Query` returns `json.RawMessage` (which is a
`[]byte`), the caller may hold a reference to stale memory if they don't
unmarshal immediately. Fix this by copying the bytes.

**Files:**
- Modify: `packages/mgp/internal/graphqlclient/client.go`

**Step 1: Add copy in Query method**

In the `Query` method, replace:

```go
return json.RawMessage(c.stdout.Bytes()), nil
```

with:

```go
line := make([]byte, len(c.stdout.Bytes()))
copy(line, c.stdout.Bytes())
return json.RawMessage(line), nil
```

**Step 2: Run tests**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first/.worktrees/mgp-mux && go test ./packages/mgp/... -v`

Expected: PASS.

**Step 3: Commit**

```
git add packages/mgp/internal/graphqlclient/client.go
git commit -m "fix(mgp): copy scanner bytes in graphqlclient to prevent stale references"
```
