package cargo_metadata

import (
	"os/exec"
	"strings"
	"testing"

	topological_sort "code.linenisgreat.com/purse-first/libs/dewey/internal/0/topological_sort"
)

const goldenMetadata = `{
  "packages": [
    {
      "name": "blob_store_id_internal",
      "manifest_path": "/ws/internal/0/blob_store_id/Cargo.toml",
      "dependencies": []
    },
    {
      "name": "store_internal",
      "manifest_path": "/ws/internal/alfa/store/Cargo.toml",
      "dependencies": [
        {"name": "blob_store_id_internal", "path": "/ws/internal/0/blob_store_id"},
        {"name": "serde"}
      ]
    }
  ],
  "workspace_root": "/ws"
}`

func TestParseEdges(t *testing.T) {
	edges, err := parseEdges([]byte(goldenMetadata), []string{"internal"}, 3, false)
	if err != nil {
		t.Fatal(err)
	}

	assertEdges(t, edges["internal"],
		topological_sort.Edge{Source: "internal/alfa/store", Target: "internal/0/blob_store_id"},
		topological_sort.Edge{Source: "internal/alfa/store"},
		topological_sort.Edge{Source: "internal/0/blob_store_id"},
	)
}

// Mirrors go_list: a crate with no in-prefix edges is still declared, so
// the repositioner places it (purse-first#57, #220).
func TestParseEdgesDeclaresCratesWithoutEdges(t *testing.T) {
	meta := `{"packages":[
	  {"name":"lonely_internal","manifest_path":"/ws/internal/lonely/Cargo.toml","dependencies":[]}],
	  "workspace_root":"/ws"}`
	edges, err := parseEdges([]byte(meta), []string{"internal"}, 2, false)
	if err != nil {
		t.Fatal(err)
	}

	assertEdges(t, edges["internal"], topological_sort.Edge{Source: "internal/lonely"})
}

// assertEdges compares edges against want as sets.
func assertEdges(t *testing.T, edges []topological_sort.Edge, want ...topological_sort.Edge) {
	t.Helper()

	got := make(map[topological_sort.Edge]bool, len(edges))
	for _, e := range edges {
		got[e] = true
	}

	for _, w := range want {
		if !got[w] {
			t.Errorf("missing edge %+v in %v", w, edges)
		}
	}

	if len(edges) != len(want) {
		t.Errorf("expected %d edges, got %d: %v", len(want), len(edges), edges)
	}
}

func TestParseEdgesIgnoresRegistryDeps(t *testing.T) {
	edges, _ := parseEdges([]byte(goldenMetadata), []string{"internal"}, 3, false)
	for _, e := range edges["internal"] {
		if e.Target == "serde" {
			t.Fatal("registry dep leaked into edge set")
		}
	}
}

func TestParseEdgesDepth2FlatLayout(t *testing.T) {
	meta := `{"packages":[
	  {"name":"foo_internal","manifest_path":"/ws/internal/foo/Cargo.toml","dependencies":[]},
	  {"name":"bar_internal","manifest_path":"/ws/internal/bar/Cargo.toml",
	   "dependencies":[{"name":"foo_internal","path":"/ws/internal/foo"}]}],
	  "workspace_root":"/ws"}`
	edges, err := parseEdges([]byte(meta), []string{"internal"}, 2, false)
	if err != nil {
		t.Fatal(err)
	}
	assertEdges(t, edges["internal"],
		topological_sort.Edge{Source: "internal/bar", Target: "internal/foo"},
		topological_sort.Edge{Source: "internal/bar"},
		topological_sort.Edge{Source: "internal/foo"},
	)
}

func TestParseEdgesDropsCrossPrefixEdges(t *testing.T) {
	// Mirrors go_list: an edge whose endpoints live under different
	// prefixes is dropped from both groups (see repositioner's
	// TestRepositionerCrossPrefixEdgesIgnored for why this is safe).
	//
	// go_list-exact behavior check: both sources survive the depth trim
	// (droppedSources == 0), so the zero-edge guard (go_list.go:159) does
	// NOT fire — each group holds only its node declaration, not an error.
	meta := `{"packages":[
	  {"name":"a_internal","manifest_path":"/ws/0/a/Cargo.toml","dependencies":[]},
	  {"name":"b_internal","manifest_path":"/ws/alfa/b/Cargo.toml",
	   "dependencies":[{"name":"a_internal","path":"/ws/0/a"}]}],
	  "workspace_root":"/ws"}`
	edges, err := parseEdges([]byte(meta), []string{"0", "alfa"}, 2, false)
	if err != nil {
		t.Fatal(err)
	}
	assertEdges(t, edges["0"], topological_sort.Edge{Source: "0/a"})
	assertEdges(t, edges["alfa"], topological_sort.Edge{Source: "alfa/b"})
}

func TestParseEdgesErrorsOnFlatLayoutWithDefaultDepth(t *testing.T) {
	// Mirrors go_list's zero-edge guard (go_list.go:159): every source
	// under the prefix is too short for the requested depth, so the
	// prefix errors instead of silently returning nothing.
	meta := `{"packages":[
	  {"name":"foo_internal","manifest_path":"/ws/internal/foo/Cargo.toml","dependencies":[]},
	  {"name":"bar_internal","manifest_path":"/ws/internal/bar/Cargo.toml",
	   "dependencies":[{"name":"foo_internal","path":"/ws/internal/foo"}]}],
	  "workspace_root":"/ws"}`

	_, err := parseEdges([]byte(meta), []string{"internal"}, 0, false)
	if err == nil {
		t.Fatal("expected error for flat layout at default depth=3, got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "no edges computed") {
		t.Errorf("expected error to mention %q, got %q", "no edges computed", msg)
	}

	if !strings.Contains(msg, "--initial") {
		t.Errorf("expected error to hint at --initial, got %q", msg)
	}
}

func TestReadDependenciesRealCargo(t *testing.T) {
	if _, err := exec.LookPath("cargo"); err != nil {
		t.Skip("cargo not on PATH")
	}
	root := writeFixtureWorkspace(t)
	r := Reader{Dir: root, PackagePrefixes: []string{"internal"}, ComponentDepth: 3}
	edges, err := r.ReadDependencies()
	if err != nil {
		t.Fatal(err)
	}
	if len(edges["internal"]) == 0 {
		t.Fatal("expected at least one edge from fixture workspace")
	}
}
