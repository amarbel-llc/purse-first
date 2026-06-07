# dagnabit Rust Support Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use eng:subagent-driven-development to implement this plan task-by-task.

**Goal:** Extend dagnabit's reposition / export / move / rename subcommands to Rust cargo workspaces as a language mode of the existing binary, per the approved design at `docs/plans/2026-06-06-dagnabit-rust-design.md`.

**Architecture:** Per-language implementations behind the existing `DependencyReader`/`LevelMapper`/`PackageMover` seams (Approach A). New dewey packages: `internal/0/cargo_workspace` (workspace-root discovery), `internal/0/cargo_manifest` (span-based Cargo.toml edits), `internal/alfa/cargo_metadata` (dep-graph reader), `internal/echo/dagnabit_rust` (mover, exporter, renamer). CLI auto-detects language (go.mod vs workspace Cargo.toml) with a `-lang` override. Go code paths are untouched.

**Tech Stack:** Go (existing workspace), `github.com/BurntSushi/toml` (decode-only TOML reads), `cargo metadata` (shelled), `ast-grep` (shelled, rename rewrites), nix devshell additions (cargo/rustc/rustfmt/ast-grep).

**Rollback:** N/A — purely additive. Go mode is byte-for-byte unchanged; rust mode is a new lane. Release rollback = previous dagnabit tag.

**Executor notes (read first):**

- **Devshell caveat:** `direnv reload` does NOT work inside Claude sessions. After Task 1 lands, `cargo`/`ast-grep` are NOT on PATH until the user restarts the session. Every test that needs them MUST `exec.LookPath` + `t.Skip` (and BATS equivalents) so the suite stays green either way. Use `nix develop --command …` to run rust-needing tests before the restart.
- **dewey drift gate:** every new package under `libs/dewey/internal/` requires a generated facade in `libs/dewey/pkgs/` or `just lint-dewey_pkgs_drift` fails the merge hook. Each package task ends with `just dagnabit-build && just dewey-export-library` + committing the generated facade.
- **dewey tests run with `-tags test`:** `just test-dewey` = `go test -tags test ./libs/dewey/...`. Run single packages as `nix develop --command go test -tags test ./libs/dewey/internal/0/cargo_workspace/`.
- **Cheap compile checks** via `hamster.go-build` are fine; do NOT run full `just` before `merge-this-session` (the hook runs it).
- **cargo metadata JSON shape:** the plan's golden fixture assumes path-dependencies appear as `packages[].dependencies[].path` (absolute path, present only for path deps). Verify against real `cargo metadata` output in Task 5's integration test; if the field differs, fix the golden fixture to match reality — the integration test is the source of truth.
- Reference skills: @eng:test-driven-development for the loop discipline, @eng:wiring-bats-tests for Task 12.

---

### Task 1: devenvs/rust devshell

**Files:**
- Create: `devenvs/rust/default.nix`
- Modify: `flake.nix` (devenv import block ~line 57, devShells block ~lines 102-129)

**Step 1: Write `devenvs/rust/default.nix`**

```nix
# devenvs/rust/default.nix
#
# Rust toolchain for dagnabit's rust mode: cargo/rustc for fixture
# workspaces and the `cargo metadata` / `cargo check` gates; ast-grep
# for crate-rename source rewrites.
#
# Args:
#   pkgs        — stable nixpkgs (runtimes)
#   pkgs-master — unstable nixpkgs (latest tooling)
#
{
  pkgs,
  pkgs-master,
}:
let
  packages = {
    inherit (pkgs) cargo rustc rustfmt;
    inherit (pkgs-master) ast-grep;
  };
in
{
  inherit packages;

  devShells.default = pkgs.mkShell {
    packages = builtins.attrValues packages;
  };
}
```

**Step 2: Wire into `flake.nix`**

In the devenv let-block (after `bats = import ./devenvs/bats { inherit pkgs; };`):

```nix
rust = import ./devenvs/rust { inherit pkgs pkgs-master; };
```

In `devShells.default.inputsFrom`, append `devenvs.rust.devShells.default`. In the named shells block add `rust = devenvs.rust.devShells.default;`.

**Step 3: Verify**

Run: `nix develop .#rust --command cargo --version` and `nix develop .#rust --command ast-grep --version`
Expected: both print versions.

Run: `nix flake check` (or `just validate`)
Expected: PASS.

**Step 4: Commit**

```bash
git add devenvs/rust/default.nix flake.nix
git commit -m "feat(devenvs): rust devshell (cargo, rustc, rustfmt, ast-grep) for dagnabit rust mode"
```

Tell the user the devshell changed: full PATH availability needs a session restart with `direnv reload` in between. Continue regardless (tests skip).

---

### Task 2: TOML decode dependency

**Files:**
- Modify: `libs/dewey/go.mod` (via `go get`)
- Modify: `gomod2nix.toml` (via recipe)

**Step 1:** From `libs/dewey/`: `go get github.com/BurntSushi/toml@latest` (use `hamster.go-get` with cwd `libs/dewey`).

