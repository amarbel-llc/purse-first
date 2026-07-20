package dagnabit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"code.linenisgreat.com/purse-first/libs/dewey/internal/alfa/test_ui"
)

// initModuleRepo creates a git-initialized module at t.TempDir() with a local
// go.work shadowing any outer workspace (nix-shell TMPDIR may be nested in a
// worktree with its own go.work).
func initModuleRepo(t test_ui.T) string {
	t.Helper()

	dir := initTestRepo(t)

	writeFile(t, dir, "go.mod", "module example.com/m\n\ngo 1.21\n")
	writeFile(t, dir, "go.work", "go 1.21\n\nuse .\n")

	return dir
}

// writeLeafRenameFixture populates dir with a module shaped like:
//
//	go.mod, go.work
//	internal/foo/foo.go                  package foo; func X() int { return 1 }
//	cmd/plain/main.go                    uses foo.X() unaliased
//	cmd/shadowed/main.go                 local `foo := 99`, separate call foo.X()
//	cmd/aliased/main.go                  import bar "...foo"; bar.X()
//	cmd/dotted/main.go                   . "...foo"; X() bare
func writeLeafRenameFixture(t test_ui.T, dir string) {
	t.Helper()

	writeFile(
		t,
		dir,
		"internal/foo/foo.go",
		"package foo\n\nfunc X() int { return 1 }\n",
	)

	writeFile(
		t,
		dir,
		"cmd/plain/main.go",
		`package main

import (
	"fmt"

	"example.com/m/internal/foo"
)

func main() {
	fmt.Println(foo.X())
}
`,
	)

	writeFile(
		t,
		dir,
		"cmd/shadowed/main.go",
		`package main

import (
	"fmt"

	"example.com/m/internal/foo"
)

func local() int {
	foo := 99
	return foo
}

func main() {
	fmt.Println(local(), foo.X())
}
`,
	)

	writeFile(
		t,
		dir,
		"cmd/aliased/main.go",
		`package main

import (
	"fmt"

	bar "example.com/m/internal/foo"
)

func main() {
	fmt.Println(bar.X())
}
`,
	)

	writeFile(
		t,
		dir,
		"cmd/dotted/main.go",
		`package main

import (
	"fmt"

	. "example.com/m/internal/foo"
)

func main() {
	fmt.Println(X())
}
`,
	)
}

func goBuildAll(t test_ui.T, dir string) {
	t.Helper()

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./... failed in %s: %v\n%s", dir, err, out)
	}
}

