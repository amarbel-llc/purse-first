package dagnabit

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	jen "github.com/dave/jennifer/jen"
	"golang.org/x/tools/go/packages"

	"github.com/amarbel-llc/purse-first/libs/dewey/internal/delta/files"
)

const exportDirective = "//go:generate dagnabit export"

// privateDirective marks a package as excluded from --library mode.
const privateDirective = "//go:generate dagnabit private"

// Exporter generates pkgs/ facade files from internal packages.
type Exporter struct {
	ModulePath string
	Dir        string
	OutputDir  string
	DryRun     bool

	// OutputRoot, when non-empty, is the filesystem root under which facades
	// are written (and formatted), in place of Dir. Package loading and
	// formatter-config lookup still key off Dir; only the output location
	// moves. CheckAll() uses this to render+format into a temp dir for a
	// no-mutation drift comparison against the real Dir/<outputDir>.
	OutputRoot string

	// SkipConsumerRewrite, when true, disables the post-export pass that
	// walks the workspace looking for files outside this module that import
	// the just-exported internal package and rewrites those imports to use
	// the freshly-generated facade path. Default behavior is to do the
	// rewrite — it's the natural follow-on to generating a facade.
	SkipConsumerRewrite bool

	// Copy switches the exporter from facade generation (per-symbol
	// `var = internal.X` aliases via jennifer) to source-tree copying
	// (literal copies of the internal package's .go files into pkgs/,
	// with intra-module imports rewritten to point at pkgs/ facade
	// paths). See purse-first#103 for the motivation.
	//
	// Test files (`*_test.go`) are skipped. Build-tag constraints
	// (`//go:build …`) are preserved as-is on each copied file.
	Copy bool

	// Env, when non-nil, replaces the process environment for go/packages
	// invocations. Useful in tests to set GOWORK=off.
	Env []string
}

func (exporter *Exporter) outputDir() string {
	if exporter.OutputDir != "" {
		return exporter.OutputDir
	}

	return "pkgs"
}

// outputRoot is the filesystem root facades are written under. Defaults to
// Dir; CheckAll redirects it to a temp dir so it can compare without mutating
// the real tree.
func (exporter *Exporter) outputRoot() string {
	if exporter.OutputRoot != "" {
		return exporter.OutputRoot
	}

	return exporter.Dir
}

// CheckAll verifies every facade (library mode) is in sync. See checkExport.
func (exporter *Exporter) CheckAll() error {
	// Full regeneration: an on-disk facade with no generated counterpart is
	// stale drift, so report those too.
	return exporter.checkExport(
		func(clone *Exporter) error { return clone.ExportAll() },
		true,
	)
}

// CheckPackage verifies a single package's facade is in sync. See checkExport.
func (exporter *Exporter) CheckPackage(pkgPattern string) error {
	// Partial regeneration: only the named package's files are in scope, so do
	// not flag unrelated on-disk facades as stale.
	return exporter.checkExport(
		func(clone *Exporter) error { return clone.ExportPackage(pkgPattern) },
		false,
	)
}

// CheckScan verifies the directive-marked packages' facades are in sync. See
// checkExport.
func (exporter *Exporter) CheckScan() error {
	return exporter.checkExport(
		func(clone *Exporter) error { return clone.ScanAndExport() },
		false,
	)
}

// checkExport renders + formats facades into a temp dir (via run, one of the
// Export* methods on a clone redirected there), then compares the result
// against the on-disk Dir/<outputDir> tree — without writing to it. It returns
// an error naming the out-of-sync packages if the committed facades differ
// from a fresh export (drifted content, missing facades, or — when
// reportStale is set — files no longer generated), else nil. This is the
// side-effect-free equivalent of `export` followed by `git diff --exit-code`.
//
// reportStale is meaningful only for full/library runs; partial runs
// (single-package, scan) regenerate a subset and must not flag the untouched
// on-disk facades as drift.
func (exporter *Exporter) checkExport(run func(*Exporter) error, reportStale bool) error {
	// Render the comparison copy into a temp dir UNDER the module root, not the
	// system temp dir. FormatOutput runs the project formatter (conformist)
	// with its tree root anchored at the config/module root, and a
	// formatter invoked on a path OUTSIDE that tree root formats nothing. A
	// system-temp output dir is in-tree only when $TMPDIR happens to sit inside
	// the repo; when it does not (the usual /tmp), the comparison copy is left
	// unformatted and diffs against the committed (formatted) facades, reporting
	// phantom drift (#125). Keeping the temp dir in-tree guarantees the
	// formatter treats it exactly like a real export's <outputDir>. The dot
	// prefix hides it from go/packages and `go build ./...`; defer removes it.
	tmp, err := os.MkdirTemp(exporter.Dir, ".dagnabit-check-")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmp) //defer:err-checked temp-dir cleanup; failure must not clobber the real return

	// Render + format into the temp root; never mutate the committed tree.
	// Consumer rewriting would touch files outside <outputDir>, so disable it.
	clone := *exporter
	clone.OutputRoot = tmp
	clone.SkipConsumerRewrite = true

	if err := run(&clone); err != nil {
		return err
	}
	if err := clone.FormatOutput(); err != nil {
		return err
	}

	want := filepath.Join(exporter.Dir, exporter.outputDir())
	got := filepath.Join(tmp, exporter.outputDir())

	drift, err := diffFacadeTrees(want, got, reportStale)
	if err != nil {
		return err
	}
	if len(drift) > 0 {
		return fmt.Errorf(
			"%s/ is out of sync with internal/ (run `dagnabit export` and commit):\n  %s",
			exporter.outputDir(),
			strings.Join(drift, "\n  "),
		)
	}

	return nil
}

