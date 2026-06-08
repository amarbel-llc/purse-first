package dagnabit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amarbel-llc/purse-first/libs/dewey/internal/alfa/test_ui"
)

func initTestRepo(t test_ui.T) string {
	t.Helper()

	dir := t.TempDir()

	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Stderr = os.Stderr
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	git("init")
	git("config", "user.email", "test@test.com")
	git("config", "user.name", "Test")
	git("config", "commit.gpgSign", "false")

	return dir
}

func writeFile(t test_ui.T, dir, relPath, content string) {
	t.Helper()

	abs := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t test_ui.T, dir, relPath string) string {
	t.Helper()

	abs := filepath.Join(dir, relPath)
	data, err := os.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}

func commitAll(t test_ui.T, dir, msg string) {
	t.Helper()

	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	git("add", "-A")
	git("commit", "-m", msg)
}

func TestGitMoverMovesFiles(t *testing.T) {
	tt := test_ui.T{T: t}
	dir := initTestRepo(tt)

	writeFile(tt, dir, "lib/alfa/pkg/main.go", "package pkg\n")
	commitAll(tt, dir, "initial")

	mover := &GitMover{Dir: dir, ModulePath: "example.com/mod"}

	if err := mover.MovePackage("lib/alfa/pkg", "lib/bravo/pkg"); err != nil {
		t.Fatalf("MovePackage: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "lib/bravo/pkg/main.go")); err != nil {
		t.Errorf("expected file at new location: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "lib/alfa/pkg/main.go")); !os.IsNotExist(err) {
		t.Errorf("expected old location to not exist, got err: %v", err)
	}
}

func TestGitMoverRewritesImports(t *testing.T) {
	tt := test_ui.T{T: t}
	dir := initTestRepo(tt)

	writeFile(tt, dir, "lib/alfa/pkg/pkg.go", "package pkg\n\nfunc Hello() {}\n")
	writeFile(tt, dir, "cmd/main.go", `package main

import "example.com/mod/lib/alfa/pkg"

func main() {
	pkg.Hello()
}
`)
	commitAll(tt, dir, "initial")

	mover := &GitMover{Dir: dir, ModulePath: "example.com/mod"}

	if err := mover.MovePackage("lib/alfa/pkg", "lib/bravo/pkg"); err != nil {
		t.Fatalf("MovePackage: %v", err)
	}

	content := readFile(tt, dir, "cmd/main.go")
	if !strings.Contains(content, `"example.com/mod/lib/bravo/pkg"`) {
		t.Errorf("expected rewritten import, got:\n%s", content)
	}

	if strings.Contains(content, `"example.com/mod/lib/alfa/pkg"`) {
		t.Errorf("old import should not remain, got:\n%s", content)
	}
}

func TestGitMoverRewritesSubpathImports(t *testing.T) {
	tt := test_ui.T{T: t}
	dir := initTestRepo(tt)

	writeFile(tt, dir, "lib/alfa/pkg/sub/sub.go", "package sub\n\nfunc World() {}\n")
	writeFile(tt, dir, "lib/alfa/pkg/pkg.go", "package pkg\n")
	writeFile(tt, dir, "cmd/main.go", `package main

import "example.com/mod/lib/alfa/pkg/sub"

func main() {
	sub.World()
}
`)
	commitAll(tt, dir, "initial")

	mover := &GitMover{Dir: dir, ModulePath: "example.com/mod"}

	if err := mover.MovePackage("lib/alfa/pkg", "lib/bravo/pkg"); err != nil {
		t.Fatalf("MovePackage: %v", err)
	}

	content := readFile(tt, dir, "cmd/main.go")
	if !strings.Contains(content, `"example.com/mod/lib/bravo/pkg/sub"`) {
		t.Errorf("expected subpath import rewritten, got:\n%s", content)
	}
}

func TestGitMoverSkipsUnrelatedImports(t *testing.T) {
	tt := test_ui.T{T: t}
	dir := initTestRepo(tt)

	writeFile(tt, dir, "lib/alfa/pkg/pkg.go", "package pkg\n")
	writeFile(tt, dir, "cmd/main.go", `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`)
	commitAll(tt, dir, "initial")

	mover := &GitMover{Dir: dir, ModulePath: "example.com/mod"}

	if err := mover.MovePackage("lib/alfa/pkg", "lib/bravo/pkg"); err != nil {
		t.Fatalf("MovePackage: %v", err)
	}

	content := readFile(tt, dir, "cmd/main.go")
	if !strings.Contains(content, `"fmt"`) {
		t.Errorf("unrelated import should be preserved, got:\n%s", content)
	}
}

func TestGitMoverDestinationFileConflict(t *testing.T) {
	tt := test_ui.T{T: t}
	dir := initTestRepo(tt)

	writeFile(tt, dir, "lib/alfa/pkg/pkg.go", "package pkg\n")
	// Create a file at the exact destination path so git mv cannot
	// create the directory — this forces a conflict.
	writeFile(tt, dir, "lib/bravo/pkg", "not a directory\n")
	commitAll(tt, dir, "initial")

	mover := &GitMover{Dir: dir, ModulePath: "example.com/mod"}

	err := mover.MovePackage("lib/alfa/pkg", "lib/bravo/pkg")
	if err == nil {
		t.Fatal("expected error when destination path is a file, got nil")
	}
}

func TestGitMoverMultipleImportsInOneFile(t *testing.T) {
	tt := test_ui.T{T: t}
	dir := initTestRepo(tt)

	writeFile(tt, dir, "lib/alfa/pkg/pkg.go", "package pkg\n\nfunc Hello() {}\n")
	writeFile(tt, dir, "lib/alfa/pkg/sub/sub.go", "package sub\n\nfunc World() {}\n")
	writeFile(tt, dir, "cmd/main.go", `package main

import (
	"example.com/mod/lib/alfa/pkg"
	"example.com/mod/lib/alfa/pkg/sub"
)

func main() {
	pkg.Hello()
	sub.World()
}
`)
	commitAll(tt, dir, "initial")

	mover := &GitMover{Dir: dir, ModulePath: "example.com/mod"}

	if err := mover.MovePackage("lib/alfa/pkg", "lib/bravo/pkg"); err != nil {
		t.Fatalf("MovePackage: %v", err)
	}

	content := readFile(tt, dir, "cmd/main.go")
	if !strings.Contains(content, `"example.com/mod/lib/bravo/pkg"`) {
		t.Errorf("expected rewritten import, got:\n%s", content)
	}

	if !strings.Contains(content, `"example.com/mod/lib/bravo/pkg/sub"`) {
		t.Errorf("expected subpath import rewritten, got:\n%s", content)
	}
}
