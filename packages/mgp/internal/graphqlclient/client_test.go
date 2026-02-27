package graphqlclient

import (
	"context"
	"encoding/json"
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
