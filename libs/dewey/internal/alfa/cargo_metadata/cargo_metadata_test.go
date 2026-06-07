package cargo_metadata

import (
	"os/exec"
	"strings"
	"testing"

	topological_sort "github.com/amarbel-llc/purse-first/libs/dewey/internal/0/topological_sort"
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

	want := topological_sort.Edge{Source: "internal/alfa/store", Target: "internal/0/blob_store_id"}
	got := edges["internal"]
	if len(got) != 1 || got[0] != want {
		t.Errorf("edges = %v, want [%v]", got, want)
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
	want := topological_sort.Edge{Source: "internal/bar", Target: "internal/foo"}
	if len(edges["internal"]) != 1 || edges["internal"][0] != want {
		t.Errorf("depth-2 edges = %v, want [%v]", edges["internal"], want)
	}
}

func TestParseEdgesDropsCrossPrefixEdges(t *testing.T) {
	// Mirrors go_list: an edge whose endpoints live under different
	// prefixes is dropped from both groups (see repositioner's
	// TestRepositionerCrossPrefixEdgesIgnored for why this is safe).
	//
	// go_list-exact behavior check: both sources survive the depth trim
	// (droppedSources == 0), so the zero-edge guard (go_list.go:159) does
	// NOT fire — empty edge groups are returned, not an error.
	meta := `{"packages":[
	  {"name":"a_internal","manifest_path":"/ws/0/a/Cargo.toml","dependencies":[]},
	  {"name":"b_internal","manifest_path":"/ws/alfa/b/Cargo.toml",
	   "dependencies":[{"name":"a_internal","path":"/ws/0/a"}]}],
	  "workspace_root":"/ws"}`
	edges, err := parseEdges([]byte(meta), []string{"0", "alfa"}, 2, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(edges["alfa"]) != 0 || len(edges["0"]) != 0 {
		t.Errorf("cross-prefix edge must be dropped, got %v", edges)
	}
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