// diffFacadeTrees compares the committed facade tree (want) against a freshly
// generated one (got), returning a sorted list of human-readable drift
// entries: files that differ or are missing from want, plus (when reportStale
// is set) files present in want but not regenerated.
func diffFacadeTrees(want, got string, reportStale bool) ([]string, error) {
	wantFiles, err := readTree(want)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", want, err)
	}
	gotFiles, err := readTree(got)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", got, err)
	}

	seen := make(map[string]struct{}, len(gotFiles))
	var drift []string
	for rel, gotBody := range gotFiles {
		seen[rel] = struct{}{}
		wantBody, ok := wantFiles[rel]
		if !ok {
			drift = append(drift, rel+" (missing — not committed)")
			continue
		}
		if !bytes.Equal(wantBody, gotBody) {
			drift = append(drift, rel+" (out of date)")
		}
	}
	if reportStale {
		for rel := range wantFiles {
			// Hand-written facade tests (*_test.go) live alongside generated
			// files and are not produced by the exporter (which only emits
			// main.go and build-tag facade files); never flag them as stale.
			if strings.HasSuffix(rel, "_test.go") {
				continue
			}
			if _, ok := seen[rel]; !ok {
				drift = append(drift, rel+" (stale — no longer generated)")
			}
		}
	}

	sort.Strings(drift)
	return drift, nil
}

// readTree maps each regular file under root to its contents, keyed by path
// relative to root. A non-existent root yields an empty map.
func readTree(root string) (map[string][]byte, error) {
	out := map[string][]byte{}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return out, nil
	}

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[rel] = body
		return nil
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}

// ExportPackage generates a facade for a single package path (e.g.,
// "./internal/alfa/blob_store_id" or "github.com/.../internal/alfa/blob_store_id").
func (exporter *Exporter) ExportPackage(pkgPattern string) error {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedFiles | packages.NeedSyntax,
		Dir:  exporter.Dir,
		Env:  exporter.Env,
	}

	pkgs, err := packages.Load(cfg, pkgPattern)
	if err != nil {
		return fmt.Errorf("loading package %s: %w", pkgPattern, err)
	}

	if len(pkgs) == 0 {
		return fmt.Errorf("no packages matched %s", pkgPattern)
	}

	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			return fmt.Errorf("package %s has errors: %v", pkg.PkgPath, pkg.Errors[0])
		}

		if err := exporter.exportSinglePackage(pkg); err != nil {
			return err
		}
	}

	return nil
}

// ScanAndExport walks internal/ looking for //go:generate dagnabit export
// directives and generates facades for each discovered package.
func (exporter *Exporter) ScanAndExport() error {
	internalDir := filepath.Join(exporter.Dir, "internal")

	pkgDirs, err := scanForExportDirectives(internalDir)
	if err != nil {
		return fmt.Errorf("scanning for export directives: %w", err)
	}

	if len(pkgDirs) == 0 {
		fmt.Fprintf(os.Stderr, "no //go:generate dagnabit export directives found under internal/\n")
		return nil
	}

	for _, dir := range pkgDirs {
		rel, err := filepath.Rel(exporter.Dir, dir)
		if err != nil {
			return fmt.Errorf("computing relative path for %s: %w", dir, err)
		}

		pattern := "./" + rel
		if err := exporter.ExportPackage(pattern); err != nil {
			return fmt.Errorf("exporting %s: %w", pattern, err)
		}
	}

	return nil
}

// ExportAll generates facades for every package under internal/, without
// requiring //go:generate dagnabit export directives. Packages containing a
// //dagnabit:private directive are skipped. It fails if any
// //go:generate dagnabit export directives are found — they are incompatible
// with library mode and should be removed.
func (exporter *Exporter) ExportAll() error {
	internalDir := filepath.Join(exporter.Dir, "internal")

	exportDirs, err := scanForExportDirectives(internalDir)
	if err != nil {
		return fmt.Errorf("scanning for export directives: %w", err)
	}

	if len(exportDirs) > 0 {
		return fmt.Errorf(
			"--library mode requires no //go:generate dagnabit export directives, but found them in:\n%s\nRemove these directives before using --library",
			strings.Join(exportDirs, "\n"),
		)
	}

	privateDirs, err := scanForPrivateDirectives(internalDir)
	if err != nil {
		return fmt.Errorf("scanning for private directives: %w", err)
	}

	privateSet := make(map[string]struct{}, len(privateDirs))
	for _, d := range privateDirs {
		privateSet[d] = struct{}{}
	}

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedFiles | packages.NeedSyntax,
		Dir:  exporter.Dir,
		Env:  exporter.Env,
	}

	pkgs, err := packages.Load(cfg, "./internal/...")
	if err != nil {
		return fmt.Errorf("loading internal packages: %w", err)
	}

	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			return fmt.Errorf("package %s has errors: %v", pkg.PkgPath, pkg.Errors[0])
		}

		if !strings.Contains(pkg.PkgPath, "/internal/") {
			continue
		}

		if len(pkg.GoFiles) > 0 {
			pkgDir := filepath.Dir(pkg.GoFiles[0])
			if _, private := privateSet[pkgDir]; private {
				continue
			}
		}

		if err := exporter.exportSinglePackage(pkg); err != nil {
			return fmt.Errorf("exporting %s: %w", pkg.PkgPath, err)
		}
	}

	return nil
}