**Step 2:** Run `just build-nix-gomod2nix`. Expected: `gomod2nix.toml` gains a BurntSushi/toml entry; `go.work.sum` may update.

**Step 3: Commit**

```bash
git add libs/dewey/go.mod libs/dewey/go.sum gomod2nix.toml go.work.sum
git commit -m "build(dewey): add BurntSushi/toml for cargo manifest reads"
```

---

### Task 3: `internal/0/cargo_workspace` — workspace root discovery

Rust analog of `go_module.ResolveModulePath`. Pure file parsing — no cargo needed; tests run everywhere.

**Files:**
- Create: `libs/dewey/internal/0/cargo_workspace/cargo_workspace.go`
- Test: `libs/dewey/internal/0/cargo_workspace/cargo_workspace_test.go`

**Step 1: Write the failing tests**

```go
package cargo_workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFindRootAtWorkspaceDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Cargo.toml"), `
[workspace]
members = ["internal/0/blob_store_id", "pkgs/blob_store_id"]
resolver = "2"
`)

	ws, err := FindRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ws.RootDir != dir {
		t.Errorf("RootDir = %q, want %q", ws.RootDir, dir)
	}
	if len(ws.Members) != 2 || ws.Members[0] != "internal/0/blob_store_id" {
		t.Errorf("Members = %v", ws.Members)
	}
}

func TestFindRootWalksUpPastMemberManifest(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Cargo.toml"), "[workspace]\nmembers = [\"a\"]\n")
	// member crate has its own Cargo.toml WITHOUT [workspace]
	writeFile(t, filepath.Join(dir, "a", "Cargo.toml"), "[package]\nname = \"a\"\nversion = \"0.1.0\"\n")

	ws, err := FindRoot(filepath.Join(dir, "a"))
	if err != nil {
		t.Fatal(err)
	}
	if ws.RootDir != dir {
		t.Errorf("RootDir = %q, want %q", ws.RootDir, dir)
	}
}

func TestFindRootErrorsWithoutWorkspace(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Cargo.toml"), "[package]\nname = \"solo\"\nversion = \"0.1.0\"\n")

	if _, err := FindRoot(dir); err == nil {
		t.Fatal("expected error for crate without workspace root")
	}
}

func TestFindRootErrorsWithoutAnyManifest(t *testing.T) {
	if _, err := FindRoot(t.TempDir()); err == nil {
		t.Fatal("expected error when no Cargo.toml exists")
	}
}
```

**Step 2:** Run: `nix develop --command go test -tags test ./libs/dewey/internal/0/cargo_workspace/`
Expected: FAIL (package does not exist / FindRoot undefined).

**Step 3: Implement**

```go
// Package cargo_workspace discovers a cargo workspace root by walking up
// from a directory to the nearest Cargo.toml containing a [workspace]
// table. Rust analog of go_module's module-path discovery.
package cargo_workspace

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Workspace is a located cargo workspace.
type Workspace struct {
	// RootDir is the absolute directory containing the workspace Cargo.toml.
	RootDir string
	// Members is the [workspace] members list exactly as written
	// (relative dirs or globs).
	Members []string
}

type manifestWorkspace struct {
	Workspace *struct {
		Members []string `toml:"members"`
	} `toml:"workspace"`
}

// FindRoot walks up from dir to the nearest Cargo.toml whose contents
// include a [workspace] table. Crate manifests without [workspace] are
// skipped (the walk continues upward). Errors when the filesystem root
// is reached without finding one.
func FindRoot(dir string) (Workspace, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Workspace{}, err
	}

	for current := abs; ; {
		manifestPath := filepath.Join(current, "Cargo.toml")

		if _, err := os.Stat(manifestPath); err == nil {
			var m manifestWorkspace
			if _, err := toml.DecodeFile(manifestPath, &m); err != nil {
				return Workspace{}, fmt.Errorf("parsing %s: %w", manifestPath, err)
			}

			if m.Workspace != nil {
				return Workspace{RootDir: current, Members: m.Workspace.Members}, nil
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			return Workspace{}, fmt.Errorf(
				"no Cargo.toml with a [workspace] table found walking up from %s", abs)
		}
		current = parent
	}
}
```

**Step 4:** Run the test again. Expected: PASS.

**Step 5: Regenerate dewey facades, commit**

```bash
just dagnabit-build && just dewey-export-library
git add libs/dewey/internal/0/cargo_workspace libs/dewey/pkgs/cargo_workspace
git commit -m "feat(dewey/cargo_workspace): cargo workspace root discovery"
```

---

### Task 4: `internal/0/cargo_manifest` — span-based Cargo.toml edits

