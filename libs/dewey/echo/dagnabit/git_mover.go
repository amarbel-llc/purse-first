package dagnabit

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/amarbel-llc/purse-first/libs/dewey/bravo/errors"
)

// GitMover moves packages by running git mv and rewriting Go import paths.
// Dir is the working directory (module root). ModulePath is the Go module
// path used to construct fully-qualified import paths from src/dst.
type GitMover struct {
	Dir        string
	ModulePath string
}

func (m *GitMover) MovePackage(src, dst string) error {
	if err := m.gitMove(src, dst); err != nil {
		return err
	}

	oldPrefix := m.ModulePath + "/" + src
	newPrefix := m.ModulePath + "/" + dst

	if err := m.rewriteImports(oldPrefix, newPrefix); err != nil {
		return fmt.Errorf("rewriting imports: %w", err)
	}

	if err := m.formatFiles(); err != nil {
		return fmt.Errorf("formatting: %w", err)
	}

	return nil
}

func (m *GitMover) gitMove(src, dst string) error {
	dstAbs := filepath.Join(m.Dir, dst)
	parentDir := filepath.Dir(dstAbs)

	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", parentDir, err)
	}

	cmd := exec.Command("git", "mv", src, dst)
	cmd.Dir = m.Dir
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git mv %s %s: %w", src, dst, err)
	}

	return nil
}

func (m *GitMover) rewriteImports(oldPrefix, newPrefix string) error {
	return filepath.Walk(m.Dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "vendor" || base == "node_modules" {
				return filepath.SkipDir
			}

			return nil
		}

		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		return rewriteFileImports(path, oldPrefix, newPrefix)
	})
}

func rewriteFileImports(path, oldPrefix, newPrefix string) error {
	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}

	changed := false

	for _, imp := range f.Imports {
		importPath, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}

		if importPath == oldPrefix || strings.HasPrefix(importPath, oldPrefix+"/") {
			suffix := strings.TrimPrefix(importPath, oldPrefix)
			imp.Path.Value = strconv.Quote(newPrefix + suffix)
			changed = true
		}
	}

	if !changed {
		return nil
	}

	// Also update any named imports whose alias matches the old package name,
	// but only if the package base name actually changed.
	oldBase := filepath.Base(oldPrefix)
	newBase := filepath.Base(newPrefix)

	if oldBase != newBase {
		for _, imp := range f.Imports {
			if imp.Name != nil && imp.Name.Name == oldBase {
				imp.Name.Name = newBase
			}
		}
	}

	return writeFormattedFile(fset, f, path)
}

func writeFormattedFile(fset *token.FileSet, f *ast.File, path string) (err error) {
	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("opening %s for writing: %w", path, err)
	}
	defer errors.DeferredCloser(&err, out)

	if err := format.Node(out, fset, f); err != nil {
		return fmt.Errorf("formatting %s: %w", path, err)
	}

	return nil
}

func (m *GitMover) formatFiles() error {
	if goimportsPath, err := exec.LookPath("goimports"); err == nil {
		cmd := exec.Command(goimportsPath, "-w", ".")
		cmd.Dir = m.Dir
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("goimports: %w", err)
		}

		return nil
	}

	cmd := exec.Command("gofmt", "-w", ".")
	cmd.Dir = m.Dir
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gofmt: %w", err)
	}

	return nil
}
