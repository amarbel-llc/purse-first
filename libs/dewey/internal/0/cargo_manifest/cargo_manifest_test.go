package cargo_manifest

import (
	"strings"
	"testing"
)

const fixtureInline = `# top comment
[package]
name = "store_internal" # trailing comment
version = "0.1.0"

[dependencies]
blob_store_id_internal = { path = "../../0/blob_store_id" } # keep me
serde = "1"
`

const fixtureTable = `[package]
name = "store_internal"
version = "0.1.0"

[dependencies.blob_store_id_internal]
path = "../../0/blob_store_id"
features = ["x"]
`

func TestRewritePathDepsInline(t *testing.T) {
	out, n, err := RewritePathDeps([]byte(fixtureInline),
		"../../0/blob_store_id", "../../alfa/blob_store_id")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("rewrites = %d, want 1", n)
	}
	if !strings.Contains(string(out), `{ path = "../../alfa/blob_store_id" } # keep me`) {
		t.Errorf("inline path not rewritten or comment lost:\n%s", out)
	}
	if !strings.Contains(string(out), "# top comment") {
		t.Errorf("comments must be preserved")
	}
}

func TestRewritePathDepsTableForm(t *testing.T) {
	out, n, err := RewritePathDeps([]byte(fixtureTable),
		"../../0/blob_store_id", "../../alfa/blob_store_id")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || !strings.Contains(string(out), `path = "../../alfa/blob_store_id"`) {
		t.Errorf("table-form path not rewritten (n=%d):\n%s", n, out)
	}
}

func TestRewritePathDepsDollarInNewPath(t *testing.T) {
	out, n, err := RewritePathDeps([]byte(fixtureInline),
		"../../0/blob_store_id", "../$weird/loc")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("rewrites = %d, want 1", n)
	}
	if !strings.Contains(string(out), `{ path = "../$weird/loc" } # keep me`) {
		t.Errorf("$ in newPath not emitted literally:\n%s", out)
	}
}

func TestRewritePathDepsNoMatchIsNoop(t *testing.T) {
	out, n, err := RewritePathDeps([]byte(fixtureInline), "../../0/nope", "../../1/nope")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 || string(out) != fixtureInline {
		t.Errorf("expected byte-identical noop, n=%d", n)
	}
}

func TestRenameDepKeyInline(t *testing.T) {
	out, n, err := RenameDepKey([]byte(fixtureInline),
		"blob_store_id_internal", "blob_id_internal")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || !strings.Contains(string(out), `blob_id_internal = { path =`) {
		t.Errorf("dep key not renamed (n=%d):\n%s", n, out)
	}
	if strings.Contains(string(out), "\nblob_store_id_internal") {
		t.Errorf("old key still present")
	}
}

func TestRenameDepKeyTableHeader(t *testing.T) {
	out, n, err := RenameDepKey([]byte(fixtureTable),
		"blob_store_id_internal", "blob_id_internal")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || !strings.Contains(string(out), "[dependencies.blob_id_internal]") {
		t.Errorf("table header not renamed (n=%d):\n%s", n, out)
	}
}

func TestSetPackageName(t *testing.T) {
	out, err := SetPackageName([]byte(fixtureInline), "store2_internal")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `name = "store2_internal" # trailing comment`) {
		t.Errorf("package name not rewritten / comment lost:\n%s", out)
	}
}

func TestSetPackageNameIgnoresDependencySections(t *testing.T) {
	manifest := "[package]\nname = \"a\"\n\n[dependencies.name]\npath = \"../name\"\n"
	out, err := SetPackageName([]byte(manifest), "b")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "[dependencies.name]") {
		t.Errorf("dependency table mangled:\n%s", out)
	}
}

func TestRewritePathDepsStopsAtArrayOfTablesHeader(t *testing.T) {
	manifest := `[dependencies]
foo = { path = "../foo" }

[[bin]]
name = "tool"
path = "../foo"
`
	out, n, err := RewritePathDeps([]byte(manifest), "../foo", "../bar")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("rewrites = %d, want 1 (must not touch [[bin]] path)", n)
	}
	if !strings.Contains(string(out), "[[bin]]\nname = \"tool\"\npath = \"../foo\"\n") {
		t.Errorf("[[bin]] section mangled:\n%s", out)
	}
}

const fixtureWorkspace = `[workspace]
resolver = "2"
members = [
  "internal/0/blob_store_id", # tier 0
  "pkgs/blob_store_id",
]
`

func TestReplaceMember(t *testing.T) {
	out, ok, err := ReplaceMember([]byte(fixtureWorkspace),
		"internal/0/blob_store_id", "internal/alfa/blob_store_id")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if !strings.Contains(string(out), `"internal/alfa/blob_store_id", # tier 0`) {
		t.Errorf("member not replaced in place:\n%s", out)
	}
}

func TestAddMemberAppendsOnceIdempotently(t *testing.T) {
	out, added, err := AddMember([]byte(fixtureWorkspace), "pkgs/store")
	if err != nil || !added {
		t.Fatalf("added=%v err=%v", added, err)
	}
	out2, added2, err := AddMember(out, "pkgs/store")
	if err != nil || added2 {
		t.Fatalf("second add: added=%v err=%v", added2, err)
	}
	if string(out2) != string(out) {
		t.Errorf("second add changed content")
	}
}