// internalToFacade remaps an internal package path to its pkgs/ facade path.
// "mod/internal/<level>/<leaf>" → "mod/<outputDir>/<leaf>"; others pass through.
func internalToFacade(modulePath, outputDir, pkgPath string) string {
	prefix := modulePath + "/internal/"
	if !strings.HasPrefix(pkgPath, prefix) {
		return pkgPath
	}
	rest := strings.TrimPrefix(pkgPath, prefix)
	slash := strings.Index(rest, "/")
	if slash < 0 {
		return pkgPath
	}
	leaf := rest[slash+1:]
	return modulePath + "/" + outputDir + "/" + leaf
}

func (exporter *Exporter) exportSinglePackage(pkg *packages.Package) error {
	importPath := pkg.PkgPath

	relPath := strings.TrimPrefix(importPath, exporter.ModulePath+"/")
	relPath = strings.TrimPrefix(relPath, "internal/")

	// Strip the NATO level to get the facade subpath.
	parts := strings.SplitN(relPath, "/", 2)

	var facadeSubpath string
	if len(parts) >= 2 {
		facadeSubpath = parts[1]
	} else {
		facadeSubpath = parts[0]
	}

	facadePkgName := filepath.Base(facadeSubpath)

	outputPath := filepath.Join(
		exporter.outputRoot(),
		exporter.outputDir(),
		facadeSubpath,
		"main.go",
	)

	if exporter.DryRun {
		return exporter.dryRunSummary(pkg, outputPath)
	}

	outputSubdir := filepath.Join(exporter.outputRoot(), exporter.outputDir(), facadeSubpath)

	if exporter.Copy {
		if err := exporter.exportPackageAsCopy(pkg, outputSubdir); err != nil {
			return fmt.Errorf("copying %s into %s: %w", importPath, outputSubdir, err)
		}
	} else {
		// Do not remap importPath itself — same-package named types must stay on
		// the "internal" alias, not be redirected to the facade being generated
		// (which would create an import cycle).
		remap := func(p string) string {
			if p == importPath {
				return p
			}
			return internalToFacade(exporter.ModulePath, exporter.outputDir(), p)
		}
		docs := buildDocMap(pkg)
		facadeCode, err := generateFacadeJen(facadePkgName, importPath, pkg.Types, docs, remap)
		if err != nil {
			return fmt.Errorf("generating facade for %s: %w", importPath, err)
		}

		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return fmt.Errorf("creating output directory: %w", err)
		}

		if err := os.WriteFile(outputPath, facadeCode, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", outputPath, err)
		}

		fmt.Printf("generated: %s\n", outputPath)

		// Generate per-build-tag facade files for any tagged symbols.
		// Copy mode preserves the original tagged source files naturally,
		// so this analysis only runs in alias mode.
		pkgDir := ""
		if len(pkg.GoFiles) > 0 {
			pkgDir = filepath.Dir(pkg.GoFiles[0])
		}
		if pkgDir != "" {
			if err := exporter.exportTaggedFacades(pkg, pkgDir, importPath, facadePkgName, outputSubdir, remap); err != nil {
				return fmt.Errorf("generating tagged facades for %s: %w", importPath, err)
			}
		}
	}

	if exporter.SkipConsumerRewrite {
		return nil
	}

	facadeImportPath := exporter.ModulePath + "/" + exporter.outputDir() + "/" + facadeSubpath
	if err := exporter.rewriteConsumers(importPath, facadeImportPath); err != nil {
		return fmt.Errorf("rewriting consumers: %w", err)
	}

	return nil
}

// dryRunSummary prints a short report of what exportSinglePackage would
// emit for pkg. Used by --dry-run.
func (exporter *Exporter) dryRunSummary(pkg *packages.Package, outputPath string) error {
	if exporter.Copy {
		// In copy mode we emit one file per non-test source file in the
		// internal package's directory, with imports rewritten.
		var pkgDir string
		if len(pkg.GoFiles) > 0 {
			pkgDir = filepath.Dir(pkg.GoFiles[0])
		}
		fmt.Printf("would copy: %s/* → %s/* (rewriting internal/ imports)\n",
			pkgDir, filepath.Dir(outputPath))
		return nil
	}

	scope := pkg.Types.Scope()
	var nTypes, nVars, nFuncWrappers, nConsts int
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if !obj.Exported() {
			continue
		}
		switch obj := obj.(type) {
		case *types.TypeName:
			nTypes++
		case *types.Func:
			sig := obj.Type().(*types.Signature)
			if sig.TypeParams() != nil && sig.TypeParams().Len() > 0 {
				nFuncWrappers++
			} else {
				nVars++
			}
		case *types.Var:
			nVars++
		case *types.Const:
			nConsts++
		}
	}
	fmt.Printf("would generate: %s (%d types, %d vars, %d func wrappers, %d consts)\n",
		outputPath, nTypes, nVars, nFuncWrappers, nConsts)
	return nil
}

