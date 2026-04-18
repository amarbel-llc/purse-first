package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/amarbel-llc/purse-first/libs/dewey/0/dagnabit"
	go_list "github.com/amarbel-llc/purse-first/libs/dewey/0/go_list"
	go_module "github.com/amarbel-llc/purse-first/libs/dewey/0/go_module"
	nato_levels "github.com/amarbel-llc/purse-first/libs/dewey/0/nato_levels"
)

func main() {
	if len(os.Args) > 1 && os.Args[0] != "-" && os.Args[1] == "export" {
		os.Args = append(os.Args[:1], os.Args[2:]...)
		runExport()
		return
	}

	if len(os.Args) > 1 && os.Args[0] != "-" && os.Args[1] == "move" {
		os.Args = append(os.Args[:1], os.Args[2:]...)
		runMove()
		return
	}

	runReposition()
}

func runReposition() {
	var dryRun bool
	var verbose bool
	var modulePath string
	var depth int
	var initial bool

	flag.BoolVar(&dryRun, "n", false, "show what would be moved without moving")
	flag.BoolVar(&dryRun, "dry-run", false, "show what would be moved without moving")
	flag.BoolVar(&verbose, "v", false, "enable verbose output")
	flag.BoolVar(&verbose, "verbose", false, "enable verbose output")
	flag.StringVar(&modulePath, "module", "", "Go module path (read from go.mod if empty)")
	flag.IntVar(&depth, "depth", 3, "path component depth: 3 for prefix/level/package, 2 for level/package")
	flag.BoolVar(&initial, "initial", false, "insert a NATO level segment into a flat <prefix>/<pkg> layout (forces depth=2)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: dagnabit [flags] <prefix>...\n\n")
		fmt.Fprintf(os.Stderr, "Positional arguments:\n")
		fmt.Fprintf(os.Stderr, "  prefix   package tree prefixes to analyze (e.g. lib internal src)\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if initial {
		depth = 2
	}

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

	modulePath, err = go_module.ResolveModulePath(dir, modulePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	reader := &go_list.Reader{
		Dir:             dir,
		ModulePath:      modulePath,
		PackagePrefixes: prefixes,
		ComponentDepth:  depth,
		Verbose:         verbose,
	}

	mapper := nato_levels.MakeNATOLevelMapper()

	mover := &dagnabit.GitMover{Dir: dir, ModulePath: modulePath}

	mode := dagnabit.ModeReposition
	if initial {
		mode = dagnabit.ModeInitial
	}

	r := &dagnabit.Repositioner{
		Reader:         reader,
		Mapper:         mapper,
		Mover:          mover,
		DryRun:         dryRun,
		Verbose:        verbose,
		ComponentDepth: depth,
		Mode:           mode,
	}

	if err := r.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runMove() {
	moveFlags := flag.NewFlagSet("move", flag.ExitOnError)

	var dryRun bool
	var verbose bool
	var modulePath string
	var force bool

	moveFlags.BoolVar(&dryRun, "n", false, "show what would be moved without moving")
	moveFlags.BoolVar(&dryRun, "dry-run", false, "show what would be moved without moving")
	moveFlags.BoolVar(&verbose, "v", false, "enable verbose output")
	moveFlags.BoolVar(&verbose, "verbose", false, "enable verbose output")
	moveFlags.StringVar(&modulePath, "module", "", "Go module path (read from go.mod if empty)")
	moveFlags.BoolVar(&force, "force", false, "proceed even if packages.Load reports type errors")
	moveFlags.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: dagnabit move [flags] <src> <dst>\n\n")
		fmt.Fprintf(os.Stderr, "Positional arguments:\n")
		fmt.Fprintf(os.Stderr, "  src   package path relative to module root (e.g. internal/foo)\n")
		fmt.Fprintf(os.Stderr, "  dst   destination path relative to module root (e.g. internal/bar)\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		moveFlags.PrintDefaults()
	}

	moveFlags.Parse(os.Args[1:])

	args := moveFlags.Args()
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "error: move requires exactly 2 positional arguments (src and dst)\n\n")
		moveFlags.Usage()
		os.Exit(1)
	}

	src := args[0]
	dst := args[1]

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	modulePath, err = go_module.ResolveModulePath(dir, modulePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	mover := &dagnabit.GitMover{Dir: dir, ModulePath: modulePath}

	opts := dagnabit.MoveOptions{
		DryRun:          dryRun,
		Verbose:         verbose,
		AllowTypeErrors: force,
	}

	if err := mover.MovePackageRename(src, dst, opts); err != nil {
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

	modulePath, err = go_module.ResolveModulePath(dir, modulePath)
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