Comment/formatting-preserving edits, implemented as a section-aware line scanner (Go TOML libs don't round-trip comments). Handles the two dep forms: inline `foo = { path = "…" }` and `[dependencies.foo]` tables. Pure string manipulation — tests run everywhere.

**Files:**
- Create: `libs/dewey/internal/0/cargo_manifest/cargo_manifest.go`
- Test: `libs/dewey/internal/0/cargo_manifest/cargo_manifest_test.go`

**Step 1: Write the failing tests** (table-driven; the important cases)

```go
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
```

**Step 2:** Run: `nix develop --command go test -tags test ./libs/dewey/internal/0/cargo_manifest/`
Expected: FAIL (undefined functions).

**Step 3: Implement** — a line scanner that tracks the current `[section]` header and applies targeted `strings.Replace` within matching lines. Core shape (implement all five exported funcs in this style; keep each small):

```go
// Package cargo_manifest performs comment-preserving, span-based edits
// on Cargo.toml files. Go TOML libraries do not round-trip comments, so
// mutation is line-oriented: parse just enough structure (section
// headers, key positions) to locate the edit, then rewrite the raw text
// span. Reads that need full TOML semantics use BurntSushi/toml
// elsewhere; this package never re-serializes whole documents.
package cargo_manifest

import (
	"fmt"
	"regexp"
	"strings"
)

var sectionRE = regexp.MustCompile(`^\s*\[([^\]]+)\]\s*(?:#.*)?$`)

// forEachLine calls fn with (sectionName, line) and collects fn's
// replacement line. Section names are the literal header contents, e.g.
// "dependencies", "dependencies.foo", "workspace", "package".
func forEachLine(manifest []byte, fn func(section, line string) string) []byte {
	var section string
	lines := strings.Split(string(manifest), "\n")
	for i, line := range lines {
		if m := sectionRE.FindStringSubmatch(line); m != nil {
			section = m[1]
			continue
		}
		lines[i] = fn(section, line)
	}
	return []byte(strings.Join(lines, "\n"))
}

// RewritePathDeps replaces dependency `path = "<oldPath>"` values with
// newPath, in both inline-table deps under [dependencies] (and
// [dev-dependencies], [build-dependencies]) and table-form
// [dependencies.<name>] sections. Returns the rewritten manifest and
// the number of replacements. Zero replacements returns the input
// byte-identical.
func RewritePathDeps(manifest []byte, oldPath, newPath string) ([]byte, int, error) { … }

// RenameDepKey renames dependency key oldName to newName: inline-table
// keys (`oldName = { … }` / `oldName = "1"`) and table headers
// ([dependencies.oldName] and dev-/build- variants). …
func RenameDepKey(manifest []byte, oldName, newName string) ([]byte, int, error) { … }

// SetPackageName rewrites `name = "…"` inside the [package] section only. …
func SetPackageName(manifest []byte, newName string) ([]byte, error) { … }

// ReplaceMember swaps a [workspace] members entry string. …
func ReplaceMember(manifest []byte, oldRel, newRel string) ([]byte, bool, error) { … }

// AddMember appends rel to [workspace] members if absent (idempotent). …
func AddMember(manifest []byte, rel string) ([]byte, bool, error) { … }
```

Implementation rules the tests pin down: only lines inside the relevant section are touched; dependency-section matching covers `dependencies`, `dev-dependencies`, `build-dependencies` and their `.name` table forms; path matching is exact-string on the quoted value; everything else (comments, spacing, ordering) passes through untouched.

**Step 4:** Run tests. Expected: PASS. Iterate until all table cases pass.

**Step 5: Regenerate facades, commit**

```bash
just dagnabit-build && just dewey-export-library
git add libs/dewey/internal/0/cargo_manifest libs/dewey/pkgs/cargo_manifest
git commit -m "feat(dewey/cargo_manifest): comment-preserving span edits for Cargo.toml"
```

---

### Task 5: `internal/alfa/cargo_metadata` — DependencyReader

Peer of `*/go_list`. Two layers: a pure parse function tested on golden JSON (runs everywhere), and the shelling `Reader` tested against a real fixture workspace (skips without cargo). Mirror `go_list.Reader` semantics exactly: node = first `ComponentDepth` components of the manifest dir relative to the workspace root; edges grouped per prefix; cross-prefix edges dropped; intra-node (same trimmed node) edges dropped.

**Files:**
- Create: `libs/dewey/internal/alfa/cargo_metadata/cargo_metadata.go`
- Test: `libs/dewey/internal/alfa/cargo_metadata/cargo_metadata_test.go`
- Test fixture: golden JSON embedded as a const in the test file

**Step 1: Write failing tests**

```go
package cargo_metadata

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	topological_sort "github.com/amarbel-llc/purse-first/libs/dewey/internal/0/topological_sort"
)

// goldenMetadata is a trimmed real `cargo metadata --format-version 1
// --no-deps` output for a workspace at /ws with members
// internal/0/blob_store_id, internal/alfa/store (depends on
// blob_store_id via path), and one registry dep (serde) that must be
// ignored. Regenerate from the Task 5 fixture if cargo's schema moves.
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
	want := topological_sort.Edge{
		Source: "internal/alfa/store",
		Target: "internal/0/blob_store_id",
	}
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