// exportTaggedFacades generates one facade file per unique build tag
// expression found in pkgDir, plus rewrites main.go to contain only symbols
// present across ALL tag combinations (the intersection).
//
// Algorithm:
//  1. Collect all build-tag expressions from the source directory.
//  2. For each positive expression (e.g. "test", "test && debug"), load the
//     package with those tags active.
//  3. Compute the intersection of base names ∩ all positive-tag name sets.
//     Symbols in the intersection are guaranteed to be present regardless of
//     build tags → these go in main.go.
//  4. For each positive expression, diff its symbol set against the
//     intersection → the extras go in <expr>.go with //go:build <expr>.
//  5. For each negation-only expression (e.g. "!debug"), diff base names
//     against the corresponding positive load (base − positive = unique to
//     the negated build) → these go in not_debug.go with //go:build !debug.
func (exporter *Exporter) exportTaggedFacades(
	basePkg *packages.Package,
	pkgDir, importPath, facadePkgName, outputSubdir string,
	remap func(string) string,
) error {
	tags, err := collectBuildTags(pkgDir)
	if err != nil {
		return fmt.Errorf("collecting build tags from %s: %w", pkgDir, err)
	}

	if len(tags) == 0 {
		return nil
	}

	baseNames := exportedNames(basePkg.Types)

	// Load every positive-tag combination and record their symbol sets.
	// For a negated expression like "!debug", the "positive counterpart" is
	// "debug" — we load that to discover which symbols disappear under !debug.
	type tagLoad struct {
		expr        string // original expression (e.g. "!debug")
		negated     bool   // true when expr is purely negated
		positiveKey string // positive atoms used for loading (e.g. "debug")
		pkg         *packages.Package
		names       map[string]struct{}
	}

	loads := make([]tagLoad, 0, len(tags))
	for _, expr := range tags {
		flags := buildFlagsForExpression(expr)
		negated := flags == ""

		if negated {
			// For "!debug", the positive counterpart is the atoms after stripping !
			// e.g. "!debug" → positiveKey "debug"
			positiveKey := positiveAtomsForNegatedExpr(expr)
			if positiveKey == "" {
				continue
			}
			pkg, err := exporter.loadPackageWithTag(importPath, positiveKey)
			if err != nil {
				return fmt.Errorf("loading %s with -tags %s (for %s): %w", importPath, positiveKey, expr, err)
			}
			loads = append(loads, tagLoad{
				expr:        expr,
				negated:     true,
				positiveKey: positiveKey,
				pkg:         pkg,
				names:       exportedNames(pkg.Types),
			})
		} else {
			pkg, err := exporter.loadPackageWithTag(importPath, flags)
			if err != nil {
				return fmt.Errorf("loading %s with -tags %s: %w", importPath, flags, err)
			}
			loads = append(loads, tagLoad{
				expr:        expr,
				negated:     false,
				positiveKey: flags,
				pkg:         pkg,
				names:       exportedNames(pkg.Types),
			})
		}
	}

	// Compute the intersection: symbols present in base AND in every positive load.
	intersection := copyNameSet(baseNames)
	for _, load := range loads {
		if !load.negated {
			for name := range intersection {
				if _, ok := load.names[name]; !ok {
					delete(intersection, name)
				}
			}
		}
	}

	// Rewrite main.go with only the intersection.
	baseDocs := buildDocMap(basePkg)
	mainCode, err := generateFacadeJenFiltered(facadePkgName, importPath, basePkg.Types, intersection, baseDocs, remap)
	if err != nil {
		return fmt.Errorf("regenerating main.go for %s: %w", importPath, err)
	}
	mainPath := filepath.Join(outputSubdir, "main.go")
	if err := os.WriteFile(mainPath, mainCode, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", mainPath, err)
	}
	fmt.Printf("generated: %s\n", mainPath)

	// Generate per-expression tagged facade files.
	for _, load := range loads {
		var uniqueNames []string
		if load.negated {
			// Symbols unique to the negated build = base − positive-load − intersection
			uniqueNames = diffSymbols(load.names, baseNames)
			// Further remove anything already in intersection (already in main.go)
			uniqueNames = filterOut(uniqueNames, intersection)
		} else {
			// Symbols unique to this positive tag combo = load − intersection
			uniqueNames = diffSymbols(intersection, load.names)
		}

		if len(uniqueNames) == 0 {
			continue
		}

		// Symbols for negated exprs come from basePkg (present without positive tags).
		// Symbols for positive exprs come from the tagged load (already cached in load.pkg).
		var (
			scope   *types.Scope
			tagDocs map[string]*ast.CommentGroup
		)
		if load.negated {
			scope = basePkg.Types.Scope()
			tagDocs = buildDocMap(basePkg)
		} else {
			scope = load.pkg.Types.Scope()
			tagDocs = buildDocMap(load.pkg)
		}

		code, err := generateTaggedFacadeJen(facadePkgName, importPath, load.expr, scope, uniqueNames, tagDocs, remap)
		if err != nil {
			return fmt.Errorf("generating tagged facade for %s expr=%s: %w", importPath, load.expr, err)
		}

		outPath := filepath.Join(outputSubdir, filenameForExpression(load.expr)+".go")
		if err := os.WriteFile(outPath, code, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", outPath, err)
		}
		fmt.Printf("generated: %s\n", outPath)
	}

	return nil
}

// positiveAtomsForNegatedExpr extracts the positive atoms from a purely-negated
// expression like "!debug" → "debug", or "!debug && !test" → "" (empty, since
// both are negated and there's no positive counterpart to load).
// For mixed expressions the caller uses buildFlagsForExpression instead.
func positiveAtomsForNegatedExpr(expr string) string {
	parts := strings.Split(expr, " && ")
	var atoms []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, "!") {
			atoms = append(atoms, strings.TrimPrefix(p, "!"))
		}
	}
	return strings.Join(atoms, ",")
}

