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
