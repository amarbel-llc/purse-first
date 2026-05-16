package dagnabit

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/tools/go/packages"
)

// branchLeafNode extracts the first two path components of pkgPath after
// stripping modulePath. Returns the joined "<branch>/<leaf>" node, the
// individual parts, and ok=true; or zero values and ok=false when pkgPath
// is outside modulePath or has fewer than two components (e.g., a package
// directly at module root).
func branchLeafNode(pkgPath, modulePath string) (node, branch, leaf string, ok bool) {
	if !strings.HasPrefix(pkgPath, modulePath+"/") {
		return "", "", "", false
	}

	rel := strings.TrimPrefix(pkgPath, modulePath+"/")
	parts := strings.SplitN(rel, "/", 3)

	if len(parts) < 2 {
		return "", "", "", false
	}

	return parts[0] + "/" + parts[1], parts[0], parts[1], true
}

// loadInModulePackages runs `packages.Load("./...")` in dir with the
// minimal config validators need (NeedName + NeedFiles) and returns only
// packages whose PkgPath is rooted at modulePath and that have at least
// one Go file. Sub-packages are kept — callers that require strict
// <branch>/<leaf> layouts should filter via branchLeafNode.
func loadInModulePackages(dir, modulePath string) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Dir: dir,
		Mode: packages.NeedName |
			packages.NeedFiles,
		Tests: false,
		Env:   os.Environ(),
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("packages.Load: %w", err)
	}

	// In-place filter — reuses pkgs' backing array since packages.Load
	// returns a fresh slice on every call. If a future caller ever
	// pre-allocates and passes a slice through this helper, that
	// contract changes.
	out := pkgs[:0]
	for _, p := range pkgs {
		if len(p.GoFiles) == 0 {
			continue
		}

		if !strings.HasPrefix(p.PkgPath, modulePath+"/") {
			continue
		}

		out = append(out, p)
	}

	return out, nil
}

// walkSkipDirs lists directory base names that filepath.Walk callbacks
// should always SkipDir on. Centralized so dagnabit's three walkers
// (move, rewrite-consumers, post-move rewrite) treat `result/` (Nix
// build outputs), `.direnv/`, etc. consistently — past divergence here
// was a latent bug where a Nix `result/` symlink would lead a walker
// into /nix/store.
var walkSkipDirs = map[string]struct{}{
	".git":         {},
	".direnv":      {},
	".tmp":         {},
	"build":        {},
	"node_modules": {},
	"result":       {},
	"testdata":     {},
	"vendor":       {},
}

// shouldSkipWalkDir returns true for directory base names dagnabit's
// filepath.Walk callbacks should always skip.
func shouldSkipWalkDir(base string) bool {
	_, ok := walkSkipDirs[base]
	return ok
}