// copyNameSet returns a shallow copy of a name set.
func copyNameSet(src map[string]struct{}) map[string]struct{} {
	dst := make(map[string]struct{}, len(src))
	for k := range src {
		dst[k] = struct{}{}
	}
	return dst
}

// filterOut removes from names any entry present in exclude, returning the
// remaining sorted slice.
func filterOut(names []string, exclude map[string]struct{}) []string {
	out := names[:0]
	for _, n := range names {
		if _, skip := exclude[n]; !skip {
			out = append(out, n)
		}
	}
	return out
}

// buildFlagsForExpression converts a //go:build expression to a -tags argument.
// "test && debug" → "test,debug"; "!debug" → "" (caller should skip negation-only).
// Strips negated atoms — the go tool can't activate a negated tag.
func buildFlagsForExpression(expr string) string {
	parts := strings.Split(expr, " && ")
	var positive []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if !strings.HasPrefix(p, "!") && p != "" {
			positive = append(positive, p)
		}
	}
	return strings.Join(positive, ",")
}

// filenameForExpression converts a //go:build expression to a safe filename stem.
// "test" → "test"; "test && debug" → "test_debug"; "!debug" → "not_debug".
func filenameForExpression(expr string) string {
	r := strings.NewReplacer(" && ", "_", "!", "not_", " ", "_")
	return r.Replace(expr)
}

// loadPackageWithTag reloads a package with a specific build tag expression active.
func (exporter *Exporter) loadPackageWithTag(importPath, expr string) (*packages.Package, error) {
	flags := buildFlagsForExpression(expr)
	cfg := &packages.Config{
		Mode:       packages.NeedName | packages.NeedTypes | packages.NeedFiles,
		Dir:        exporter.Dir,
		Env:        exporter.Env,
		BuildFlags: []string{"-tags", flags},
	}

	pkgs, err := packages.Load(cfg, importPath)
	if err != nil {
		return nil, err
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no packages matched %s", importPath)
	}
	if len(pkgs[0].Errors) > 0 {
		return nil, fmt.Errorf("%v", pkgs[0].Errors[0])
	}
	return pkgs[0], nil
}

// exportedNames returns the set of exported symbol names in a types.Package.
func exportedNames(typePkg *types.Package) map[string]struct{} {
	scope := typePkg.Scope()
	names := make(map[string]struct{}, len(scope.Names()))
	for _, name := range scope.Names() {
		if obj := scope.Lookup(name); obj.Exported() {
			names[name] = struct{}{}
		}
	}
	return names
}

// diffSymbols returns names present in tagged but absent in base.
func diffSymbols(base, tagged map[string]struct{}) []string {
	var result []string
	for name := range tagged {
		if _, inBase := base[name]; !inBase {
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result
}

// generateTaggedFacadeJen produces a facade file with a //go:build constraint,
// re-exporting only the symbols in names from importPath.
func generateTaggedFacadeJen(
	pkgName, importPath, tag string,
	scope *types.Scope,
	names []string,
	docs map[string]*ast.CommentGroup,
	remap func(string) string,
) ([]byte, error) {
	f := jen.NewFile(pkgName)
	f.HeaderComment(generatedHeaderText())
	f.HeaderComment("//go:build " + tag)
	f.ImportAlias(importPath, "internal")

	emitFacadeDecls(f, scope, names, importPath, docs, remap)

	var buf strings.Builder
	if err := f.Render(&buf); err != nil {
		return nil, fmt.Errorf("rendering: %w", err)
	}

	return []byte(buf.String()), nil
}

// collectBuildTags scans all .go files in dir and returns the unique,
// sorted, non-empty build-tag expressions found in //go:build lines.
func collectBuildTags(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}

		tag, err := firstBuildTag(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		if tag != "" {
			seen[tag] = struct{}{}
		}
	}

	tags := make([]string, 0, len(seen))
	for t := range seen {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	return tags, nil
}

// firstBuildTag returns the first //go:build expression from a file header,
// or "" if none is found. Stops reading at the first non-comment, non-blank line.
func firstBuildTag(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer files.CloseReadOnly(f)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "//go:build ") {
			return strings.TrimPrefix(line, "//go:build "), nil
		}
		if line != "" && !strings.HasPrefix(line, "//") {
			break
		}
	}
	return "", scanner.Err()
}

// generateFacadeJenFiltered is like generateFacadeJen but only emits symbols
// whose names are present in the allow set. Used to write main.go with only
// the intersection of symbols across all build-tag combinations.
func generateFacadeJenFiltered(
	pkgName, importPath string,
	typePkg *types.Package,
	allow map[string]struct{},
	docs map[string]*ast.CommentGroup,
	remap func(string) string,
) ([]byte, error) {
	scope := typePkg.Scope()
	var filtered []string
	for _, name := range scope.Names() {
		if _, ok := allow[name]; ok {
			filtered = append(filtered, name)
		}
	}
	sort.Strings(filtered)

	f := jen.NewFile(pkgName)
	f.HeaderComment(generatedHeaderText())
	f.ImportAlias(importPath, "internal")

	emitFacadeDecls(f, scope, filtered, importPath, docs, remap)

	var buf strings.Builder
	if err := f.Render(&buf); err != nil {
		return nil, fmt.Errorf("rendering: %w", err)
	}

	return []byte(buf.String()), nil
}

