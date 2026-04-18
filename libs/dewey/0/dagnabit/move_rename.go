package dagnabit

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// MoveOptions controls MovePackageRename behavior.
type MoveOptions struct {
	DryRun          bool
	Verbose         bool
	AllowTypeErrors bool
}

// MovePackageRename relocates a package from src to dst. When the leaf
// directory name differs between src and dst, performs type-aware rewrites
// of the moved package's `package` declaration and of qualified references
// (oldLeaf.X) in callers, in addition to import-path rewrites.
//
// When filepath.Base(src) == filepath.Base(dst), delegates to MovePackage
// (no type-aware analysis required).
//
// Phases when a leaf rename is needed:
//  1. Pre-move: golang.org/x/tools/go/packages.Load computes the rewrite
//     plan. Refuses to proceed if any package has type errors, unless
//     opts.AllowTypeErrors is set.
//  2. Filesystem: git mv src dst.
//  3. Post-move: a single parse/write pass per .go file applies import-path
//     rewrites, import-alias rewrites (only on paths that were rewritten),
//     package-declaration rewrites (for files in the moved package), and
//     qualified-reference rewrites at the positions recorded in phase 1.
//  4. gofmt/goimports.
func (m *GitMover) MovePackageRename(src, dst string, opts MoveOptions) error {
	oldLeaf := filepath.Base(src)
	newLeaf := filepath.Base(dst)

	if oldLeaf == newLeaf {
		if opts.DryRun {
			fmt.Printf("would move: %s -> %s (same leaf, import rewrites only)\n", src, dst)
			return nil
		}

		return m.MovePackage(src, dst)
	}

	plan, err := analyzeLeafRename(m.Dir, m.ModulePath, src, dst, opts.AllowTypeErrors)
	if err != nil {
		return fmt.Errorf("analyzing leaf rename: %w", err)
	}

	if opts.DryRun {
		m.printLeafRenameDryRun(src, dst, plan)
		return nil
	}

	if opts.Verbose {
		nSites := 0
		for _, ss := range plan.CallerSites {
			nSites += len(ss)
		}

		fmt.Fprintf(
			os.Stderr,
			"dagnabit: leaf rename %s -> %s: %d qualified-ref sites in %d callers\n",
			oldLeaf, newLeaf,
			nSites, len(plan.CallerSites),
		)
	}

	if err := m.gitMove(src, dst); err != nil {
		return err
	}

	callerSites := make(map[string][]leafRewriteSite, len(plan.CallerSites))
	for oldAbs, ss := range plan.CallerSites {
		callerSites[remapMovedPath(oldAbs, m.Dir, src, dst)] = ss
	}

	dstAbs, err := filepath.Abs(filepath.Join(m.Dir, dst))
	if err != nil {
		return fmt.Errorf("resolving dst: %w", err)
	}

	err = filepath.Walk(m.Dir, func(path string, info os.FileInfo, werr error) error {
		if werr != nil {
			return werr
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

		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}

		// A file belongs to the moved package when its parent directory
		// is exactly the destination directory. Subpackage files (in
		// dstDir/sub/...) have their own package declaration and are not
		// rewritten. Using the filesystem check instead of packages.Load
		// naturally includes _test.go files that NeedCompiledGoFiles drops.
		isMovedPkgFile := filepath.Dir(abs) == dstAbs

		return rewriteFileForLeafRename(path, plan, callerSites[abs], isMovedPkgFile)
	})
	if err != nil {
		return fmt.Errorf("rewriting files: %w", err)
	}

	if err := m.formatFiles(); err != nil {
		return fmt.Errorf("formatting: %w", err)
	}

	return nil
}

func (m *GitMover) printLeafRenameDryRun(src, dst string, plan *leafRenamePlan) {
	nSites := 0
	for _, ss := range plan.CallerSites {
		nSites += len(ss)
	}

	fmt.Printf(
		"would move: %s -> %s\n  package decl rewrites: %d files\n  qualified-ref rewrites: %d sites in %d caller files\n",
		src, dst,
		len(plan.MovedPkgFiles),
		nSites, len(plan.CallerSites),
	)
}

// rewriteFileForLeafRename parses path once, applies every applicable
// rewrite (import paths, import aliases, package decl, caller qualified
// refs), and writes back if anything changed. A file that is untouched by
// the rename returns with no work.
func rewriteFileForLeafRename(
	path string,
	plan *leafRenamePlan,
	sites []leafRewriteSite,
	isMovedPkgFile bool,
) error {
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

		if importPath != plan.OldPath &&
			!strings.HasPrefix(importPath, plan.OldPath+"/") {
			continue
		}

		suffix := strings.TrimPrefix(importPath, plan.OldPath)
		imp.Path.Value = strconv.Quote(plan.NewPath + suffix)
		changed = true

		if imp.Name != nil && imp.Name.Name == plan.OldLeaf {
			imp.Name.Name = plan.NewLeaf
		}
	}

	if isMovedPkgFile && f.Name != nil {
		switch f.Name.Name {
		case plan.OldLeaf:
			f.Name.Name = plan.NewLeaf
			changed = true
		case plan.OldLeaf + "_test":
			// External test package (package foo_test) — rename to match.
			f.Name.Name = plan.NewLeaf + "_test"
			changed = true
		}

		// Rewrite the godoc-style doc comment attached to the package
		// declaration: "// Package oldLeaf ..." -> "// Package newLeaf ...".
		if f.Doc != nil {
			oldMarker := "// Package " + plan.OldLeaf
			newMarker := "// Package " + plan.NewLeaf

			for _, c := range f.Doc.List {
				if c.Text == oldMarker {
					c.Text = newMarker
					changed = true

					continue
				}

				if strings.HasPrefix(c.Text, oldMarker+" ") {
					c.Text = newMarker + strings.TrimPrefix(c.Text, oldMarker)
					changed = true
				}
			}
		}
	}

	if len(sites) > 0 {
		posSet := make(map[[2]int]bool, len(sites))
		for _, s := range sites {
			posSet[[2]int{s.Line, s.Col}] = true
		}

		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			xIdent, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}

			if xIdent.Name != plan.OldLeaf {
				return true
			}

			pos := fset.Position(xIdent.Pos())
			if posSet[[2]int{pos.Line, pos.Column}] {
				xIdent.Name = plan.NewLeaf
				changed = true
			}

			return true
		})
	}

	if !changed {
		return nil
	}

	return writeFormattedFile(fset, f, path)
}
