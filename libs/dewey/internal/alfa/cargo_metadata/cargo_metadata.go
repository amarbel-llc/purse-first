// Package cargo_metadata reads a cargo workspace's internal dependency
// graph by shelling out to `cargo metadata` and returns it as a map of
// prefix to edge slice suitable for topological_sort.Sort. Rust analog of
// go_list, mirroring its node/edge semantics exactly.
package cargo_metadata

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	topological_sort "code.linenisgreat.com/purse-first/libs/dewey/internal/0/topological_sort"
)

// Reader reads cargo workspace-member dependencies by shelling out to
// `cargo metadata`. Dir is the working directory to run `cargo metadata`
// from. PackagePrefixes are directory prefixes containing crates (e.g.,
// ["internal"]). Node names are crate manifest directories relative to the
// workspace root and include the prefix (e.g., "internal/alfa/store").
//
// ComponentDepth controls how many path components identify a crate node:
//   - 3 (default): prefix/level/crate (e.g., "internal/alfa/store")
//   - 2: level/crate (e.g., "alfa/store") — for repos where NATO levels are top-level dirs
//
// When Verbose is true, sources dropped because their path has fewer than
// ComponentDepth components are logged to stderr. Independent of Verbose,
// a prefix that matched sources in `cargo metadata` output but produced
// zero edges (because every source was too short) returns an error.
type Reader struct {
	Dir             string
	PackagePrefixes []string
	ComponentDepth  int
	Verbose         bool
}

func (reader Reader) ReadDependencies() (map[string][]topological_sort.Edge, error) {
	args := []string{"metadata", "--format-version", "1", "--no-deps"}

	cmd := exec.Command("cargo", args...)
	cmd.Dir = reader.Dir

	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		var stderr []byte
		if errors.As(err, &exitErr) {
			stderr = exitErr.Stderr
		}

		return nil, fmt.Errorf(
			"cargo %s: %w: %s",
			strings.Join(args, " "),
			err,
			stderr,
		)
	}

	return parseEdges(
		out,
		reader.PackagePrefixes,
		reader.ComponentDepth,
		reader.Verbose,
	)
}

// metadata is the subset of `cargo metadata --format-version 1` output
// this package consumes.
type metadata struct {
	Packages      []metadataPackage `json:"packages"`
	WorkspaceRoot string            `json:"workspace_root"`
}

type metadataPackage struct {
	Name         string               `json:"name"`
	ManifestPath string               `json:"manifest_path"`
	Dependencies []metadataDependency `json:"dependencies"`
}

type metadataDependency struct {
	Name string `json:"name"`
	// Path is the absolute crate directory for path dependencies; empty
	// for registry dependencies.
	Path string `json:"path"`
}

func parseEdges(
	jsonBytes []byte,
	prefixes []string,
	componentDepth int,
	verbose bool,
) (map[string][]topological_sort.Edge, error) {
	var meta metadata

	if err := json.Unmarshal(jsonBytes, &meta); err != nil {
		return nil, fmt.Errorf("decoding cargo metadata output: %w", err)
	}

	// Mirrors go_list.Reader.componentDepth (go_list.go:42-48).
	depth := componentDepth
	if depth < 2 {
		depth = 3
	}

	workspaceRoot := filepath.Clean(meta.WorkspaceRoot)

	// Member crate dirs relative to the workspace root: only path-deps
	// resolving to one of these are workspace-internal edges.
	memberDirs := make(map[string]struct{}, len(meta.Packages))

	for _, pkg := range meta.Packages {
		rel, ok := relativeToWorkspace(workspaceRoot, filepath.Dir(pkg.ManifestPath))
		if !ok {
			continue
		}

		memberDirs[rel] = struct{}{}
	}

	edgesByPrefix := make(map[string][]topological_sort.Edge)

	for _, prefix := range prefixes {
		edges, err := parsePrefix(meta, workspaceRoot, memberDirs, prefix, depth, verbose)
		if err != nil {
			return nil, err
		}

		edgesByPrefix[prefix] = edges
	}

	return edgesByPrefix, nil
}

// parsePrefix mirrors go_list.Reader.readPrefix's filtering
// (go_list.go:84-169): sources outside the prefix are skipped, too-short
// sources are dropped (logged when verbose), targets outside the prefix
// are dropped (go_list.go:131 — cross-prefix edges are discarded), and
// too-short or self targets are dropped (go_list.go:140).
func parsePrefix(
	meta metadata,
	workspaceRoot string,
	memberDirs map[string]struct{},
	prefix string,
	depth int,
	verbose bool,
) ([]topological_sort.Edge, error) {
	prefixFilter := prefix + "/"
	seen := make(map[topological_sort.Edge]struct{})
	var edges []topological_sort.Edge
	matchedSources := 0
	droppedSources := 0

	for _, pkg := range meta.Packages {
		sourceRel, ok := relativeToWorkspace(workspaceRoot, filepath.Dir(pkg.ManifestPath))
		if !ok {
			continue
		}

		if !strings.HasPrefix(sourceRel, prefixFilter) {
			continue
		}

		matchedSources++

		source := trimToNComponents(sourceRel, depth)

		if source == "" {
			droppedSources++
			if verbose {
				fmt.Fprintf(
					os.Stderr,
					"dagnabit: skipping %s: only %d path components, need %d (try --depth=%d or --initial)\n",
					sourceRel,
					countComponents(sourceRel),
					depth,
					countComponents(sourceRel),
				)
			}

			continue
		}

		for _, dep := range pkg.Dependencies {
			if dep.Path == "" {
				continue
			}

			targetRel, ok := relativeToWorkspace(workspaceRoot, dep.Path)
			if !ok {
				continue
			}

			if _, ok := memberDirs[targetRel]; !ok {
				continue
			}

			if !strings.HasPrefix(targetRel, prefixFilter) {
				continue
			}

			target := trimToNComponents(targetRel, depth)

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

	// Mirrors go_list's zero-edge guard (go_list.go:159-167).
	if matchedSources > 0 && droppedSources == matchedSources {
		return nil, fmt.Errorf(
			"no edges computed for prefix %q: all %d sources under %s had fewer than %d path components (try --depth=2 or --initial for flat layouts)",
			prefix,
			droppedSources,
			prefix,
			depth,
		)
	}

	return edges, nil
}

// relativeToWorkspace returns path relative to workspaceRoot in slash
// form, reporting false when path is the root itself or outside it.
func relativeToWorkspace(workspaceRoot, path string) (string, bool) {
	rel, err := filepath.Rel(workspaceRoot, filepath.Clean(path))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", false
	}

	return filepath.ToSlash(rel), true
}

// countComponents and trimToNComponents are intentionally duplicated from
// go_list (go_list.go); hoisting two trivial functions to a shared level-0
// package was deferred. If you fix a bug here, also fix it in go_list.go.

// countComponents returns the number of slash-separated components in path.
// Empty string returns 0.
func countComponents(path string) int {
	if path == "" {
		return 0
	}

	return strings.Count(path, "/") + 1
}

// trimToNComponents returns the first n path components (e.g., n=3:
// "internal/alfa/store/sub" -> "internal/alfa/store"). Returns "" if fewer
// than n.
func trimToNComponents(path string, n int) string {
	parts := strings.SplitN(path, "/", n+1)
	if len(parts) < n {
		return ""
	}

	return strings.Join(parts[:n], "/")
}