// generateFacadeJen produces a facade Go source file using jennifer for
// proper import management.
func generateFacadeJen(
	pkgName, importPath string,
	typePkg *types.Package,
	docs map[string]*ast.CommentGroup,
	remap func(string) string,
) ([]byte, error) {
	f := jen.NewFile(pkgName)
	f.HeaderComment(generatedHeaderText())

	// Force the internal package to use "internal" as its import alias.
	// This avoids collisions when the package name matches a stdlib package
	// (e.g., internal/alfa/cmp vs stdlib cmp).
	f.ImportAlias(importPath, "internal")

	scope := typePkg.Scope()
	emitFacadeDecls(f, scope, scope.Names(), importPath, docs, remap)

	var buf strings.Builder
	if err := f.Render(&buf); err != nil {
		return nil, fmt.Errorf("rendering: %w", err)
	}

	return []byte(buf.String()), nil
}

// emitFacadeDecls writes per-symbol top-level declarations to f, in the
// order: types, then non-generic vars/funcs (alphabetical), then generic
// function wrappers, then consts. Each declaration is preceded by the
// symbol's doc comment lines (from docs[name]) when present.
//
// Per-symbol (rather than grouped `var (…)` / `type (…)` / `const (…)`)
// emission is required by issue #102: jennifer does not support per-spec
// comments inside grouped Defs, and doc-comment propagation needs to
// attach to each individual declaration.
func emitFacadeDecls(
	f *jen.File,
	scope *types.Scope,
	names []string,
	importPath string,
	docs map[string]*ast.CommentGroup,
	remap func(string) string,
) {
	type emission struct {
		name string
		emit func()
	}
	var (
		typeEmissions  []emission
		varEmissions   []emission
		funcEmissions  []emission
		constEmissions []emission
	)

	for _, name := range names {
		obj := scope.Lookup(name)
		if obj == nil || !obj.Exported() {
			continue
		}

		switch obj := obj.(type) {
		case *types.TypeName:
			typeEmissions = append(typeEmissions, emission{name, func() {
				emitDoc(f, docs[name])
				f.Type().Add(jenTypeAlias(name, importPath, obj, remap))
			}})

		case *types.Func:
			sig := obj.Type().(*types.Signature)
			if sig.TypeParams() != nil && sig.TypeParams().Len() > 0 {
				if referencesUnexportedType(sig.Params()) || referencesUnexportedType(sig.Results()) {
					continue
				}
				funcEmissions = append(funcEmissions, emission{name, func() {
					emitDoc(f, docs[name])
					f.Add(jenFuncWrapper(name, importPath, sig, remap))
				}})
			} else {
				varEmissions = append(varEmissions, emission{name, func() {
					emitDoc(f, docs[name])
					f.Var().Id(name).Op("=").Qual(importPath, name)
				}})
			}

		case *types.Var:
			varEmissions = append(varEmissions, emission{name, func() {
				emitDoc(f, docs[name])
				f.Var().Id(name).Op("=").Qual(importPath, name)
			}})

		case *types.Const:
			constEmissions = append(constEmissions, emission{name, func() {
				emitDoc(f, docs[name])
				f.Const().Id(name).Op("=").Qual(importPath, name)
			}})
		}
	}

	for _, e := range typeEmissions {
		e.emit()
	}
	for _, e := range varEmissions {
		e.emit()
	}
	if len(funcEmissions) > 0 {
		f.Comment("Generic function wrappers — Go does not support assigning")
		f.Comment("generic functions to variables without instantiation.")
		f.Comment("See https://github.com/golang/go/issues/52654")
		for _, e := range funcEmissions {
			e.emit()
		}
	}
	for _, e := range constEmissions {
		e.emit()
	}
}

// emitDoc writes each line of a doc comment to f as a top-level
// comment, immediately preceding the next declaration. No-op when cg
// is nil or has no surviving lines after directive-stripping.
func emitDoc(f *jen.File, cg *ast.CommentGroup) {
	for _, line := range docCommentLines(cg) {
		f.Comment(line)
	}
}

// jenTypeAlias produces a type alias statement, handling generic types.
func jenTypeAlias(name, importPath string, obj *types.TypeName, remap func(string) string) *jen.Statement {
	var params *types.TypeParamList

	switch t := obj.Type().(type) {
	case *types.Named:
		params = t.TypeParams()
	case *types.Alias:
		params = t.TypeParams()
	}

	if params == nil || params.Len() == 0 {
		return jen.Id(name).Op("=").Qual(importPath, name)
	}

	// Generic type: type X[T any] = internal.X[T]
	s := jen.Id(name)

	s.TypesFunc(func(g *jen.Group) {
		for i := 0; i < params.Len(); i++ {
			p := params.At(i)
			g.Add(jenTypeParam(p, remap))
		}
	})

	s.Op("=")

	rhs := jen.Qual(importPath, name)
	rhs.TypesFunc(func(g *jen.Group) {
		for i := 0; i < params.Len(); i++ {
			g.Add(jen.Id(params.At(i).Obj().Name()))
		}
	})

	s.Add(rhs)

	return s
}

