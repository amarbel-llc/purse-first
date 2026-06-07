package main

import (
	"flag"
	"fmt"
	"os"

	cargo_metadata "github.com/amarbel-llc/purse-first/libs/dewey/pkgs/cargo_metadata"
	"github.com/amarbel-llc/purse-first/libs/dewey/pkgs/dagnabit"
	dagnabit_rust "github.com/amarbel-llc/purse-first/libs/dewey/pkgs/dagnabit_rust"
	go_list "github.com/amarbel-llc/purse-first/libs/dewey/pkgs/go_list"
	go_module "github.com/amarbel-llc/purse-first/libs/dewey/pkgs/go_module"
	nato_levels "github.com/amarbel-llc/purse-first/libs/dewey/pkgs/nato_levels"
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

	if len(os.Args) > 1 && os.Args[0] != "-" && os.Args[1] == "rename" {
		os.Args = append(os.Args[:1], os.Args[2:]...)
		runRename()
		return
	}

	runReposition()
}

func runRename() {
	renameFlags := flag.NewFlagSet("rename", flag.ExitOnError)

	var dryRun bool
	var verbose bool
	var modulePath string
	var force bool

	renameFlags.BoolVar(&dryRun, "n", false, "show what would happen without moving")
	renameFlags.BoolVar(&dryRun, "dry-run", false, "show what would happen without moving")
	renameFlags.BoolVar(&verbose, "v", false, "enable verbose output")
	renameFlags.BoolVar(&verbose, "verbose", false, "enable verbose output")
	renameFlags.StringVar(&modulePath, "module", "", "Go module path (read from go.mod if empty)")
	renameFlags.BoolVar(&force, "force", false, "proceed even if packages.Load reports type errors")
	renameFlags.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: dagnabit rename [flags] <src> [<new-leaf>]\n\n")
		fmt.Fprintf(os.Stderr, "Repositions ONE package to the NATO level dictated by its\n")
		fmt.Fprintf(os.Stderr, "transitive in-module dependencies, optionally renaming its leaf.\n")
		fmt.Fprintf(os.Stderr, "Other packages are NOT moved.\n\n")
		fmt.Fprintf(os.Stderr, "Positional arguments:\n")
		fmt.Fprintf(os.Stderr, "  src        package path relative to module root (e.g. charlie/dagnabit)\n")
		fmt.Fprintf(os.Stderr, "  new-leaf   new leaf directory name (optional; defaults to src's leaf)\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		renameFlags.PrintDefaults()
	}

	renameFlags.Parse(os.Args[1:])

	args := renameFlags.Args()
	if len(args) < 1 || len(args) > 2 {
		fmt.Fprintf(os.Stderr, "error: rename requires 1 or 2 positional arguments\n\n")
		renameFlags.Usage()
		os.Exit(1)
	}

	src := args[0]

	var newLeaf string
	if len(args) == 2 {
		newLeaf = args[1]
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

	mapper := nato_levels.MakeNATOLevelMapper()

	mover := &dagnabit.GitMover{Dir: dir, ModulePath: modulePath}

	opts := dagnabit.MoveOptions{
		DryRun:          dryRun,
		Verbose:         verbose,
		AllowTypeErrors: force,
	}

	if err := mover.RenamePackage(src, newLeaf, mapper, opts); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runReposition() {
	var dryRun bool
	var verbose bool
	var modulePath string
	var depth int
	var initial bool
	var lang string

	flag.BoolVar(&dryRun, "n", false, "show what would be moved without moving")
	flag.BoolVar(&dryRun, "dry-run", false, "show what would be moved without moving")
	flag.BoolVar(&verbose, "v", false, "enable verbose output")
	flag.BoolVar(&verbose, "verbose", false, "enable verbose output")
	flag.StringVar(&modulePath, "module", "", "Go module path (read from go.mod if empty)")
	flag.StringVar(&lang, "lang", "", "operating language: go or rust (auto-detected when empty)")
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

	detected, rootDir, err := detectLanguage(dir, lang)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	var reader dagnabit.DependencyReader
	var mover dagnabit.PackageMover

	switch detected {
	case langGo:
		// Deliberately dir (cwd), not rootDir: go mode keeps its
		// pre-detection contract of running from the module root.
		// Honoring rootDir from a subdirectory is tracked separately.
		modulePath, err = go_module.ResolveModulePath(dir, modulePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		reader = &go_list.Reader{
			Dir:             dir,
			ModulePath:      modulePath,
			PackagePrefixes: prefixes,
			ComponentDepth:  depth,
			Verbose:         verbose,
		}

		mover = &dagnabit.GitMover{Dir: dir, ModulePath: modulePath}

	case langRust:
		if modulePath != "" {
			fmt.Fprintf(os.Stderr, "error: -module is go-only; not valid with -lang rust\n")
			os.Exit(1)
		}

		reader = &cargo_metadata.Reader{
			Dir:             rootDir,
			PackagePrefixes: prefixes,
			ComponentDepth:  depth,
			Verbose:         verbose,
		}

		mover = &dagnabit_rust.Mover{WorkspaceRoot: rootDir}

	default:
		// detectLanguage never returns langUnknown with a nil error;
		// fail fast if a future language breaks that invariant.
		fmt.Fprintf(os.Stderr, "error: internal: unhandled language %d\n", detected)
		os.Exit(1)
	}

	mapper := nato_levels.MakeNATOLevelMapper()

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
	var noRewriteConsumers bool
	var library bool
	var copyMode bool
	var check bool

	exportFlags.BoolVar(&dryRun, "n", false, "show what would be generated without writing files")
	exportFlags.BoolVar(&dryRun, "dry-run", false, "show what would be generated without writing files")
	exportFlags.StringVar(&outputDir, "output-dir", "pkgs", "output directory for generated facades (relative to module root)")
	exportFlags.StringVar(&modulePath, "module", "", "Go module path (read from go.mod if empty)")
	exportFlags.BoolVar(&noRewriteConsumers, "no-rewrite-consumers", false, "skip rewriting external workspace consumers' imports to the new facade path")
	exportFlags.BoolVar(&library, "library", false, "export facades for every package under internal/ (fails if any //go:generate dagnabit export directives exist)")
	exportFlags.BoolVar(&copyMode, "copy", false, "copy internal source files into pkgs/, rewriting only intra-module imports, instead of emitting thin re-export aliases")
	exportFlags.BoolVar(&check, "check", false, "verify the committed facades match a fresh export without writing; exit nonzero on drift (works with --library, explicit packages, or directive scan)")
	exportFlags.BoolVar(&check, "c", false, "alias for --check")
	exportFlags.Parse(os.Args[1:])

	args := exportFlags.Args()

	if library && len(args) > 0 {
		fmt.Fprintf(os.Stderr, "error: --library does not accept package arguments\n")
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

	exporter := &dagnabit.Exporter{
		ModulePath:          modulePath,
		Dir:                 dir,
		OutputDir:           outputDir,
		DryRun:              dryRun,
		SkipConsumerRewrite: noRewriteConsumers,
		Copy:                copyMode,
	}

	if check {
		// Check mode renders + formats into a temp dir and compares against the
		// on-disk facades; it never writes the real tree and does its own
		// formatting, so skip the trailing in-place FormatOutput below. Mirrors
		// the export dispatch (library / explicit packages / directive scan).
		var checkErr error
		switch {
		case library:
			checkErr = exporter.CheckAll()
		case len(args) > 0:
			for _, arg := range args {
				if checkErr = exporter.CheckPackage(arg); checkErr != nil {
					break
				}
			}
		default:
			checkErr = exporter.CheckScan()
		}
		if checkErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", checkErr)
			os.Exit(1)
		}
		return
	}

	if library {
		if err := exporter.ExportAll(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	} else if len(args) > 0 {
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

	if err := exporter.FormatOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
