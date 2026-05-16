package dagnabit

import (
	"fmt"
	"os"
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
	cfg := &packages.Config{
		Dir: dir,
		Mode: packages.NeedName |
			packages.NeedFiles,
		Tests: false,
		Env:   os.Environ(),
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return fmt.Errorf("packages.Load: %w", err)
	}

	// leaf name -> sorted list of `<branch>/<leaf>` paths sharing it.
	byLeaf := make(map[string][]string)

	for _, p := range pkgs {
		if len(p.GoFiles) == 0 {
			continue
		}

		if !strings.HasPrefix(p.PkgPath, modulePath+"/") {
			continue
		}

		rel := strings.TrimPrefix(p.PkgPath, modulePath+"/")
		parts := strings.SplitN(rel, "/", 3)

		if len(parts) < 2 {
			continue
		}

		node := parts[0] + "/" + parts[1]
		leaf := parts[1]

		byLeaf[leaf] = append(byLeaf[leaf], node)
	}

	var collisions [][]string

	for leaf, nodes := range byLeaf {
		uniq := make(map[string]bool, len(nodes))
		for _, n := range nodes {
			uniq[n] = true
		}

		if len(uniq) < 2 {
			continue
		}

		paths := make([]string, 0, len(uniq))
		for n := range uniq {
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
	cfg := &packages.Config{
		Dir: dir,
		Mode: packages.NeedName |
			packages.NeedFiles,
		Tests: false,
		Env:   os.Environ(),
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return fmt.Errorf("packages.Load: %w", err)
	}

	var violations []string

	for _, p := range pkgs {
		if len(p.GoFiles) == 0 {
			continue
		}

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