// jenFuncWrapper produces a wrapper function for a generic function.
func jenFuncWrapper(name, importPath string, sig *types.Signature, remap func(string) string) jen.Code {
	params := sig.TypeParams()

	stmt := jen.Func().Id(name)

	// Type parameters
	stmt.TypesFunc(func(g *jen.Group) {
		for i := 0; i < params.Len(); i++ {
			g.Add(jenTypeParam(params.At(i), remap))
		}
	})

	// Function parameters
	stmt.ParamsFunc(func(g *jen.Group) {
		p := sig.Params()
		for i := 0; i < p.Len(); i++ {
			param := p.At(i)
			paramName := param.Name()
			if paramName == "" {
				paramName = fmt.Sprintf("p%d", i)
			}

			if sig.Variadic() && i == p.Len()-1 {
				// Variadic: last param is ...T, underlying is *types.Slice
				sliceType := param.Type().(*types.Slice)
				g.Add(jen.Id(paramName).Op("...").Add(jenType(sliceType.Elem(), remap)))
			} else {
				g.Add(jen.Id(paramName).Add(jenType(param.Type(), remap)))
			}
		}
	})

	// Return types
	res := sig.Results()
	if res.Len() > 0 {
		if res.Len() == 1 && !isRealName(res.At(0).Name()) {
			stmt.Add(jenType(res.At(0).Type(), remap))
		} else {
			stmt.ParamsFunc(func(g *jen.Group) {
				for i := 0; i < res.Len(); i++ {
					r := res.At(i)
					if isRealName(r.Name()) {
						g.Add(jen.Id(r.Name()).Add(jenType(r.Type(), remap)))
					} else {
						g.Add(jenType(r.Type(), remap))
					}
				}
			})
		}
	}

	// Body: return internal.Func[T, U](args...)
	call := jen.Qual(importPath, name)
	call.TypesFunc(func(g *jen.Group) {
		for i := 0; i < params.Len(); i++ {
			g.Add(jen.Id(params.At(i).Obj().Name()))
		}
	})
	call.CallFunc(func(g *jen.Group) {
		p := sig.Params()
		for i := 0; i < p.Len(); i++ {
			param := p.At(i)
			paramName := param.Name()
			if paramName == "" {
				paramName = fmt.Sprintf("p%d", i)
			}

			if sig.Variadic() && i == p.Len()-1 {
				g.Add(jen.Id(paramName).Op("..."))
			} else {
				g.Add(jen.Id(paramName))
			}
		}
	})

	if res.Len() > 0 {
		stmt.Block(jen.Return(call))
	} else {
		stmt.Block(call)
	}

	return stmt
}

// referencesUnexportedType checks if any type in a tuple references an
// unexported named type. Used to skip generic function wrappers whose
// return types would expose private types from the internal package.
func referencesUnexportedType(tuple *types.Tuple) bool {
	for i := 0; i < tuple.Len(); i++ {
		if containsUnexportedNamed(tuple.At(i).Type()) {
			return true
		}
	}

	return false
}

func containsUnexportedNamed(t types.Type) bool {
	switch t := t.(type) {
	case *types.Named:
		if t.Obj().Pkg() != nil && !t.Obj().Exported() {
			return true
		}

		for i := 0; i < t.TypeArgs().Len(); i++ {
			if containsUnexportedNamed(t.TypeArgs().At(i)) {
				return true
			}
		}

	case *types.Pointer:
		return containsUnexportedNamed(t.Elem())

	case *types.Slice:
		return containsUnexportedNamed(t.Elem())

	case *types.Map:
		return containsUnexportedNamed(t.Key()) || containsUnexportedNamed(t.Elem())

	case *types.Chan:
		return containsUnexportedNamed(t.Elem())
	}

	return false
}

// isRealName returns true if the name is a real user-defined name,
// not a synthetic name generated by go/types (e.g., "#rv1").
func isRealName(name string) bool {
	return name != "" && !strings.HasPrefix(name, "#")
}

// jenTypeParam renders a single type parameter declaration (e.g., "T any").
func jenTypeParam(p *types.TypeParam, remap func(string) string) jen.Code {
	return jen.Id(p.Obj().Name()).Add(jenType(p.Constraint(), remap))
}