func TestMovePackageRenameFullLeafRenameAllScenarios(t *testing.T) {
	tt := test_ui.T{T: t}
	dir := initModuleRepo(tt)
	writeLeafRenameFixture(tt, dir)
	commitAll(tt, dir, "initial")

	goBuildAll(tt, dir) // sanity: baseline compiles

	mover := &GitMover{Dir: dir, ModulePath: "example.com/m"}

	err := mover.MovePackageRename(
		"internal/foo",
		"internal/bar",
		MoveOptions{},
	)
	if err != nil {
		t.Fatalf("MovePackageRename: %v", err)
	}

	// Package declaration rewritten.
	fooContent := readFile(tt, dir, "internal/bar/foo.go")
	if !strings.Contains(fooContent, "package bar") {
		t.Errorf("expected moved package to declare `package bar`, got:\n%s", fooContent)
	}

	if strings.Contains(fooContent, "package foo") {
		t.Errorf("moved package should not contain `package foo`, got:\n%s", fooContent)
	}

	// Plain caller: import rewritten AND qualified ref rewritten.
	plain := readFile(tt, dir, "cmd/plain/main.go")
	if !strings.Contains(plain, `"example.com/m/internal/bar"`) {
		t.Errorf("plain caller: expected new import, got:\n%s", plain)
	}

	if !strings.Contains(plain, "bar.X()") {
		t.Errorf("plain caller: expected bar.X() rewrite, got:\n%s", plain)
	}

	if strings.Contains(plain, "foo.X()") {
		t.Errorf("plain caller: foo.X() should have been rewritten, got:\n%s", plain)
	}

	// Shadowed caller: package use rewritten but local var `foo` untouched.
	shadowed := readFile(tt, dir, "cmd/shadowed/main.go")
	if !strings.Contains(shadowed, `"example.com/m/internal/bar"`) {
		t.Errorf("shadowed: expected new import, got:\n%s", shadowed)
	}

	if !strings.Contains(shadowed, "bar.X()") {
		t.Errorf("shadowed: expected bar.X() rewrite, got:\n%s", shadowed)
	}

	if !strings.Contains(shadowed, "foo := 99") {
		t.Errorf("shadowed: local `foo := 99` must be preserved, got:\n%s", shadowed)
	}

	// Aliased caller: import path rewrites but identifier stays `bar`.
	aliased := readFile(tt, dir, "cmd/aliased/main.go")
	if !strings.Contains(aliased, `bar "example.com/m/internal/bar"`) {
		t.Errorf("aliased: expected bar alias preserved with new path, got:\n%s", aliased)
	}

	if !strings.Contains(aliased, "bar.X()") {
		t.Errorf("aliased: bar.X() must be preserved, got:\n%s", aliased)
	}

	// Dotted caller: import path rewrites, no selector change.
	dotted := readFile(tt, dir, "cmd/dotted/main.go")
	if !strings.Contains(dotted, `. "example.com/m/internal/bar"`) {
		t.Errorf("dotted: expected dot import with new path, got:\n%s", dotted)
	}

	if !strings.Contains(dotted, "fmt.Println(X())") {
		t.Errorf("dotted: bare X() should be preserved, got:\n%s", dotted)
	}

	// Compilation guarantee.
	goBuildAll(tt, dir)
}

func TestMovePackageRenameSameLeafDelegatesToMovePackage(t *testing.T) {
	tt := test_ui.T{T: t}
	dir := initModuleRepo(tt)

	writeFile(
		tt,
		dir,
		"internal/alfa/foo/foo.go",
		"package foo\n\nfunc X() int { return 1 }\n",
	)
	writeFile(
		tt,
		dir,
		"cmd/main.go",
		`package main

import (
	"fmt"

	"example.com/m/internal/alfa/foo"
)

func main() {
	fmt.Println(foo.X())
}
`,
	)

	commitAll(tt, dir, "initial")

	mover := &GitMover{Dir: dir, ModulePath: "example.com/m"}

	err := mover.MovePackageRename(
		"internal/alfa/foo",
		"internal/bravo/foo",
		MoveOptions{},
	)
	if err != nil {
		t.Fatalf("MovePackageRename: %v", err)
	}

	content := readFile(tt, dir, "cmd/main.go")
	if !strings.Contains(content, `"example.com/m/internal/bravo/foo"`) {
		t.Errorf("expected rewritten import, got:\n%s", content)
	}

	if !strings.Contains(content, "foo.X()") {
		t.Errorf("foo.X() should be preserved (same leaf), got:\n%s", content)
	}

	goBuildAll(tt, dir)
}

