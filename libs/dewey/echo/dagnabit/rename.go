package dagnabit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	topological_sort "github.com/amarbel-llc/purse-first/libs/dewey/0/topological_sort"
	"golang.org/x/tools/go/packages"
)

// RenamePackage moves src to <computed-level>/<newLeaf>, computing the
// required NATO level from a subgraph consisting of src and all the
// packages it transitively imports inside m.ModulePath.
//
// When newLeaf is empty, src's existing leaf is preserved so the operation
// is purely a single-package reposition.
//
// Other packages in the module are NOT moved, even if they are at the wrong
// level. To rebalance the whole tree, use the Repositioner.
//
// The actual move is delegated to MovePackageRename, so type-aware leaf
// renaming, import-path rewriting, and dry-run behavior are unchanged from
// the `move` subcommand.
func (m *GitMover) RenamePackage(
	src, newLeaf string,
	mapper LevelMapper,
	opts MoveOptions,
) error {
	if newLeaf == "" {
		newLeaf = filepath.Base(src)
	}

	requiredLevel, err := m.computeRequiredLevel(src, mapper)
	if err != nil {
		return fmt.Errorf("computing required level for %s: %w", src, err)
	}

	dst := requiredLevel + "/" + newLeaf

	if dst == src {
		if opts.Verbose || opts.DryRun {
			fmt.Fprintf(
				os.Stderr,
				"dagnabit: %s already at level %q with leaf %q; nothing to do\n",
				src, requiredLevel, newLeaf,
			)
		}

		return nil
	}

	return m.MovePackageRename(src, dst, opts)
}

// computeRequiredLevel loads src via packages.Load, walks its transitive
// in-module imports to build a constrained dep subgraph, and runs
// topological_sort to find src's height. The mapper converts that height
// to a level name.
func (m *GitMover) computeRequiredLevel(
	src string,
	mapper LevelMapper,
) (string, error) {
	srcImport := m.ModulePath + "/" + src

	cfg := &packages.Config{
		Dir:   m.Dir,
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedImports |
			packages.NeedDeps,
		Tests: false,
		Env:   os.Environ(),
	}

	pkgs, err := packages.Load(cfg, srcImport)
	if err != nil {
		return "", fmt.Errorf("packages.Load: %w", err)
	}

	if len(pkgs) == 0 {
		return "", fmt.Errorf("no packages found for %s", srcImport)
	}

	srcPkg := pkgs[0]

	if len(srcPkg.GoFiles) == 0 {
		return "", fmt.Errorf(
			"%s does not exist or has no Go files (load errors: %v)",
			src, srcPkg.Errors,
		)
	}

	srcNode, ok := m.nodeFor(srcPkg.PkgPath)
	if !ok {
		return "", fmt.Errorf(
			"src %q is not a 2-layer package path (<branch>/<leaf>)",
			src,
		)
	}

	// Walk transitively. Each in-module package contributes itself to the
	// node set and edges from itself to each of its in-module imports.
	nodes := map[string]bool{srcNode: true}
	var edges []topological_sort.Edge
	seenEdges := make(map[topological_sort.Edge]bool)
	visited := make(map[string]bool)

	var walk func(p *packages.Package)
	walk = func(p *packages.Package) {
		if visited[p.PkgPath] {
			return
		}
		visited[p.PkgPath] = true

		fromNode, fromOK := m.nodeFor(p.PkgPath)
		if fromOK {
			nodes[fromNode] = true
		}

		for _, dep := range p.Imports {
			walk(dep)

			if !fromOK {
				continue
			}

			toNode, toOK := m.nodeFor(dep.PkgPath)
			if !toOK || toNode == fromNode {
				continue
			}

			edge := topological_sort.Edge{Source: fromNode, Target: toNode}
			if seenEdges[edge] {
				continue
			}

			seenEdges[edge] = true
			edges = append(edges, edge)
		}
	}

	walk(srcPkg)

	heights, err := topological_sort.Sort(edges)
	if err != nil {
		return "", fmt.Errorf("topological sort: %w", err)
	}

	height, ok := heights[srcNode]
	if !ok {
		// src has no in-module deps; it sits at height 0.
		height = 0
	}

	return mapper.LevelName(height)
}

// nodeFor extracts the first two path components after stripping the module
// path. Returns "" and false for packages outside the module, or paths with
// fewer than two components (e.g., a package directly at module root).
func (m *GitMover) nodeFor(pkgPath string) (string, bool) {
	if !strings.HasPrefix(pkgPath, m.ModulePath+"/") {
		return "", false
	}

	rel := strings.TrimPrefix(pkgPath, m.ModulePath+"/")
	parts := strings.SplitN(rel, "/", 3)

	if len(parts) < 2 {
		return "", false
	}

	return parts[0] + "/" + parts[1], true
}
