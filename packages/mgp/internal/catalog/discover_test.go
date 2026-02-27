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

	// Verify GraphQL client was stored on catalog
	if cat.GraphQLClient == nil {
		t.Fatal("expected GraphQLClient to be set")
	}
	cat.GraphQLClient.Close()
}