func TestMovePackageRenameDryRunMakesNoChanges(t *testing.T) {
	tt := test_ui.T{T: t}
	dir := initModuleRepo(tt)
	writeLeafRenameFixture(tt, dir)
	commitAll(tt, dir, "initial")

	mover := &GitMover{Dir: dir, ModulePath: "example.com/m"}

	err := mover.MovePackageRename(
		"internal/foo",
		"internal/bar",
		MoveOptions{DryRun: true},
	)
	if err != nil {
		t.Fatalf("MovePackageRename dry-run: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "internal/bar")); !os.IsNotExist(err) {
		t.Errorf("dry-run must not create internal/bar (err: %v)", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "internal/foo/foo.go")); err != nil {
		t.Errorf("dry-run must preserve internal/foo/foo.go: %v", err)
	}

	plain := readFile(tt, dir, "cmd/plain/main.go")
	if !strings.Contains(plain, "foo.X()") {
		t.Errorf("dry-run must preserve foo.X(), got:\n%s", plain)
	}
}

func TestMovePackageRenameHandlesTestFiles(t *testing.T) {
	tt := test_ui.T{T: t}
	dir := initModuleRepo(tt)

	writeFile(
		tt,
		dir,
		"internal/foo/foo.go",
		"package foo\n\nfunc X() int { return 1 }\n",
	)

	// Internal test file (package foo): sees unexported symbols via the
	// same package name.
	writeFile(
		tt,
		dir,
		"internal/foo/internal_test.go",
		`package foo

import "testing"

func TestInternal(t *testing.T) {
	if X() != 1 {
		t.Fatal("bad")
	}
}
`,
	)

	// External test file (package foo_test): imports the package and uses
	// qualified refs.
	writeFile(
		tt,
		dir,
		"internal/foo/external_test.go",
		`package foo_test

import (
	"testing"

	"example.com/m/internal/foo"
)

func TestExternal(t *testing.T) {
	if foo.X() != 1 {
		t.Fatal("bad")
	}
}
`,
	)

	commitAll(tt, dir, "initial")

	mover := &GitMover{Dir: dir, ModulePath: "example.com/m"}

	err := mover.MovePackageRename(
		"internal/foo",
		"internal/bar",
		MoveOptions{},
	)
	if err != nil {
		t.Fatalf("MovePackageRename: %v", err)
	}

	internalTest := readFile(tt, dir, "internal/bar/internal_test.go")
	if !strings.Contains(internalTest, "package bar") {
		t.Errorf("internal test: expected `package bar`, got:\n%s", internalTest)
	}

	if strings.Contains(internalTest, "package foo") {
		t.Errorf("internal test: `package foo` should be rewritten, got:\n%s", internalTest)
	}

	externalTest := readFile(tt, dir, "internal/bar/external_test.go")
	if !strings.Contains(externalTest, "package bar_test") {
		t.Errorf("external test: expected `package bar_test`, got:\n%s", externalTest)
	}

	if !strings.Contains(externalTest, `"example.com/m/internal/bar"`) {
		t.Errorf("external test: expected new import, got:\n%s", externalTest)
	}

	if !strings.Contains(externalTest, "bar.X()") {
		t.Errorf("external test: qualified ref should be rewritten, got:\n%s", externalTest)
	}

	// Compilation guarantee (go test compiles the test binary).
	cmd := exec.Command("go", "test", "-run", "^$", "./...")
	cmd.Dir = dir

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go test -run ^$ ./... failed: %v\n%s", err, out)
	}
}

func TestMovePackageRenameTypeErrorsAbortWithoutForce(t *testing.T) {
	tt := test_ui.T{T: t}
	dir := initModuleRepo(tt)

	// Create a package with a deliberate type error that does not involve
	// the moved package — analyzeLeafRename should still refuse.
	writeFile(
		tt,
		dir,
		"internal/foo/foo.go",
		"package foo\n\nfunc X() int { return 1 }\n",
	)
	writeFile(
		tt,
		dir,
		"cmd/broken/main.go",
		"package main\n\nfunc main() { undefinedSymbol() }\n",
	)
	commitAll(tt, dir, "initial")

	mover := &GitMover{Dir: dir, ModulePath: "example.com/m"}

	err := mover.MovePackageRename(
		"internal/foo",
		"internal/bar",
		MoveOptions{},
	)
	if err == nil {
		t.Fatal("expected error due to type errors in module, got nil")
	}

	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("expected error to hint at --force, got %q", err.Error())
	}

	// Still intact.
	if _, err := os.Stat(filepath.Join(dir, "internal/foo/foo.go")); err != nil {
		t.Errorf("foo.go should still exist after aborted move: %v", err)
	}
}
