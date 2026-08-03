package dagnabit

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// enumerate returns the sorted import paths of every in-module package that
// BUILDS for arch and is eligible for init-smoke coverage: not a main package,
// importable (has non-test .go files for the arch), not the init-smoke package
// itself, and not in the arch's skip list.
//
// Buildability is determined by type-checking under the arch (GOOS/GOARCH), not
// by `go list` alone: an init hazard's siblings — a package that references a
// symbol undefined for the arch (purse-first#172/#173) or imports one that
// does — compiles clean under `go list` (which only resolves the import graph)
// and fails only at type-check/compile. Type errors populate a package's Errors
// and propagate to its importers, so an importer of an unbuildable package is
// itself excluded, giving transitive correctness.
//
// Packages that do NOT build for the arch are auto-excluded (their init() can
// never run there). Detecting a package that SHOULD build for the arch but
// regressed is the wasm-build gate's job (purse-first#174), kept deliberately
// separate. A skip entry matching no in-module package is an error (typo guard).
func (is *InitSmoke) enumerate(arch InitSmokeArch) ([]string, error) {
	pkgs, err := is.loadPackages(arch)
	if err != nil {
		return nil, err
	}

	self := is.initSmokeSelfImport()

	skip := make(map[string]bool, len(arch.Skip))
	for _, s := range arch.Skip {
		skip[path.Join(is.ModulePath, s)] = true
	}
	seenSkip := make(map[string]bool, len(arch.Skip))

	var eligible []string
	for _, p := range pkgs {
		if p.PkgPath != is.ModulePath &&
			!strings.HasPrefix(p.PkgPath, is.ModulePath+"/") {
			continue
		}

		// Mark skip membership before any other filter so a skip entry naming a
		// main / unbuildable / test-only package still counts as "seen" (it
		// exists), rather than tripping the typo guard below.
		if skip[p.PkgPath] {
			seenSkip[p.PkgPath] = true
			continue
		}

		if p.Name == "main" {
			continue
		}
		if p.PkgPath == self {
			continue
		}
		// No buildable non-test files for this arch (all excluded by build
		// constraints, or a test-only package): not importable via a blank
		// import.
		if len(p.GoFiles) == 0 {
			continue
		}
		// Does not type-check for this arch (own compile error like an
		// undefined arch symbol, or an import of a package that fails to build
		// for the arch): auto-excluded. IllTyped propagates transitively — a
		// package importing an ill-typed dependency is itself ill-typed — so an
		// importer of an unbuildable package is excluded too.
		if p.IllTyped || len(p.Errors) > 0 {
			continue
		}

		eligible = append(eligible, p.PkgPath)
	}

	var unknownSkip []string
	for full := range skip {
		if !seenSkip[full] {
			unknownSkip = append(unknownSkip, full)
		}
	}
	if len(unknownSkip) > 0 {
		sort.Strings(unknownSkip)

		return nil, fmt.Errorf(
			"skip entries match no package under %s: %s",
			is.ModulePath, strings.Join(unknownSkip, ", "),
		)
	}

	sort.Strings(eligible)

	return eligible, nil
}

// loadPackages type-checks every package under the module root for arch and
// returns them. NeedTypes (with NeedDeps) is what makes a package that fails to
// compile for the arch — or imports one that does — surface in its Errors, so
// enumerate can exclude it. Load errors on individual packages are reported per
// package (in Errors), not returned here, so the whole graph is still walked.
func (is *InitSmoke) loadPackages(arch InitSmokeArch) ([]*packages.Package, error) {
	base := is.Env
	if base == nil {
		base = os.Environ()
	}
	// Arch overrides go last so they win over any inherited GOOS/GOARCH.
	env := make([]string, 0, len(base)+2)
	env = append(env, base...)
	env = append(env, "GOOS="+arch.GOOS, "GOARCH="+arch.GOARCH)

	cfg := &packages.Config{
		// NeedSyntax is essential: without it NeedTypes loads types from export
		// data (the compiled .a), which does not exist for a package that fails
		// to build for the arch, so its compile error would go unreported.
		// NeedSyntax forces type-checking from source, populating Errors and
		// setting IllTyped on the failing package and its importers.
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedSyntax |
			packages.NeedTypes,
		Dir: is.Dir,
		Env: env,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf(
			"packages.Load ./... (GOOS=%s GOARCH=%s): %w",
			arch.GOOS, arch.GOARCH, err,
		)
	}

	return pkgs, nil
}
