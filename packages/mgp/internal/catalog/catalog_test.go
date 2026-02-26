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
