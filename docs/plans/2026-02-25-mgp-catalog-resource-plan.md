# mgp Catalog Resource Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Expose the mgp tool catalog as an MCP resource at `mgp://catalog` so hosts can passively include tool metadata in context without requiring a GraphQL query round-trip.

**Architecture:** Register a single MCP resource in `main.go` using the existing `server.ResourceRegistry` from go-mcp. The resource reader serializes the in-memory catalog (built at startup) into a JSON document grouped by server, with metadata-only tool entries (no inputSchema). The query and exec tools remain unchanged.

**Tech Stack:** Go, `libs/go-mcp` (`server.ResourceRegistry`, `protocol.Resource`, `protocol.ResourceContent`)

---

### Task 1: Add catalog serialization type and function

**Files:**
- Create: `packages/mgp/internal/catalog/resource.go`
- Create: `packages/mgp/internal/catalog/resource_test.go`

**Step 1: Write the failing test**

```go
// packages/mgp/internal/catalog/resource_test.go
package catalog

import (
	"encoding/json"
	"testing"
)

func TestCatalogResourceJSON_Empty(t *testing.T) {
	cat := NewCatalog()
	data, err := CatalogResourceJSON(cat)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result catalogResource
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(result.Servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(result.Servers))
	}
}

func TestCatalogResourceJSON_GroupsByServer(t *testing.T) {
	cat := NewCatalog()

	readOnly := true
	cat.AddTool(CatalogTool{
		Name:        "status",
		Title:       "Show status",
		Description: "Show working tree status",
		Package:     "grit",
		ReadOnly:    &readOnly,
	})
	cat.AddTool(CatalogTool{
		Name:        "diff",
		Title:       "Show diff",
		Description: "Show changes",
		Package:     "grit",
	})
	cat.AddTool(CatalogTool{
		Name:        "repo_view",
		Title:       "View repo",
		Description: "View repository details",
		Package:     "get-hubbed",
	})

	data, err := CatalogResourceJSON(cat)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result catalogResource
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(result.Servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(result.Servers))
	}

	// Find grit server
	var grit *catalogServer
	for i := range result.Servers {
		if result.Servers[i].Name == "grit" {
			grit = &result.Servers[i]
			break
		}
	}
	if grit == nil {
		t.Fatal("grit server not found")
	}
	if len(grit.Tools) != 2 {
		t.Errorf("expected 2 grit tools, got %d", len(grit.Tools))
	}
}

func TestCatalogResourceJSON_OmitsInputSchema(t *testing.T) {
	cat := NewCatalog()
	cat.AddTool(CatalogTool{
		Name:        "status",
		Package:     "grit",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	})

	data, err := CatalogResourceJSON(cat)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The raw JSON should not contain "inputSchema"
	if json.Valid(data) {
		var raw map[string]interface{}
		json.Unmarshal(data, &raw)
		servers := raw["servers"].([]interface{})
		server := servers[0].(map[string]interface{})
		tools := server["tools"].([]interface{})
		tool := tools[0].(map[string]interface{})
		if _, exists := tool["inputSchema"]; exists {
			t.Error("inputSchema should be omitted from resource JSON")
		}
	}
}

func TestCatalogResourceJSON_OmitsNullAnnotations(t *testing.T) {
	cat := NewCatalog()
	readOnly := true
	cat.AddTool(CatalogTool{
		Name:     "status",
		Package:  "grit",
		ReadOnly: &readOnly,
		// Destructive, Idempotent, OpenWorld are nil
	})

	data, err := CatalogResourceJSON(cat)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var raw map[string]interface{}
	json.Unmarshal(data, &raw)
	servers := raw["servers"].([]interface{})
	server := servers[0].(map[string]interface{})
	tools := server["tools"].([]interface{})
	tool := tools[0].(map[string]interface{})

	if _, exists := tool["readOnly"]; !exists {
		t.Error("readOnly should be present when set")
	}
	if _, exists := tool["destructive"]; exists {
		t.Error("destructive should be omitted when nil")
	}
	if _, exists := tool["idempotent"]; exists {
		t.Error("idempotent should be omitted when nil")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first/.spinclass/mcp && nix develop ./devenvs/go -c go test ./packages/mgp/internal/catalog/ -run 'TestCatalogResource' -v`
