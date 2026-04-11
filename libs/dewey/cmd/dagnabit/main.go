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
	var dryRun bool
	var verbose bool
	var modulePath string
	var packagePrefixes string

	flag.BoolVar(&dryRun, "n", false, "show what would be moved without moving")
	flag.BoolVar(&dryRun, "dry-run", false, "show what would be moved without moving")
	flag.BoolVar(&verbose, "v", false, "enable verbose output")
	flag.BoolVar(&verbose, "verbose", false, "enable verbose output")
	flag.StringVar(&modulePath, "module", "", "Go module path (read from go.mod if empty)")
	flag.StringVar(&packagePrefixes, "prefixes", "lib,internal", "comma-separated package tree prefixes to analyze")
	flag.Parse()

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	goModPath := filepath.Join(dir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "error: must be run from a directory containing go.mod\n")
		os.Exit(1)
	}

	if modulePath == "" {
		modulePath, err = readModulePath(goModPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading module path: %v\n", err)
			os.Exit(1)
		}
	}

	var prefixes []string
	for _, p := range strings.Split(packagePrefixes, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			prefixes = append(prefixes, p)
		}
	}

	r := dagnabit.Repositioner{
		Reader: dagnabit.GoListReader{
			ModulePath:      modulePath,
			Dir:             dir,
			PackagePrefixes: prefixes,
		},
		Mapper:  dagnabit.MakeNATOLevelMapper(),
		Mover:   dagnabit.JustMover{Dir: dir},
		DryRun:  dryRun,
		Verbose: verbose,
	}

	if err := r.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
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
