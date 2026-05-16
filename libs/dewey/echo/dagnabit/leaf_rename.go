package dagnabit

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// leafRewriteSite locates a SelectorExpr.X identifier whose Name must change
// from OldLeaf to NewLeaf. Position is taken from the pre-move FileSet; since
// file content does not change across a git mv, Line and Column remain stable
// when the file is reparsed post-move.
type leafRewriteSite struct {
	Line int
	Col  int
}

// leafRenamePlan captures the full rewrite plan produced by a pre-move
// analysis using golang.org/x/tools/go/packages.
//
// All file paths are absolute and reflect the PRE-move filesystem. Callers of
// MovePackageRename remap these to post-move paths after git mv.
type leafRenamePlan struct {
	OldLeaf       string
	NewLeaf       string
	OldPath       string
	NewPath       string
	MovedPkgFiles []string
	CallerSites   map[string][]leafRewriteSite
}

// analyzeLeafRename runs the type-aware analysis for a leaf-renaming move.
// MUST be called before git mv while the module's old state is still valid.
//
// When allowTypeErrors is false (default), any package load errors in the
// module abort the analysis with an error message listing up to five errors.
// Callers may pass --force (or MoveOptions{AllowTypeErrors: true}) to
// proceed anyway.
func analyzeLeafRename(
	dir, modulePath, src, dst string,
	allowTypeErrors bool,
) (*leafRenamePlan, error) {
	oldPath := modulePath + "/" + src
	newPath := modulePath + "/" + dst
	oldLeaf := filepath.Base(src)
	newLeaf := filepath.Base(dst)

	cfg := &packages.Config{
		Dir: dir,
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedDeps |
			packages.NeedImports,
		// Include test variants so qualified refs in _test.go files
		// (both internal `package foo` tests and external
		// `package foo_test` tests) are found and rewritten.
		Tests: true,
		Env:   os.Environ(),
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("packages.Load: %w", err)
	}

	if !allowTypeErrors {
		var errs []string

		packages.Visit(pkgs, nil, func(p *packages.Package) {
			for _, e := range p.Errors {
				if len(errs) >= 5 {
					return
				}

				errs = append(errs, fmt.Sprintf("%s: %s", p.PkgPath, e.Msg))
			}
		})

		if len(errs) > 0 {
			return nil, fmt.Errorf(
				"package load errors (pass --force to proceed anyway):\n  %s",
				strings.Join(errs, "\n  "),
			)
		}
	}

	plan := &leafRenamePlan{
		OldLeaf:     oldLeaf,
		NewLeaf:     newLeaf,
		OldPath:     oldPath,
		NewPath:     newPath,
		CallerSites: make(map[string][]leafRewriteSite),
	}

	var moved *packages.Package

	packages.Visit(pkgs, nil, func(p *packages.Package) {
		if p.PkgPath == oldPath {
			moved = p
		}
	})

	if moved == nil {
		return nil, fmt.Errorf(
			"moved package %q not found in module (looked under %q)",
			oldPath,
			dir,
		)
	}

	// Enumerate .go files directly under the source directory, including
	// test files (which packages.Load's NeedCompiledGoFiles mode drops).
	srcAbs, err := filepath.Abs(filepath.Join(dir, src))
	if err != nil {
		return nil, fmt.Errorf("resolving src dir: %w", err)
	}

	entries, err := os.ReadDir(srcAbs)
	if err != nil {
		return nil, fmt.Errorf("reading src dir %s: %w", srcAbs, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}

		plan.MovedPkgFiles = append(
			plan.MovedPkgFiles,
			filepath.Join(srcAbs, entry.Name()),
		)
	}

	fset := moved.Fset

	packages.Visit(pkgs, nil, func(p *packages.Package) {
		if p.PkgPath == oldPath {
			return
		}

		if _, ok := p.Imports[oldPath]; !ok {
			return
		}

		if p.TypesInfo == nil {
			return
		}

		for _, file := range p.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}

				xIdent, ok := sel.X.(*ast.Ident)
				if !ok {
					return true
				}

				// Early filter: an aliased import uses a different local name
				// than the path leaf. Only unaliased usage of `oldLeaf.X`
				// requires renaming; aliases are preserved as-is.
				if xIdent.Name != oldLeaf {
					return true
				}

				use := p.TypesInfo.Uses[xIdent]
				if use == nil {
					return true
				}

				pkgName, ok := use.(*types.PkgName)
				if !ok {
					return true
				}

				if pkgName.Imported().Path() != oldPath {
					return true
				}

				pos := fset.Position(xIdent.Pos())

				abs, absErr := filepath.Abs(pos.Filename)
				if absErr != nil {
					abs = pos.Filename
				}

				plan.CallerSites[abs] = append(
					plan.CallerSites[abs],
					leafRewriteSite{Line: pos.Line, Col: pos.Column},
				)

				return true
			})
		}
	})

	return plan, nil
}

// remapMovedPath translates an absolute path that was inside <dir>/<src>
// before the move to its new location under <dir>/<dst>. Paths outside the
// moved subtree are returned unchanged.
func remapMovedPath(oldAbs, dir, src, dst string) string {
	srcAbs, err := filepath.Abs(filepath.Join(dir, src))
	if err != nil {
		return oldAbs
	}

	rel, err := filepath.Rel(srcAbs, oldAbs)
	if err != nil {
		return oldAbs
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return oldAbs
	}

	dstAbs, err := filepath.Abs(filepath.Join(dir, dst))
	if err != nil {
		return oldAbs
	}

	return filepath.Join(dstAbs, rel)
}
