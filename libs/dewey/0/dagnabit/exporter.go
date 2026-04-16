package dagnabit

import (
	"bufio"
	"bytes"
	"fmt"
	"go/format"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

const exportDirective = "//go:generate dagnabit export"

// Exporter generates pkgs/ facade files from internal packages.
type Exporter struct {
	ModulePath string
	Dir        string
	OutputDir  string
	DryRun     bool
}

func (exporter *Exporter) outputDir() string {
	if exporter.OutputDir != "" {
		return exporter.OutputDir
	}

	return "pkgs"
}

// ExportPackage generates a facade for a single package path (e.g.,
// "./internal/alfa/blob_store_id" or "github.com/.../internal/alfa/blob_store_id").
func (exporter *Exporter) ExportPackage(pkgPattern string) error {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedTypes,
		Dir:  exporter.Dir,
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

func (exporter *Exporter) exportSinglePackage(pkg *packages.Package) error {
	importPath := pkg.PkgPath

	// Extract the package name for the facade directory.
	// Strip the module path prefix and "internal/" to get "level/name",
	// then take the last component as the package name.
	relPath := strings.TrimPrefix(importPath, exporter.ModulePath+"/")
	relPath = strings.TrimPrefix(relPath, "internal/")

	// The last component after stripping internal/ and the level is the package name.
	// e.g., "alfa/blob_store_id" -> "blob_store_id"
	// e.g., "0/domain_interfaces" -> "domain_interfaces"
	parts := strings.SplitN(relPath, "/", 2)

	var facadePkgName string
	if len(parts) >= 2 {
		facadePkgName = parts[1]
	} else {
		facadePkgName = parts[0]
	}

	symbols := enumerateExportedSymbols(pkg)

	facadeCode := generateFacade(facadePkgName, importPath, symbols)

	outputPath := filepath.Join(
		exporter.Dir,
		exporter.outputDir(),
		facadePkgName,
		"main.go",
	)

	if exporter.DryRun {
		fmt.Printf("would generate: %s (%d types, %d vars, %d consts)\n",
			outputPath,
			len(symbols.Types),
			len(symbols.Vars),
			len(symbols.Consts),
		)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	if err := os.WriteFile(outputPath, facadeCode, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", outputPath, err)
	}

	if err := runGoimports(outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "warning: goimports failed on %s: %v\n", outputPath, err)
	}

	fmt.Printf("generated: %s\n", outputPath)
	return nil
}

// Symbol categories for facade generation.
type ExportedSymbols struct {
	Types  []TypeSymbol
	Vars   []string
	Consts []string
}

// TypeSymbol represents an exported type, possibly with type parameters.
type TypeSymbol struct {
	Name       string
	TypeParams string // e.g., "[T any, U comparable]" or "" for non-generic
	TypeArgs   string // e.g., "[T, U]" or "" for non-generic
}

func enumerateExportedSymbols(pkg *packages.Package) ExportedSymbols {
	scope := pkg.Types.Scope()
	names := scope.Names()

	var symbols ExportedSymbols

	for _, name := range names {
		obj := scope.Lookup(name)
		if !obj.Exported() {
			continue
		}

		switch obj.(type) {
		case *types.TypeName:
			ts := TypeSymbol{Name: name}

			named, ok := obj.Type().(*types.Named)
			if ok && named.TypeParams() != nil {
				ts.TypeParams, ts.TypeArgs = formatTypeParams(named.TypeParams())
			}

			symbols.Types = append(symbols.Types, ts)

		case *types.Func:
			symbols.Vars = append(symbols.Vars, name)

		case *types.Var:
			symbols.Vars = append(symbols.Vars, name)

		case *types.Const:
			symbols.Consts = append(symbols.Consts, name)
		}
	}

	sort.Slice(symbols.Types, func(i, j int) bool {
		return symbols.Types[i].Name < symbols.Types[j].Name
	})
	sort.Strings(symbols.Vars)
	sort.Strings(symbols.Consts)

	return symbols
}

func formatTypeParams(params *types.TypeParamList) (declaration, usage string) {
	if params == nil || params.Len() == 0 {
		return "", ""
	}

	var declParts, usageParts []string

	for i := 0; i < params.Len(); i++ {
		p := params.At(i)
		constraint := p.Constraint()
		declParts = append(declParts, fmt.Sprintf("%s %s", p.Obj().Name(), types.TypeString(constraint, nil)))
		usageParts = append(usageParts, p.Obj().Name())
	}

	return "[" + strings.Join(declParts, ", ") + "]", "[" + strings.Join(usageParts, ", ") + "]"
}

func generateFacade(pkgName, importPath string, symbols ExportedSymbols) []byte {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "// Code generated by dagnabit; DO NOT EDIT.\n\n")
	fmt.Fprintf(&buf, "package %s\n\n", pkgName)
	fmt.Fprintf(&buf, "import (\n")
	fmt.Fprintf(&buf, "\tinternal %q\n", importPath)
	fmt.Fprintf(&buf, ")\n")

	if len(symbols.Types) > 0 {
		fmt.Fprintf(&buf, "\ntype (\n")

		for _, ts := range symbols.Types {
			if ts.TypeParams != "" {
				fmt.Fprintf(&buf, "\t%s%s = internal.%s%s\n", ts.Name, ts.TypeParams, ts.Name, ts.TypeArgs)
			} else {
				fmt.Fprintf(&buf, "\t%s = internal.%s\n", ts.Name, ts.Name)
			}
		}

		fmt.Fprintf(&buf, ")\n")
	}

	if len(symbols.Vars) > 0 {
		fmt.Fprintf(&buf, "\nvar (\n")

		for _, name := range symbols.Vars {
			fmt.Fprintf(&buf, "\t%s = internal.%s\n", name, name)
		}

		fmt.Fprintf(&buf, ")\n")
	}

	if len(symbols.Consts) > 0 {
		fmt.Fprintf(&buf, "\nconst (\n")

		for _, name := range symbols.Consts {
			fmt.Fprintf(&buf, "\t%s = internal.%s\n", name, name)
		}

		fmt.Fprintf(&buf, ")\n")
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: gofmt failed for %s facade: %v\n", pkgName, err)
		return buf.Bytes()
	}

	return formatted
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

func fileContainsDirective(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == exportDirective {
			return true, nil
		}
	}

	return false, scanner.Err()
}

func runGoimports(path string) error {
	cmd := exec.Command("goimports", "-w", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("goimports: %v\n%s", err, output)
	}

	return nil
}