func TestParseEdgesDepth2(t *testing.T) {
	// depth 2 trims to level/package, prefixes are level dirs
	meta := `{"packages":[
	  {"name":"a_internal","manifest_path":"/ws/0/a/Cargo.toml","dependencies":[]},
	  {"name":"b_internal","manifest_path":"/ws/alfa/b/Cargo.toml",
	   "dependencies":[{"name":"a_internal","path":"/ws/0/a"}]}],
	  "workspace_root":"/ws"}`
	edges, err := parseEdges([]byte(meta), []string{"0", "alfa"}, 2, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(edges["alfa"]) != 1 || edges["alfa"][0].Source != "alfa/b" {
		t.Errorf("depth-2 edges = %v", edges)
	}
}

// --- integration: real cargo ---

func writeFixtureWorkspace(t *testing.T) string { /* helper shared via fixtures_test.go, see Step 3 */ }

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
```

**Step 2:** Run; expected FAIL.

**Step 3: Implement.** `Reader` struct mirrors `go_list.Reader` fields (`Dir`, `PackagePrefixes`, `ComponentDepth`, `Verbose` — no ModulePath; the workspace root IS Dir). `ReadDependencies` runs `cargo metadata --format-version 1 --no-deps` in `Dir` (on failure: error containing argv + stderr), then delegates to `parseEdges(jsonBytes, prefixes, componentDepth, verbose)`:

1. Decode `packages[].{name,manifest_path,dependencies[].{name,path}}` + `workspace_root`.
2. Build crate-name → node map: node = `filepath.Dir(manifest_path)` made relative to `workspace_root`, trimmed to the first `componentDepth` path components; sources with fewer components are dropped (logged when verbose), exactly like `go_list`.
3. For each package and each dependency that has a non-empty `path` resolving to a known workspace member: emit `Edge{Source: node(pkg), Target: node(dep)}`.
4. Group per prefix: an edge belongs to prefix P when **both** endpoints' first path component == P (depth 3) / both nodes start with P (depth 2, where prefixes are the level dirs). Drop self-edges (same trimmed node) and cross-prefix edges.
5. A prefix that matched member crates but produced zero usable nodes errors (mirror `go_list`'s zero-edge guard).

Also create `fixtures_test.go` with `writeFixtureWorkspace(t *testing.T) string` building a real two-crate workspace in `t.TempDir()` (root `Cargo.toml` with `[workspace] members`, `internal/0/blob_store_id` lib crate, `internal/alfa/store` lib crate with a path-dep on it, each with a one-line `src/lib.rs`). Reuse this helper from Tasks 6/8/9/10 — export it behind the `test` build tag if cross-package reuse is needed, or duplicate the ~40 lines per package (duplication is acceptable; shared test helpers across dewey packages need the `-tags test` gating treated carefully).

**Step 4:** Run unit tests (pass everywhere) + integration test under `nix develop --command` (passes with cargo, else skips). Verify the golden JSON's `path` field matches real output here; correct the golden if not.

**Step 5: Regenerate facades, commit**

```bash
just dagnabit-build && just dewey-export-library
git add libs/dewey/internal/alfa/cargo_metadata libs/dewey/pkgs/cargo_metadata
git commit -m "feat(dewey/cargo_metadata): cargo-metadata DependencyReader for dagnabit rust mode"
```

---

### Task 6: `internal/echo/dagnabit_rust` — Mover (reposition support)

Implements `dagnabit.PackageMover`: `git mv` + workspace-wide path-dep rewrites + members update + `cargo metadata` gate.

**Files:**
- Create: `libs/dewey/internal/echo/dagnabit_rust/mover.go`
- Test: `libs/dewey/internal/echo/dagnabit_rust/mover_test.go` (+ `fixtures_test.go` fixture builder per Task 5)

**Step 1: Write failing tests**

```go
func TestMoverMovesCrateAndRewritesPathDeps(t *testing.T) {
	requireCargo(t) // LookPath + Skip helper in fixtures_test.go
	root := writeFixtureWorkspace(t) // also git init + commit (mover uses git mv)
	m := &Mover{WorkspaceRoot: root}

	if err := m.MovePackage("internal/0/blob_store_id", "internal/alfa/blob_store_id"); err != nil {
		t.Fatal(err)
	}

	// dir moved
	if _, err := os.Stat(filepath.Join(root, "internal/alfa/blob_store_id/Cargo.toml")); err != nil {
		t.Fatal("crate dir not moved")
	}
	// dependent's path-dep rewritten
	dep, _ := os.ReadFile(filepath.Join(root, "internal/alfa/store/Cargo.toml"))
	if !strings.Contains(string(dep), `path = "../blob_store_id"`) &&
		!strings.Contains(string(dep), `path = "../../alfa/blob_store_id"`) {
		t.Errorf("dependent path-dep not rewritten:\n%s", dep)
	}
	// members updated
	ws, _ := os.ReadFile(filepath.Join(root, "Cargo.toml"))
	if !strings.Contains(string(ws), "internal/alfa/blob_store_id") {
		t.Errorf("workspace members not updated:\n%s", ws)
	}
	// the gate: workspace still resolves
	cmd := exec.Command("cargo", "metadata", "--format-version", "1", "--no-deps")
	cmd.Dir = root
	if err := cmd.Run(); err != nil {
		t.Fatal("workspace broken after move")
	}
}

func TestMoverNoRsFilesTouched(t *testing.T) { /* hash all .rs files before/after; equal */ }
func TestMoverRewritesMovedCratesOwnDeps(t *testing.T) { /* move the DEPENDENT crate; its own path = "../../0/…" entries must be recomputed */ }
```

**Step 2:** FAIL.

**Step 3: Implement `Mover`:**

```go
// Mover implements dagnabit.PackageMover for cargo workspaces: git-mv
// the crate directory, then rewrite every relative path-dependency in
// the workspace that the move invalidated (references TO the moved
// crate from dependents, and the moved crate's OWN path-deps whose
// relative base changed), plus the [workspace] members entry. No .rs
// file is touched — crate names do not change in a pure move.
type Mover struct {
	WorkspaceRoot string
}

func (m *Mover) MovePackage(src, dst string) error
```

Algorithm:
1. `git mv src dst` (run in `WorkspaceRoot`; mirror `GitMover.gitMove`).
2. Enumerate every member manifest (workspace root `Cargo.toml` members list via `cargo_workspace` + walk): for each manifest, compute what each relative `path` value resolves to **pre-move**; if it resolved into the old `src` tree, recompute the relative path to the same location under `dst` and apply `cargo_manifest.RewritePathDeps` with the exact old/new strings.
3. For the moved crate's own manifest (now at `dst`): recompute each of its relative path-deps from the new base dir.
4. `cargo_manifest.ReplaceMember` on the root manifest (`src` → `dst`).
5. Gate: run `cargo metadata --format-version 1 --no-deps`; on failure return an error embedding stderr ("workspace no longer resolves after move").

**Step 4:** Tests pass under `nix develop --command` (skip without cargo).

**Step 5:** `just dagnabit-build && just dewey-export-library`; commit:

```bash
git add libs/dewey/internal/echo/dagnabit_rust libs/dewey/pkgs/dagnabit_rust
git commit -m "feat(dewey/dagnabit_rust): cargo workspace crate mover"
```

---

### Task 7: CLI language detection + rust reposition wiring

**Files:**
- Create: `cmd/dagnabit/lang.go`
- Test: `cmd/dagnabit/lang_test.go`
- Modify: `cmd/dagnabit/main.go` (`runReposition`, flag defs)

**Step 1: Failing tests** for `detectLanguage`:

```go
func TestDetectGo(t *testing.T)          // dir with go.mod → langGo
func TestDetectRust(t *testing.T)        // dir under a [workspace] Cargo.toml → langRust
func TestDetectBothErrors(t *testing.T)  // go.mod AND workspace Cargo.toml in same root, no flag → error
func TestDetectNeitherErrors(t *testing.T)
func TestFlagOverridesDetection(t *testing.T) // -lang rust in a go.mod dir → langRust
func TestCrateManifestWithoutWorkspaceIsNotRust(t *testing.T) // [package]-only Cargo.toml keeps walking; go.mod above wins
```

**Step 2:** FAIL.

**Step 3: Implement:**

```go
type language int

const (
	langUnknown language = iota
	langGo
	langRust
)

// detectLanguage resolves the operating language and its root directory.
// flagVal ("", "go", "rust") wins when set; otherwise walk up from dir
// looking for go.mod (→ go) or a Cargo.toml with [workspace] (→ rust).
// Finding both at the same level, or neither anywhere, is an error that
// names the -lang flag.
func detectLanguage(dir, flagVal string) (language, string, error)
```

Walk-up loop checks both markers per level (go.mod via `os.Stat`; Cargo.toml via `cargo_workspace.FindRoot`-style single-level check — add a small exported `HasWorkspaceManifest(dir) (bool, error)` to `cargo_workspace` if cleaner). First level where exactly one marker exists wins; both at one level → error unless flag.

Wire `runReposition`: add `-lang` flag; on `langRust` construct `reader := &cargo_metadata.Reader{…}` and `mover := &dagnabit_rust.Mover{WorkspaceRoot: root}` (import the **pkgs facades**: `libs/dewey/pkgs/cargo_metadata`, `libs/dewey/pkgs/dagnabit_rust`, `libs/dewey/pkgs/cargo_workspace`), same `nato_levels` mapper and `Repositioner`. In rust mode, `-module` errors with `-module is go-only; not valid with -lang rust`.

**Step 4:** `go test ./cmd/dagnabit/` passes; manual smoke (needs cargo): build `just dagnabit-build`, run `build/dagnabit -n -lang rust internal` against a scratch fixture workspace under `.tmp/` — expect dry-run move plan output.

**Step 5: Commit**

```bash
git add cmd/dagnabit/lang.go cmd/dagnabit/lang_test.go cmd/dagnabit/main.go
git commit -m "feat(dagnabit): language auto-detection and rust reposition wiring"
```

---

### Task 8: Rust exporter — glob facade generation

**Files:**
- Create: `libs/dewey/internal/echo/dagnabit_rust/exporter.go`
- Test: `libs/dewey/internal/echo/dagnabit_rust/exporter_test.go`

**Step 1: Failing tests**

```go
func TestExportGeneratesFacadeCrate(t *testing.T) {
	// pure-file test: no cargo needed for generation itself
	root := writeFixtureWorkspaceFiles(t) // files-only variant of the fixture builder
	e := &Exporter{WorkspaceRoot: root, OutputDir: "pkgs"}
	if err := e.ExportPackage("internal/0/blob_store_id"); err != nil {
		t.Fatal(err)
	}
	lib, _ := os.ReadFile(filepath.Join(root, "pkgs/blob_store_id/src/lib.rs"))
	want := "pub use blob_store_id_internal::*;"
	if !strings.Contains(string(lib), want) || !strings.Contains(string(lib), "Code generated by dagnabit; DO NOT EDIT.") {
		t.Errorf("lib.rs:\n%s", lib)
	}
	manifest, _ := os.ReadFile(filepath.Join(root, "pkgs/blob_store_id/Cargo.toml"))
	for _, want := range []string{`name = "blob_store_id"`, `path = "../../internal/0/blob_store_id"`} {
		if !strings.Contains(string(manifest), want) {
			t.Errorf("Cargo.toml missing %q:\n%s", want, manifest)
		}
	}
	ws, _ := os.ReadFile(filepath.Join(root, "Cargo.toml"))
	if !strings.Contains(string(ws), `"pkgs/blob_store_id"`) {
		t.Errorf("facade not added to workspace members")
	}
}

func TestExportRejectsNamingCollision(t *testing.T) {
	// internal crate named plain "blob_store_id" (no _internal suffix)
	// would collide with its facade → hard error mentioning the convention
}

func TestExportAllExportsEverythingUnderInternal(t *testing.T) {}

func TestExportScanHonorsMetadataDirective(t *testing.T) {
	// only crates with [package.metadata.dagnabit] export = true are exported
}

func TestExportCheckDetectsDrift(t *testing.T) {
	// export, then corrupt pkgs/<name>/src/lib.rs, then Check → error naming the file
}

func TestExportDryRunWritesNothing(t *testing.T) {}

func TestGeneratedFacadeCompiles(t *testing.T) {
	requireCargo(t)
	// full fixture, export, then `cargo check --workspace` passes
}
```

**Step 2:** FAIL.

**Step 3: Implement `Exporter`:**

```go
type Exporter struct {
	WorkspaceRoot string
	OutputDir     string // default "pkgs"
	DryRun        bool
}
```

- `ExportPackage(crateDir string)`: read the internal crate's `Cargo.toml` (BurntSushi decode: name, version, edition); facade name = leaf dir of `crateDir`; **collision check**: if internal package name == facade name → error: `internal crate %q must not use its facade name; rename it %q (convention: <name>_internal)`. Generate:
  - `pkgs/<name>/Cargo.toml` from a template: `[package] name/version/edition` + `[dependencies] <internal_name> = { path = "<rel>" }` (rel computed from facade dir to crate dir).
  - `pkgs/<name>/src/lib.rs`: `// Code generated by dagnabit; DO NOT EDIT.\n\npub use <internal_name>::*;\n`.
  - `cargo_manifest.AddMember` for `pkgs/<name>`.
- `ExportAll()`: every dir under `internal/` containing a `Cargo.toml` (any depth — walk; a crate is a dir with Cargo.toml).
- `ScanAndExport()`: only crates whose manifest decodes `[package.metadata.dagnabit] export = true`.
- `Check*` variants: render into `os.MkdirTemp`, byte-compare against the committed `pkgs/<name>` (reuse the readTree/diff idea from the Go exporter at `libs/dewey/internal/echo/dagnabit/exporter.go:172-244`, but Rust facades are exactly two files — keep it simple).
- Formatting: after generation call the existing treefmt discovery — `dagnabit.Exporter.FormatOutput` is a method; extract nothing, just reimplement the small call in dagnabit_rust by invoking the same `FormatOutput` helper IF it is exported via the dewey facade; otherwise call `rustfmt --edition 2021` on generated lib.rs files when rustfmt is on PATH, skipping silently when absent (generated content is already canonical).
  **Decision for implementer:** check whether `pkgs/dagnabit.Exporter.FormatOutput` is reusable standalone; if its receiver state is Go-specific, do the rustfmt fallback — do NOT refactor the Go exporter.

**Step 4:** Unit tests pass everywhere; compile test passes under devshell.

**Step 5:** Facades regen + commit: `feat(dewey/dagnabit_rust): glob-facade exporter (pub use) with metadata directive + check mode`.

---

### Task 9: CLI export wiring (rust)

**Files:**
- Modify: `cmd/dagnabit/main.go` (`runExport`)

**Steps:** add `-lang` resolution at the top of `runExport`; rust branch constructs `dagnabit_rust.Exporter` and dispatches `--library` → `ExportAll`, explicit args → `ExportPackage` each, default → `ScanAndExport`, `--check` → Check variants — mirroring the existing Go dispatch block (`cmd/dagnabit/main.go:296-343`). `--copy` and `--no-rewrite-consumers` in rust mode error: `not supported for rust (see docs/plans/2026-06-06-dagnabit-rust-design.md §3)`. Compile via `hamster.go-build ./cmd/dagnabit`, smoke-test against the scratch fixture (`build/dagnabit export --library` in a fixture workspace), commit `feat(dagnabit): rust export CLI wiring`.

---

### Task 10: Renamer — move/rename with ast-grep rewrites

**Files:**
- Create: `libs/dewey/internal/echo/dagnabit_rust/renamer.go`, `libs/dewey/internal/echo/dagnabit_rust/astgrep.go`
- Test: `libs/dewey/internal/echo/dagnabit_rust/renamer_test.go`
- Modify: `cmd/dagnabit/main.go` (`runMove`, `runRename`)

**Step 1: Failing tests** (all `requireCargo(t)` + `requireAstGrep(t)`)

```go
func TestMoveRenameRewritesDependentSources(t *testing.T) {
	// fixture where store's src/lib.rs has:
	//   use blob_store_id_internal::Id;
	//   pub fn make() -> Id { blob_store_id_internal::make_id() }
	// rename blob_store_id → blob_id (internal name blob_store_id_internal → blob_id_internal)
	// then: dependent sources reference blob_id_internal::…, cargo check --workspace passes
}
func TestMoveRenamePreflightGateAbortsOnBrokenWorkspace(t *testing.T) // broken fixture + no --force → error, tree untouched
func TestMoveRenameForceSkipsPreflight(t *testing.T)
func TestMoveWithoutLeafRenameTouchesNoRs(t *testing.T)               // delegates to Mover; .rs hashes identical
func TestRenameComputesRequiredLevel(t *testing.T)                    // crate with one tier-0 dep → level alfa; mirrors dagnabit rename semantics
func TestDryRunPrintsPlanAndChangesNothing(t *testing.T)
```

**Step 2:** FAIL.

**Step 3: Implement.**

`astgrep.go`:

```go
// renamePatterns is the curated ast-grep pattern set for a crate
// rename. oldLib/newLib are lib-target names (underscored). The set is
// validated by fixture tests, not by enumeration: if cargo check fails
// after a rewrite, the set is incomplete — extend it here.
func renamePatterns(oldLib, newLib string) []patternPair {
	return []patternPair{
		{pattern: oldLib + "::$$$REST", rewrite: newLib + "::$$$REST"}, // paths in exprs, types, and use trees
		{pattern: "use " + oldLib + ";", rewrite: "use " + newLib + ";"},
		{pattern: "extern crate " + oldLib + ";", rewrite: "extern crate " + newLib + ";"},
	}
}

// runAstGrep applies one pattern pair under dir.
// dryRun uses `ast-grep run` without --update-all and returns the match count.
func runAstGrep(dir string, p patternPair, dryRun bool) (matches int, err error)
```

argv: `ast-grep run --lang rust --pattern <p> --rewrite <r> --update-all <dir>` (dry-run drops `--update-all` and counts matches from `--json=compact` output).

`renamer.go` — `MoveRename(src, dst string, opts Options)`:
1. Preflight: `cargo check --workspace` unless `opts.Force` (error embeds stderr).
2. Compute names: old/new leaf; old internal package name from the crate manifest; new internal name preserves the `_internal` suffix convention (`blob_store_id_internal` → `blob_id_internal` when leaf goes `blob_store_id` → `blob_id`). Lib-target names from `cargo metadata` `targets[].name` (underscored) — fall back to package name with `-`→`_`.
3. `Mover.MovePackage(src, dst)` (Cargo.toml path rewrites + members).
4. `cargo_manifest.SetPackageName` on the moved crate; `cargo_manifest.RenameDepKey` in every dependent manifest.
5. ast-grep pattern set over every dependent crate dir (NOT the moved crate — `crate::` self-refs need nothing).
6. Postflight: `cargo check --workspace`; on failure: report rewrite counts + stderr, leave tree dirty (document the clean-git-start recovery in the error text).
7. If the facade `pkgs/<old>` exists, error instructing to re-run `dagnabit export` after the rename (facades are generated; do not chase them in v1).

`Rename(src, newLeaf string, …)`: compute required level from `cargo_metadata` edges + `topological_sort.Sort` heights (mirror `dagnabit.computeRequiredLevel` at `libs/dewey/internal/echo/dagnabit/rename.go:78-170` — reimplement the ~40 relevant lines against the rust reader; do NOT refactor the Go original), then delegate to `MoveRename`.

CLI: `runMove`/`runRename` gain `-lang`; rust branches construct the Renamer; `-module` errors in rust mode.

**Step 4:** Tests pass under devshell; **expect the pattern set to need iteration** — the fixture's `cargo check` postflight is the oracle.

**Step 5:** Facades regen + commit: `feat(dewey/dagnabit_rust): crate move/rename with ast-grep rewrites and cargo check gates`.

---

### Task 11: Nix runtime PATH for ast-grep

**Files:**
- Modify: `gomod.nix` (dagnabit derivation, ~line 149)

**Step 1:** Extend the dagnabit `mkGoModule` call:

```nix
dagnabit = mkGoModule {
  pname = "dagnabit";
  version = "0.1.0";
  subPackages = [ "cmd/dagnabit" ];
  nativeBuildInputs = [ goPkgs.makeWrapper ];
  postInstall = ''
    install -Dm644 $src/cmd/dagnabit/dagnabit.1 $out/share/man/man1/dagnabit.1
    wrapProgram $out/bin/dagnabit \
      --suffix PATH : ${goPkgs.lib.makeBinPath [ goPkgsMaster.ast-grep ]}
  '';
};
```

(Adapt the exact pkgs/pkgs-master variable names to what `gomod.nix` has in scope — check the file header. `--suffix` not `--prefix`: a user-provided ast-grep on PATH wins; the wrapped one is the fallback. `cargo` is intentionally NOT wrapped — runtime PATH expectation, like `go`.)

**Step 2:** Verify `mkGoModule` passes `nativeBuildInputs` through to `buildGoApplication`; if not, thread it (check `gomod.nix`'s mkGoModule definition).

**Step 3:** Run: `nix build .#dagnabit` then `./result/bin/dagnabit --help` and `grep -l ast-grep result/bin/.dagnabit-wrapped` existence check. Expected: build succeeds; wrapper references ast-grep store path.

**Step 4: Commit:** `build(dagnabit): wrap binary with ast-grep on PATH for rust rename rewrites`.

---

### Task 12: BATS integration lane

Use @eng:wiring-bats-tests. Mirror the structure of existing `zz-tests_bats/*.bats`.

**Files:**
- Create: `zz-tests_bats/dagnabit_rust.bats`
- Modify: `justfile` (test recipes block)

**Step 1: Write the BATS file** — `setup()` skips when `cargo` or `ast-grep` missing (`command -v cargo || skip`); builds a fixture workspace in `$BATS_TEST_TMPDIR` (same shape as the Go-test fixture: two internal crates, path dep); resolves dagnabit from `build/dagnabit`. Tests:

```bash
@test "rust reposition dry-run reports planned moves" { … }
@test "rust reposition moves crate and workspace still resolves" { … }
@test "rust export --library generates glob facades that cargo check accepts" { … }
@test "rust export --check detects drift" { … }
@test "rust rename rewrites dependents and cargo check passes" { … }
@test "go mode is unaffected: existing validate_documents fixtures still pass" { … }  # cheap canary: run dagnabit -n on a tiny go fixture
```

**Step 2: justfile recipe** (next to the other test-* leaves; depends on dagnabit-build):

```just
# BATS lane for dagnabit rust mode (reposition/export/rename against cargo fixture workspaces)
[group('test')]
test-dagnabit-rust: dagnabit-build
    {{ cmd_nix_dev }} bats --tap zz-tests_bats/dagnabit_rust.bats
```

Add `test-dagnabit-rust` to the `test` aggregate's dependency list (it skips gracefully without cargo, so the CI gate stays green pre-devshell-reload).

**Step 3:** Run `just test-dagnabit-rust`. Expected: PASS (or all-skip outside devshell).

**Step 4: Commit:** `test(dagnabit): BATS lane for rust mode`.

---

### Task 13: Documentation

**Files:**
- Modify: `cmd/dagnabit/dagnabit.1` — add a LANGUAGES section (auto-detection rules, `-lang` flag), rust notes per subcommand (glob facades, `[package.metadata.dagnabit] export = true` directive, `_internal` convention, ast-grep/cargo runtime requirements, unsupported flags), and rust EXAMPLES.
- Modify: `libs/dewey/CLAUDE.md` — add `*/cargo_workspace`, `*/cargo_manifest`, `*/cargo_metadata`, `*/dagnabit_rust` under **Experimental** (they are new; promotion to battle-tested needs field use).
- Modify: root `CLAUDE.md` — one line in the dagnabit row of Repository Layout mentioning rust mode.

Commit: `docs(dagnabit): document rust mode (manpage, dewey package tiers)`.

---

### Task 14: FDR

Invoke @eng:fdr for "dagnabit rust mode". MUST include:
- **Limitations:** the two-language if/else dispatch (flagged during design — registry/backend refactor is the trigger at language #3); module-level granularity out of scope; `#[macro_export]` coverage status (whatever Task 8's fixture verification found); no consumer-rewrite / no `--copy` in rust mode.
- **Tuning Levers** (from the design doc §Tuning levers): dispatch shape, `_internal` convention, cargo check gate scope, ast-grep pattern set.

Commit per the fdr skill's conventions.

---

## Task dependency order

1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 → 10 → 11 → 12 → 13 → 14

Strictly sequential except: 11 (nix wrap) can land any time after 1; 13 can trail. Each task leaves the tree green (`just` passes) because every rust-needing test skips without the toolchain.