Expected: FAIL — `CatalogResourceJSON` undefined

**Step 3: Write minimal implementation**

```go
// packages/mgp/internal/catalog/resource.go
package catalog

import "encoding/json"

type catalogResourceTool struct {
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	ReadOnly    *bool  `json:"readOnly,omitempty"`
	Destructive *bool  `json:"destructive,omitempty"`
	Idempotent  *bool  `json:"idempotent,omitempty"`
}

type catalogServer struct {
	Name  string                `json:"name"`
	Tools []catalogResourceTool `json:"tools"`
}

type catalogResource struct {
	Servers []catalogServer `json:"servers"`
}

// CatalogResourceJSON serializes the catalog into JSON grouped by server,
// with metadata-only tool entries (no inputSchema).
func CatalogResourceJSON(cat *Catalog) ([]byte, error) {
	byServer := make(map[string][]catalogResourceTool)

	for _, t := range cat.Tools {
		byServer[t.Package] = append(byServer[t.Package], catalogResourceTool{
			Name:        t.Name,
			Title:       t.Title,
			Description: t.Description,
			ReadOnly:    t.ReadOnly,
			Destructive: t.Destructive,
			Idempotent:  t.Idempotent,
		})
	}

	servers := make([]catalogServer, 0, len(byServer))
	for name, tools := range byServer {
		servers = append(servers, catalogServer{
			Name:  name,
			Tools: tools,
		})
	}

	return json.Marshal(catalogResource{Servers: servers})
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first/.spinclass/mcp && nix develop ./devenvs/go -c go test ./packages/mgp/internal/catalog/ -run 'TestCatalogResource' -v`
Expected: PASS

**Step 5: Commit**

```
git add packages/mgp/internal/catalog/resource.go packages/mgp/internal/catalog/resource_test.go
git commit -m "feat(mgp): add catalog resource JSON serialization"
```

---

### Task 2: Register the catalog resource in main.go

**Files:**
- Modify: `packages/mgp/cmd/mgp/main.go:1-82`

**Step 1: Add the resource registry**

After the tool registry block (lines 66-67) and before `server.New` (line 69),
add the resource registry:

```go
	resources := server.NewResourceRegistry()
	resources.RegisterResource(
		protocol.Resource{
			URI:         "mgp://catalog",
			Name:        "Tool Catalog",
			Description: "Complete catalog of tools available across all MCP servers",
			MimeType:    "application/json",
		},
		func(ctx context.Context, uri string) (*protocol.ResourceReadResult, error) {
			data, err := catalog.CatalogResourceJSON(cat)
			if err != nil {
				return nil, fmt.Errorf("serializing catalog: %w", err)
			}
			return &protocol.ResourceReadResult{
				Contents: []protocol.ResourceContent{{
					URI:      uri,
					MimeType: "application/json",
					Text:     string(data),
				}},
			}, nil
		},
	)
```

Add `Resources: resources` to the `server.Options` struct (alongside `Tools: registry`):

```go
	srv, err := server.New(t, server.Options{
		ServerName:    app.Name,
		ServerVersion: app.Version,
		Instructions:  "Model graph protocol MCP server. Query and execute tools from the purse-first tool catalog via GraphQL.",
		Tools:         registry,
		Resources:     resources,
	})
```

Add the `protocol` import to the import block:

```go
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
```

**Step 2: Verify it compiles**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first/.spinclass/mcp && nix develop ./devenvs/go -c go build ./packages/mgp/cmd/mgp/`
Expected: Success (no output)

**Step 3: Commit**

```
git add packages/mgp/cmd/mgp/main.go
git commit -m "feat(mgp): register tool catalog as MCP resource"
```

---

### Task 3: Verify the Nix build

**Files:** None modified — this validates the build pipeline.

**Step 1: Build the mgp package via Nix**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first/.spinclass/mcp && nix build .#mgp --show-trace`
Expected: Success, `result` symlink created

**Step 2: Verify the binary runs**

Run: `./result/bin/mgp --help`
Expected: Usage output mentioning flags

**Step 3: Run all Go tests**

Run: `cd /Users/sfriedenberg/eng/repos/purse-first/.spinclass/mcp && nix develop ./devenvs/go -c go test ./packages/mgp/...`
Expected: PASS
