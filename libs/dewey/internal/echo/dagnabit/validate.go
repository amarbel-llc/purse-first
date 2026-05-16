package dagnabit

import (
	"fmt"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// ValidateUniqueLeaves loads every package in the module and reports an
// error if two `<branch>/<leaf>` packages share a leaf name. Packages with
// fewer than two path components (e.g., a package directly at module root)
// are ignored.
//
// dewey's convention is that leaf names are unique across the entire tree
// so docs and tooling can refer to packages as `*/<leaf>` without pinning
// a level that may change as dependencies evolve.
func ValidateUniqueLeaves(dir, modulePath string) error {
	pkgs, err := loadInModulePackages(dir, modulePath)
	if err != nil {
		return err
	}

	return validateUniqueLeavesPkgs(pkgs, modulePath)
}

func validateUniqueLeavesPkgs(pkgs []*packages.Package, modulePath string) error {
	// leaf name -> set of `<branch>/<leaf>` paths sharing it.
	byLeaf := make(map[string]map[string]struct{})

	for _, p := range pkgs {
		node, _, leaf, ok := branchLeafNode(p.PkgPath, modulePath)
		if !ok {
			continue
		}

		nodes := byLeaf[leaf]
		if nodes == nil {
			nodes = make(map[string]struct{})
			byLeaf[leaf] = nodes
		}

		nodes[node] = struct{}{}
	}

	var collisions [][]string

	for leaf, nodes := range byLeaf {
		if len(nodes) < 2 {
			continue
		}

		paths := make([]string, 0, len(nodes))
		for n := range nodes {
			paths = append(paths, n)
		}

		sort.Strings(paths)

		collisions = append(collisions, append([]string{leaf}, paths...))
	}

	if len(collisions) == 0 {
		return nil
	}

	sort.Slice(collisions, func(i, j int) bool {
		return collisions[i][0] < collisions[j][0]
	})

	var b strings.Builder
	fmt.Fprintf(&b, "leaf name collisions detected (each leaf must be unique across the tree):\n")

	for _, c := range collisions {
		fmt.Fprintf(&b, "  %s: %s\n", c[0], strings.Join(c[1:], ", "))
	}

	return fmt.Errorf("%s", b.String())
}

// ValidateTwoLayerLayout loads every package in the module and reports an
// error if any package's path within the module has more than two
// components (e.g., `<branch>/<leaf>/<sub>` instead of `<branch>/<leaf>`).
//
// dagnabit's reposition machinery assumes a strict <branch>/<leaf> layout.
// Sub-packages break that assumption: the level-mapping math treats every
// path under `<branch>/<leaf>/` as the same node, so a sub-package's deps
// silently get attributed to its parent leaf and the level computation
// drifts. Forcing flatness keeps the model honest.
//
// Packages with fewer than two components (e.g., at the module root) are
// ignored — Go's `testdata/` directories never appear here because
// `packages.Load("./...")` skips them by convention.
func ValidateTwoLayerLayout(dir, modulePath string) error {
	pkgs, err := loadInModulePackages(dir, modulePath)
	if err != nil {
		return err
	}

	return validateTwoLayerLayoutPkgs(pkgs, modulePath)
}

func validateTwoLayerLayoutPkgs(pkgs []*packages.Package, modulePath string) error {
	var violations []string

	for _, p := range pkgs {
		if !strings.HasPrefix(p.PkgPath, modulePath+"/") {
			continue
		}

		rel := strings.TrimPrefix(p.PkgPath, modulePath+"/")

		if strings.Count(rel, "/") < 2 {
			continue
		}

		violations = append(violations, rel)
	}

	if len(violations) == 0 {
		return nil
	}

	sort.Strings(violations)

	var b strings.Builder
	fmt.Fprintf(&b, "layout violations (dagnabit requires <branch>/<leaf>; sub-packages are not allowed):\n")

	for _, v := range violations {
		fmt.Fprintf(&b, "  %s\n", v)
	}

	return fmt.Errorf("%s", b.String())
}
