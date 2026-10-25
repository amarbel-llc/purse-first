package dagnabit

import (
	"os/exec"
	"strings"
	"testing"

	"code.linenisgreat.com/purse-first/libs/dewey/internal/alfa/test_ui"
)

// purse-first#220: spinclass's level-by-level move reported three
// importers left on the old path. Each shape it named is pinned here
// against the CLI's move entry point (MovePackageRename with an unchanged
// leaf), over a sequence of per-level moves like spinclass's.
func TestGitMoverRewritesReportedImporterShapes(t *testing.T) {
	tt := test_ui.T{T: t}
	dir := initTestRepo(tt)

	writeFile(tt, dir, "go.mod", "module example.com/m\n\ngo 1.21\n")
	writeFile(tt, dir, "go.work", "go 1.21\n\nuse .\n")
	// A package named like the close builtin.
	writeFile(tt, dir, "internal/close/close.go", "package close\n\nfunc Run() string { return \"close\" }\n")
	writeFile(tt, dir, "internal/merge/merge.go", `package merge

import "example.com/m/internal/close"

func Run() string { return close.Run() }
`)
	writeFile(tt, dir, "internal/resurrect/resurrect.go", `package resurrect

import "example.com/m/internal/merge"

func Run() string { return merge.Run() }
`)
	// A _test.go importer in a package that is moved later.
	writeFile(tt, dir, "internal/resurrect/resurrect_test.go", `package resurrect

import (
	"testing"

	"example.com/m/internal/close"
)

func TestRun(t *testing.T) { _ = close.Run() }
`)
	// An aliased importer outside the prefix.
	writeFile(tt, dir, "cmd/app/close_child_cmd.go", `package main

import spinclose "example.com/m/internal/close"

func closeChild() string { return spinclose.Run() }
`)
	// A plain importer of the package moved last.
	writeFile(tt, dir, "cmd/app/resurrect_cmd.go", `package main

import "example.com/m/internal/resurrect"

func main() { _ = resurrect.Run(); _ = closeChild() }
`)
	commitAll(tt, dir, "initial")
	goBuildAll(tt, dir)

	mover := &GitMover{Dir: dir, ModulePath: "example.com/m"}

	for _, mv := range [][2]string{
		{"internal/close", "internal/0/close"},
		{"internal/merge", "internal/alfa/merge"},
		{"internal/resurrect", "internal/bravo/resurrect"},
	} {
		if err := mover.MovePackageRename(mv[0], mv[1], MoveOptions{}); err != nil {
			t.Fatalf("move %s -> %s: %v", mv[0], mv[1], err)
		}
	}

	cases := []struct {
		file, want, stale string
	}{
		{"cmd/app/close_child_cmd.go", `spinclose "example.com/m/internal/0/close"`, `"example.com/m/internal/close"`},
		{"internal/bravo/resurrect/resurrect_test.go", `"example.com/m/internal/0/close"`, `"example.com/m/internal/close"`},
		{"cmd/app/resurrect_cmd.go", `"example.com/m/internal/bravo/resurrect"`, `"example.com/m/internal/resurrect"`},
		{"internal/alfa/merge/merge.go", `"example.com/m/internal/0/close"`, `"example.com/m/internal/close"`},
	}

	for _, c := range cases {
		content := readFile(tt, dir, c.file)

		if !strings.Contains(content, c.want) {
			t.Errorf("%s: expected %s, got:\n%s", c.file, c.want, content)
		}

		if strings.Contains(content, c.stale) {
			t.Errorf("%s: stale import %s remains:\n%s", c.file, c.stale, content)
		}
	}

	// go vet type-checks _test.go files too, which go build skips.
	vet := exec.Command("go", "vet", "./...")
	vet.Dir = dir

	if out, err := vet.CombinedOutput(); err != nil {
		t.Fatalf("go vet ./... after the moves: %v\n%s", err, out)
	}
}
