package dagnabit

import (
	"fmt"
	"os"
	"path/filepath"

	topological_sort "github.com/amarbel-llc/purse-first/libs/dewey/internal/0/topological_sort"
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
	// Normalize src so `./alfa/cmp` and `alfa/cmp` are treated identically.
	// Otherwise the `dst == src` no-op guard below silently misses the
	// `./`-prefixed form and we pay for a redundant MovePackageRename.
	src = filepath.Clean(src)

	if newLeaf == "" {
		newLeaf = filepath.Base(src)
	}

	pkgs, err := loadInModulePackages(m.Dir, m.ModulePath)
	if err != nil {
		return fmt.Errorf("precondition load: %w", err)
	}

	if err := validateTwoLayerLayoutPkgs(pkgs, m.ModulePath); err != nil {
		return fmt.Errorf("precondition failed: %w", err)
	}

	if err := validateUniqueLeavesPkgs(pkgs, m.ModulePath); err != nil {
		return fmt.Errorf("precondition failed: %w", err)
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
		Dir: m.Dir,
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

	srcNode, _, _, ok := branchLeafNode(srcPkg.PkgPath, m.ModulePath)
	if !ok {
		return "", fmt.Errorf(
			"src %q is not a 2-layer package path (<branch>/<leaf>)",
			src,
		)
	}

	// Walk transitively, building the edge set of the constrained subgraph.
	var edges []topological_sort.Edge
	seenEdges := make(map[topological_sort.Edge]bool)
	visited := make(map[string]bool)

	var walk func(p *packages.Package)
	walk = func(p *packages.Package) {
		if visited[p.PkgPath] {
			return
		}
		visited[p.PkgPath] = true

		fromNode, _, _, fromOK := branchLeafNode(p.PkgPath, m.ModulePath)

		for _, dep := range p.Imports {
			walk(dep)

			if !fromOK {
				continue
			}

			toNode, _, _, toOK := branchLeafNode(dep.PkgPath, m.ModulePath)
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