// jenType converts a go/types.Type to a jennifer code statement.
// remap translates internal package paths to their pkgs/ facade equivalents.
func jenType(t types.Type, remap func(string) string) jen.Code {
	switch t := t.(type) {
	case *types.Named:
		pkg := t.Obj().Pkg()

		var base *jen.Statement
		if pkg == nil {
			// Built-in type (e.g., error)
			base = jen.Id(t.Obj().Name())
		} else {
			base = jen.Qual(remap(pkg.Path()), t.Obj().Name())
		}

		// Handle type arguments for instantiated generics
		if t.TypeArgs() != nil && t.TypeArgs().Len() > 0 {
			base.TypesFunc(func(g *jen.Group) {
				for i := 0; i < t.TypeArgs().Len(); i++ {
					g.Add(jenType(t.TypeArgs().At(i), remap))
				}
			})
		}

		return base

	case *types.Pointer:
		return jen.Op("*").Add(jenType(t.Elem(), remap))

	case *types.Slice:
		return jen.Index().Add(jenType(t.Elem(), remap))

	case *types.Array:
		return jen.Index(jen.Lit(int(t.Len()))).Add(jenType(t.Elem(), remap))

	case *types.Map:
		return jen.Map(jenType(t.Key(), remap)).Add(jenType(t.Elem(), remap))

	case *types.Chan:
		switch t.Dir() {
		case types.SendRecv:
			return jen.Chan().Add(jenType(t.Elem(), remap))
		case types.SendOnly:
			return jen.Chan().Op("<-").Add(jenType(t.Elem(), remap))
		case types.RecvOnly:
			return jen.Op("<-").Chan().Add(jenType(t.Elem(), remap))
		}

	case *types.Basic:
		return jen.Id(t.Name())

	case *types.Interface:
		if t.Empty() {
			return jen.Any()
		}

		// For complex interfaces used as constraints, render inline.
		return jen.InterfaceFunc(func(g *jen.Group) {
			for i := 0; i < t.NumEmbeddeds(); i++ {
				g.Add(jenType(t.EmbeddedType(i), remap))
			}

			for i := 0; i < t.NumExplicitMethods(); i++ {
				m := t.ExplicitMethod(i)
				g.Add(jen.Id(m.Name()).Add(jenSignature(m.Type().(*types.Signature), remap)))
			}
		})

	case *types.Signature:
		return jen.Func().Add(jenSignature(t, remap))

	case *types.Struct:
		return jen.StructFunc(func(g *jen.Group) {
			for i := 0; i < t.NumFields(); i++ {
				field := t.Field(i)
				g.Add(jen.Id(field.Name()).Add(jenType(field.Type(), remap)))
			}
		})

	case *types.TypeParam:
		return jen.Id(t.Obj().Name())

	case *types.Union:
		// Union types in constraints: T1 | T2 | ...
		var parts []jen.Code
		for i := 0; i < t.Len(); i++ {
			term := t.Term(i)
			code := jenType(term.Type(), remap)
			if term.Tilde() {
				code = jen.Op("~").Add(code)
			}

			parts = append(parts, code)
		}

		if len(parts) == 1 {
			return parts[0]
		}

		s := parts[0].(*jen.Statement)
		for _, p := range parts[1:] {
			s = s.Op("|").Add(p)
		}

		return s

	case *types.Alias:
		pkg := t.Obj().Pkg()
		var base *jen.Statement
		if pkg == nil {
			base = jen.Id(t.Obj().Name())
		} else {
			base = jen.Qual(remap(pkg.Path()), t.Obj().Name())
		}
		if t.TypeArgs() != nil && t.TypeArgs().Len() > 0 {
			base.TypesFunc(func(g *jen.Group) {
				for i := 0; i < t.TypeArgs().Len(); i++ {
					g.Add(jenType(t.TypeArgs().At(i), remap))
				}
			})
		}
		return base
	}

	// Fallback: use string representation (shouldn't happen for well-typed code)
	return jen.Id(t.String())
}

// jenSignature renders function params and results (without "func" keyword).
func jenSignature(sig *types.Signature, remap func(string) string) jen.Code {
	s := jen.ParamsFunc(func(g *jen.Group) {
		p := sig.Params()
		for i := 0; i < p.Len(); i++ {
			param := p.At(i)
			name := param.Name()

			var paramCode jen.Code
			if sig.Variadic() && i == p.Len()-1 {
				sliceType := param.Type().(*types.Slice)
				if isRealName(name) {
					paramCode = jen.Id(name).Op("...").Add(jenType(sliceType.Elem(), remap))
				} else {
					paramCode = jen.Op("...").Add(jenType(sliceType.Elem(), remap))
				}
			} else if isRealName(name) {
				paramCode = jen.Id(name).Add(jenType(param.Type(), remap))
			} else {
				paramCode = jenType(param.Type(), remap)
			}

			g.Add(paramCode)
		}
	})

	res := sig.Results()
	if res.Len() == 1 && !isRealName(res.At(0).Name()) {
		s.Add(jenType(res.At(0).Type(), remap))
	} else if res.Len() > 0 {
		s.ParamsFunc(func(g *jen.Group) {
			for i := 0; i < res.Len(); i++ {
				r := res.At(i)
				if isRealName(r.Name()) {
					g.Add(jen.Id(r.Name()).Add(jenType(r.Type(), remap)))
				} else {
					g.Add(jenType(r.Type(), remap))
				}
			}
		})
	}

	return s
}

// scanForExportDirectives walks a directory tree and returns paths of
// directories containing Go files with //go:generate dagnabit export.
func scanForExportDirectives(root string) ([]string, error) {
	seen := make(map[string]struct{})
	var dirs []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		dir := filepath.Dir(path)
		if _, ok := seen[dir]; ok {
			return nil
		}

		has, err := fileContainsDirective(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		if has {
			seen[dir] = struct{}{}
			dirs = append(dirs, dir)
		}

		return nil
	})

	sort.Strings(dirs)

	return dirs, err
}

// scanForPrivateDirectives walks a directory tree and returns paths of
// directories containing Go files with //dagnabit:private.
func scanForPrivateDirectives(root string) ([]string, error) {
	seen := make(map[string]struct{})
	var dirs []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		dir := filepath.Dir(path)
		if _, ok := seen[dir]; ok {
			return nil
		}

		has, err := fileContainsPrivateDirective(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		if has {
			seen[dir] = struct{}{}
			dirs = append(dirs, dir)
		}

		return nil
	})

	sort.Strings(dirs)

	return dirs, err
}

func fileContainsPrivateDirective(path string) (bool, error) {
	return fileContainsLine(path, privateDirective)
}

func fileContainsDirective(path string) (bool, error) {
	return fileContainsLine(path, exportDirective)
}

func fileContainsLine(path, needle string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer files.CloseReadOnly(f)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == needle {
			return true, nil
		}
	}

	return false, scanner.Err()
}
