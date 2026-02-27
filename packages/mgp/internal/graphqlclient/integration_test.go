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

	// First query — introspection
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

	// Second query — tools
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
