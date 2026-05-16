// Package go_list reads a Go module's internal import graph by shelling
// out to `go list` and returns it as a map of prefix to edge slice suitable
// for topological_sort.Sort.
package go_list

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	topological_sort "github.com/amarbel-llc/purse-first/libs/dewey/internal/0/topological_sort"
)

// TODO: refactor to use golang.org/x/tools/go/packages for direct
// programmatic access to the import graph instead of shelling out.

// Reader reads Go package dependencies by shelling out to `go list`.
// ModulePath is the Go module path (e.g., "code.linenisgreat.com/dodder/go").
// Dir is the working directory to run `go list` from.
// PackagePrefixes are directory prefixes containing packages (e.g., ["lib", "internal"]).
// Node names in returned edges include the prefix (e.g., "lib/_/ohio_buffer").
//
// ComponentDepth controls how many path components identify a package node:
//   - 3 (default): prefix/level/package (e.g., "lib/alfa/errors")
//   - 2: level/package (e.g., "alfa/errors") — for repos where NATO levels are top-level dirs
//
// When Verbose is true, sources dropped because their path has fewer than
// ComponentDepth components are logged to stderr. Independent of Verbose,
// a prefix that matched sources in `go list` output but produced zero edges
// (because every source was too short) returns an error.
type Reader struct {
	ModulePath      string
	Dir             string
	PackagePrefixes []string
	ComponentDepth  int
	Verbose         bool
}

func (reader Reader) componentDepth() int {
	if reader.ComponentDepth < 2 {
		return 3
	}

	return reader.ComponentDepth
}

func (reader Reader) ReadDependencies() (map[string][]topological_sort.Edge, error) {
	edgesByPrefix := make(map[string][]topological_sort.Edge)

	for _, prefix := range reader.PackagePrefixes {
		edges, err := reader.readPrefix(prefix)
		if err != nil {
			return nil, err
		}

		edgesByPrefix[prefix] = edges
	}

	return edgesByPrefix, nil
}

func (reader Reader) readPrefix(prefix string) ([]topological_sort.Edge, error) {
	patterns, err := listPatternsForPrefix(reader.Dir, prefix)
	if err != nil {
		return nil, err
	}

	args := append(
		[]string{"list", "-f", `{{.ImportPath}}{{"\t"}}{{range .Imports}}{{.}} {{end}}`},
		patterns...,
	)

	cmd := exec.Command("go", args...)
	cmd.Dir = reader.Dir

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list %s: %w", prefix, err)
	}

	depth := reader.componentDepth()
	prefixFilter := reader.ModulePath + "/" + prefix + "/"
	seen := make(map[topological_sort.Edge]struct{})
	var edges []topological_sort.Edge
	matchedSources := 0
	droppedSources := 0

	scanner := bufio.NewScanner(strings.NewReader(string(out)))

	for scanner.Scan() {
		line := scanner.Text()

		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}

		sourceFull := parts[0]

		if !strings.HasPrefix(sourceFull, prefixFilter) {
			continue
		}

		matchedSources++

		sourceRel := strings.TrimPrefix(sourceFull, reader.ModulePath+"/")
		source := trimToNComponents(sourceRel, depth)

		if source == "" {
			droppedSources++
			if reader.Verbose {
				fmt.Fprintf(
					os.Stderr,
					"dagnabit: skipping %s: only %d path components, need %d (try --depth=%d or --initial)\n",
					sourceFull,
					countComponents(sourceRel),
					depth,
					countComponents(sourceRel),
				)
			}

			continue
		}

		imports := strings.Fields(parts[1])

		for _, imp := range imports {
			if !strings.HasPrefix(imp, prefixFilter) {
				continue
			}

			target := trimToNComponents(
				strings.TrimPrefix(imp, reader.ModulePath+"/"),
				depth,
			)

			if target == "" || target == source {
				continue
			}

			edge := topological_sort.Edge{Source: source, Target: target}

			if _, ok := seen[edge]; ok {
				continue
			}

			seen[edge] = struct{}{}
			edges = append(edges, edge)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if matchedSources > 0 && droppedSources == matchedSources {
		return nil, fmt.Errorf(
			"no edges computed for prefix %q: all %d sources under %s had fewer than %d path components (try --depth=2 or --initial for flat layouts)",
			prefix,
			droppedSources,
			strings.TrimSuffix(prefixFilter, "/"),
			depth,
		)
	}

	return edges, nil
}

// countComponents returns the number of slash-separated components in path.
// Empty string returns 0.
func countComponents(path string) int {
	if path == "" {
		return 0
	}

	return strings.Count(path, "/") + 1
}

// listPatternsForPrefix returns go list patterns that cover all packages
// under prefix, including _-prefixed directories that go list's ...
// wildcard skips by convention.
func listPatternsForPrefix(dir, prefix string) ([]string, error) {
	patterns := []string{fmt.Sprintf("./%s/...", prefix)}

	prefixDir := filepath.Join(dir, prefix)

	topEntries, err := os.ReadDir(prefixDir)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", prefixDir, err)
	}

	for _, topEntry := range topEntries {
		name := topEntry.Name()
		if !topEntry.IsDir() || !strings.HasPrefix(name, "_") {
			continue
		}

		underscoreDir := filepath.Join(prefixDir, name)

		subEntries, err := os.ReadDir(underscoreDir)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", underscoreDir, err)
		}

		for _, subEntry := range subEntries {
			if !subEntry.IsDir() {
				continue
			}

			patterns = append(
				patterns,
				fmt.Sprintf("./%s/%s/%s", prefix, name, subEntry.Name()),
			)
		}
	}

	return patterns, nil
}

// trimToNComponents returns the first n path components (e.g., n=3:
// "lib/alfa/errors/context" -> "lib/alfa/errors"). Returns "" if fewer than n.
func trimToNComponents(path string, n int) string {
	parts := strings.SplitN(path, "/", n+1)
	if len(parts) < n {
		return ""
	}

	return strings.Join(parts[:n], "/")
}
