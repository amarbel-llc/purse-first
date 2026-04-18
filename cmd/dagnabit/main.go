package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/amarbel-llc/purse-first/libs/dewey/0/dagnabit"
)

func main() {
	if len(os.Args) > 1 && os.Args[0] != "-" && os.Args[1] == "export" {
		os.Args = append(os.Args[:1], os.Args[2:]...)
		runExport()
		return
	}

	runReposition()
}

func runReposition() {
	var dryRun bool
	var verbose bool
	var modulePath string
	var depth int

	flag.BoolVar(&dryRun, "n", false, "show what would be moved without moving")
	flag.BoolVar(&dryRun, "dry-run", false, "show what would be moved without moving")
	flag.BoolVar(&verbose, "v", false, "enable verbose output")
	flag.BoolVar(&verbose, "verbose", false, "enable verbose output")
	flag.StringVar(&modulePath, "module", "", "Go module path (read from go.mod if empty)")
	flag.IntVar(&depth, "depth", 3, "path component depth: 3 for prefix/level/package, 2 for level/package")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: dagnabit [flags] <prefix>...\n\n")
		fmt.Fprintf(os.Stderr, "Positional arguments:\n")
		fmt.Fprintf(os.Stderr, "  prefix   package tree prefixes to analyze (e.g. lib internal src)\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	prefixes := flag.Args()
	if len(prefixes) == 0 {
		fmt.Fprintf(os.Stderr, "error: at least one prefix path is required\n\n")
		flag.Usage()
		os.Exit(1)
	}

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	modulePath, err = resolveModulePath(dir, modulePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	reader := &dagnabit.GoListReader{
		Dir:             dir,
		ModulePath:      modulePath,
		PackagePrefixes: prefixes,
		ComponentDepth:  depth,
		Verbose:         verbose,
	}

	mapper := dagnabit.MakeNATOLevelMapper()

	mover := &dagnabit.GitMover{Dir: dir, ModulePath: modulePath}

	r := &dagnabit.Repositioner{
		Reader:         reader,
		Mapper:         mapper,
		Mover:          mover,
		DryRun:         dryRun,
		Verbose:        verbose,
		ComponentDepth: depth,
	}

	if err := r.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runExport() {
	exportFlags := flag.NewFlagSet("export", flag.ExitOnError)

	var dryRun bool
	var outputDir string
	var modulePath string

	exportFlags.BoolVar(&dryRun, "n", false, "show what would be generated without writing files")
	exportFlags.BoolVar(&dryRun, "dry-run", false, "show what would be generated without writing files")
	exportFlags.StringVar(&outputDir, "output-dir", "pkgs", "output directory for generated facades (relative to module root)")
	exportFlags.StringVar(&modulePath, "module", "", "Go module path (read from go.mod if empty)")
	exportFlags.Parse(os.Args[1:])

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	modulePath, err = resolveModulePath(dir, modulePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	exporter := &dagnabit.Exporter{
		ModulePath: modulePath,
		Dir:        dir,
		OutputDir:  outputDir,
		DryRun:     dryRun,
	}

	args := exportFlags.Args()

	if len(args) > 0 {
		for _, arg := range args {
			if err := exporter.ExportPackage(arg); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		}
	} else {
		if err := exporter.ScanAndExport(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
}

func resolveModulePath(dir, modulePath string) (string, error) {
	if modulePath != "" {
		return modulePath, nil
	}

	goModPath := filepath.Join(dir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return "", fmt.Errorf("must be run from a directory containing go.mod")
	}

	return readModulePath(goModPath)
}

func readModulePath(goModPath string) (string, error) {
	f, err := os.Open(goModPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}

	return "", fmt.Errorf("no module directive found in %s", goModPath)
}
