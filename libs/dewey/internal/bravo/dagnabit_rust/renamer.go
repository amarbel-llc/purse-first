package dagnabit_rust

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	cargo_manifest "code.linenisgreat.com/purse-first/libs/dewey/internal/0/cargo_manifest"
	topological_sort "code.linenisgreat.com/purse-first/libs/dewey/internal/0/topological_sort"
	cargo_metadata "code.linenisgreat.com/purse-first/libs/dewey/internal/alfa/cargo_metadata"
)

// Options mirrors dagnabit.MoveOptions for the rust renamer.
type Options struct {
	DryRun  bool
	Verbose bool
	Force   bool // skip the cargo check pre-flight gate
}

// LevelMapper is the structural subset of dagnabit.LevelMapper this
// package needs. It is declared locally instead of importing dagnabit:
// both packages live at echo level and the dewey tier convention
// forbids echo→echo imports. nato_levels' mapper satisfies it
// structurally at the CLI layer.
type LevelMapper interface {
	LevelName(height int) (string, error)
	LevelIndex(name string) (int, error)
}

// Renamer moves and renames crates in a cargo workspace: the Mover's
// directory move plus [package] name rewrite, dependent dep-key
// renames, and ast-grep source rewrites, gated by cargo check on both
// sides.
type Renamer struct {
	// WorkspaceRoot is the cargo workspace root directory (containing
	// the [workspace] Cargo.toml). src/dst are slash-relative to it.
	WorkspaceRoot string
}

// MoveRename moves the crate at src to dst (slash-relative), renaming
// the crate when the leaf differs.
//
// Same leaf: the operation is a pure directory move, delegated to
// Mover.MovePackage (Cargo.toml-only rewrites; no cargo check gates).
//
// Different leaf: full rename — pre-flight gates (ast-grep on PATH, no
// stale facade, cargo check unless opts.Force), directory move,
// [package] name rewrite, dependent dep-key renames, ast-grep source
// rewrites over dependent crates, and a cargo check post-flight.
func (r *Renamer) MoveRename(src, dst string, opts Options) error {
	src = path.Clean(src)
	dst = path.Clean(dst)

	if path.Base(src) == path.Base(dst) {
		if opts.DryRun {
			fmt.Printf("dagnabit: would move %s -> %s\n", src, dst)

			return nil
		}

		mover := &Mover{WorkspaceRoot: r.WorkspaceRoot}

		return mover.MovePackage(src, dst)
	}

	return r.moveRenameLeaf(src, dst, opts)
}

// moveRenameLeaf is the full-rename path of MoveRename: the leafs of
// src and dst differ, so crate names and dependent sources change too.
func (r *Renamer) moveRenameLeaf(src, dst string, opts Options) error {
	oldLeaf := path.Base(src)
	newLeaf := path.Base(dst)

	// Pre-flight: every gate runs before any tree mutation.
	if _, err := exec.LookPath("ast-grep"); err != nil {
		return fmt.Errorf("ast-grep not found on PATH — required for rust mode")
	}

	// Facades are generated, not chased (v1): a facade for the old
	// leaf would silently keep re-exporting a crate name that no
	// longer exists. Fail fast while the tree is still clean.
	facadeDir := filepath.Join(r.WorkspaceRoot, "pkgs", oldLeaf)
	if _, err := os.Stat(facadeDir); err == nil {
		return fmt.Errorf(
			"facade pkgs/%s exists; facades are generated and not renamed — remove it and re-run `dagnabit export` after the rename",
			oldLeaf,
		)
	}

	// The cargo check gate is also skipped in dry-run: a dry run must
	// leave the tree byte-identical, and cargo check writes Cargo.lock
	// and target/ even when the workspace is green.
	if !opts.Force && !opts.DryRun {
		if err := r.cargoCheck(); err != nil {
			return fmt.Errorf(
				"cargo check pre-flight failed (workspace must be green before a rename; -force skips this gate): %w",
				err,
			)
		}
	}

	oldName, err := r.packageName(src)
	if err != nil {
		return err
	}

	newName := renamedPackageName(oldName, oldLeaf, newLeaf)

	meta, err := r.cargoMetadata()
	if err != nil {
		return err
	}

	oldLib := libTargetName(meta, src, oldName)
	// SetPackageName leaves no explicit [lib] name behind, so cargo
	// derives the new lib-target name from the new package name.
	newLib := rustIdentifier(newName)

	dependents, err := r.dependentCrates(meta, src, oldName, newName)
	if err != nil {
		return err
	}

	patterns := renamePatterns(oldLib, newLib)

	if opts.DryRun {
		return r.printPlan(src, dst, oldName, newName, dependents, patterns)
	}

	mover := &Mover{WorkspaceRoot: r.WorkspaceRoot}

	if err := mover.MovePackage(src, dst); err != nil {
		return err
	}

	if err := r.setPackageName(dst, newName); err != nil {
		return err
	}

	if opts.Verbose {
		fmt.Fprintf(
			os.Stderr,
			"dagnabit: renamed package %s -> %s\n", oldName, newName,
		)
	}

	for _, dep := range dependents {
		if err := r.renameDepKey(dep, oldName, newName); err != nil {
			return err
		}
	}

	rewrites, err := r.rewriteDependentSources(dependents, patterns, opts)
	if err != nil {
		return err
	}

	if err := r.cargoCheck(); err != nil {
		return fmt.Errorf(
			"cargo check post-flight failed after rename (%s; tree left dirty for inspection — recover from a clean git start): %w",
			rewrites,
			err,
		)
	}

	return nil
}

// renamedPackageName computes the renamed crate's [package] name.
// The `_internal` suffix convention is preserved exactly
// (oldLeaf+"_internal" → newLeaf+"_internal"); other names replace the
// first occurrence of oldLeaf, falling back to newLeaf alone when the
// old name does not contain the old leaf at all.
func renamedPackageName(oldName, oldLeaf, newLeaf string) string {
	if oldName == oldLeaf+"_internal" {
		return newLeaf + "_internal"
	}

	if strings.Contains(oldName, oldLeaf) {
		return strings.Replace(oldName, oldLeaf, newLeaf, 1)
	}

	return newLeaf
}

// rustIdentifier converts a package name to the identifier cargo
// derives from it (dashes become underscores).
func rustIdentifier(name string) string {
	return strings.ReplaceAll(name, "-", "_")
}

// packageName reads the [package] name from the crate manifest at
// crateDir (slash-relative) with full TOML semantics.
func (r *Renamer) packageName(crateDir string) (string, error) {
	manifestPath := filepath.Join(
		r.WorkspaceRoot, filepath.FromSlash(crateDir), "Cargo.toml",
	)

	var manifest crateManifest
	if _, err := toml.DecodeFile(manifestPath, &manifest); err != nil {
		return "", fmt.Errorf("parsing %s: %w", manifestPath, err)
	}

	if manifest.Package.Name == "" {
		return "", fmt.Errorf("%s has no [package] name", manifestPath)
	}

	return manifest.Package.Name, nil
}

// setPackageName rewrites the [package] name in the crate manifest at
// crateDir (slash-relative), preserving comments and spacing.
func (r *Renamer) setPackageName(crateDir, newName string) error {
	manifestPath := filepath.Join(
		r.WorkspaceRoot, filepath.FromSlash(crateDir), "Cargo.toml",
	)

	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", manifestPath, err)
	}

	out, err := cargo_manifest.SetPackageName(manifest, newName)
	if err != nil {
		return fmt.Errorf("renaming package in %s: %w", manifestPath, err)
	}

	return writePreservingMode(manifestPath, out)
}

// dependentCrate is one workspace member whose manifest declares a
// dependency key on the renamed crate.
type dependentCrate struct {
	// relDir is the crate directory slash-relative to the workspace
	// root (the crate's PRE-move location; dependents never move).
	relDir string

	// depKeyCount is how many dep-key lines RenameDepKey would rewrite.
	depKeyCount int
}

// dependentCrates scans every workspace member except the renamed crate
// itself for dependency keys on oldName. The scan is pure (RenameDepKey
// on in-memory bytes); nothing is written.
func (r *Renamer) dependentCrates(
	meta cargoMeta,
	src, oldName, newName string,
) ([]dependentCrate, error) {
	var dependents []dependentCrate

	for _, member := range meta.memberDirs() {
		if member == src {
			continue
		}

		manifestPath := filepath.Join(
			r.WorkspaceRoot, filepath.FromSlash(member), "Cargo.toml",
		)

		manifest, err := os.ReadFile(manifestPath)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", manifestPath, err)
		}

		_, n, err := cargo_manifest.RenameDepKey(manifest, oldName, newName)
		if err != nil {
			return nil, fmt.Errorf("scanning %s: %w", manifestPath, err)
		}

		if n == 0 {
			continue
		}

		dependents = append(dependents, dependentCrate{relDir: member, depKeyCount: n})
	}

	return dependents, nil
}

// renameDepKey applies the oldName→newName dep-key rename to the
// dependent's manifest on disk.
func (r *Renamer) renameDepKey(dep dependentCrate, oldName, newName string) error {
	manifestPath := filepath.Join(
		r.WorkspaceRoot, filepath.FromSlash(dep.relDir), "Cargo.toml",
	)

	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", manifestPath, err)
	}

	out, n, err := cargo_manifest.RenameDepKey(manifest, oldName, newName)
	if err != nil {
		return fmt.Errorf("renaming dep key in %s: %w", manifestPath, err)
	}

	if n == 0 {
		return nil
	}

	return writePreservingMode(manifestPath, out)
}

// rewriteDependentSources applies the ast-grep pattern set to every
// dependent crate directory (never the moved crate) and returns a
// human-readable rewrite summary for the post-flight error path. Each
// pattern runs a dry counting pass first — ast-grep cannot combine
// `--update-all` with JSON match output — then the write pass.
func (r *Renamer) rewriteDependentSources(
	dependents []dependentCrate,
	patterns []patternPair,
	opts Options,
) (string, error) {
	totalMatches := 0

	for _, dep := range dependents {
		dirAbs := filepath.Join(r.WorkspaceRoot, filepath.FromSlash(dep.relDir))

		for _, p := range patterns {
			matches, err := runAstGrep(dirAbs, p, true)
			if err != nil {
				return "", err
			}

			if matches == 0 {
				continue
			}

			if _, err := runAstGrep(dirAbs, p, false); err != nil {
				return "", err
			}

			totalMatches += matches

			if opts.Verbose {
				fmt.Fprintf(
					os.Stderr,
					"dagnabit: pattern %q -> %q: %d matches in %s\n",
					p.pattern, p.rewrite, matches, dep.relDir,
				)
			}
		}
	}

	// Scope note in the summary: only crates with a DIRECT dep key on
	// the renamed crate are rewritten. A crate referencing the old lib
	// name through a transitive re-export (pub use) is not touched and
	// surfaces in the post-flight cargo output instead — say so, or the
	// summary misdirects exactly when that is the failure cause.
	return fmt.Sprintf(
		"rewrote %d source occurrence(s) in %d direct-dep crate(s); references via transitive re-exports, if any, appear in the cargo output below",
		totalMatches, len(dependents),
	), nil
}

// printPlan emits the dry-run plan: the move, the package rename, the
// dep-key renames, and per-pattern ast-grep match counts over each
// dependent crate. Nothing is written.
func (r *Renamer) printPlan(
	src, dst, oldName, newName string,
	dependents []dependentCrate,
	patterns []patternPair,
) error {
	fmt.Printf("dagnabit: would move %s -> %s\n", src, dst)
	fmt.Printf("dagnabit: would rename package %s -> %s\n", oldName, newName)

	for _, dep := range dependents {
		fmt.Printf(
			"dagnabit: would rename %d dep key(s) %s -> %s in %s/Cargo.toml\n",
			dep.depKeyCount, oldName, newName, dep.relDir,
		)

		dirAbs := filepath.Join(r.WorkspaceRoot, filepath.FromSlash(dep.relDir))

		for _, p := range patterns {
			matches, err := runAstGrep(dirAbs, p, true)
			if err != nil {
				return err
			}

			fmt.Printf(
				"dagnabit: pattern %q -> %q: %d matches in %s\n",
				p.pattern, p.rewrite, matches, dep.relDir,
			)
		}
	}

	return nil
}

// cargoCheck gates the rename on `cargo check --workspace`, embedding
// argv and cargo's stderr in the error.
func (r *Renamer) cargoCheck() error {
	args := []string{"check", "--workspace"}

	cmd := exec.Command("cargo", args...)
	cmd.Dir = r.WorkspaceRoot

	if _, err := cmd.Output(); err != nil {
		var exitErr *exec.ExitError
		var stderr []byte

		if errors.As(err, &exitErr) {
			stderr = exitErr.Stderr
		}

		return fmt.Errorf(
			"cargo %s (in %s): %w: %s",
			strings.Join(args, " "),
			r.WorkspaceRoot,
			err,
			stderr,
		)
	}

	return nil
}

// cargoMeta is the subset of `cargo metadata --format-version 1`
// output the renamer consumes. cargo_metadata (alfa) deliberately
// exposes only prefix-trimmed dependency edges, so target names and
// manifest paths are decoded locally here.
type cargoMeta struct {
	Packages []cargoMetaPackage `json:"packages"`
	Root     string             `json:"workspace_root"`
}

type cargoMetaPackage struct {
	Name         string            `json:"name"`
	ManifestPath string            `json:"manifest_path"`
	Targets      []cargoMetaTarget `json:"targets"`
}

type cargoMetaTarget struct {
	Kind []string `json:"kind"`
	Name string   `json:"name"`
}

// memberDirs lists member crate directories slash-relative to the
// metadata-reported workspace root (which is symlink-resolved; manifest
// paths are resolved against it, never against the caller-supplied
// WorkspaceRoot).
func (meta cargoMeta) memberDirs() []string {
	dirs := make([]string, 0, len(meta.Packages))

	for _, pkg := range meta.Packages {
		rel, err := filepath.Rel(
			filepath.Clean(meta.Root), filepath.Dir(pkg.ManifestPath),
		)
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			continue
		}

		dirs = append(dirs, filepath.ToSlash(rel))
	}

	return dirs
}

// cargoMetadata runs `cargo metadata --format-version 1 --no-deps` in
// the workspace root and decodes the package/target subset.
func (r *Renamer) cargoMetadata() (cargoMeta, error) {
	args := []string{"metadata", "--format-version", "1", "--no-deps"}

	cmd := exec.Command("cargo", args...)
	cmd.Dir = r.WorkspaceRoot

	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		var stderr []byte

		if errors.As(err, &exitErr) {
			stderr = exitErr.Stderr
		}

		return cargoMeta{}, fmt.Errorf(
			"cargo %s (in %s): %w: %s",
			strings.Join(args, " "),
			r.WorkspaceRoot,
			err,
			stderr,
		)
	}

	var meta cargoMeta
	if err := json.Unmarshal(out, &meta); err != nil {
		return cargoMeta{}, fmt.Errorf("decoding cargo metadata output: %w", err)
	}

	return meta, nil
}

// libTargetName returns the crate's lib-target name from cargo
// metadata (kind contains "lib"), normalized to identifier form. Falls
// back to the package name when the crate has no lib target on record.
func libTargetName(meta cargoMeta, crateDir, packageName string) string {
	for _, pkg := range meta.Packages {
		rel, err := filepath.Rel(
			filepath.Clean(meta.Root), filepath.Dir(pkg.ManifestPath),
		)
		if err != nil || filepath.ToSlash(rel) != crateDir {
			continue
		}

		for _, target := range pkg.Targets {
			for _, kind := range target.Kind {
				if kind == "lib" {
					return rustIdentifier(target.Name)
				}
			}
		}
	}

	return rustIdentifier(packageName)
}

// Rename repositions ONE crate to the NATO level dictated by its
// transitive in-workspace dependencies, optionally renaming its leaf.
// src must be a 3-component <prefix>/<level>/<leaf> path. Mirrors
// dagnabit.GitMover.RenamePackage: the crate's required level is its
// height in the dependency subgraph reachable from it (height 0 = no
// internal deps), converted to a name by the mapper. Other crates are
// NOT moved.
func (r *Renamer) Rename(
	src, newLeaf string,
	mapper LevelMapper,
	opts Options,
) error {
	src = path.Clean(src)

	parts := strings.Split(src, "/")
	if len(parts) != 3 {
		return fmt.Errorf(
			"src %q is not a 3-component <prefix>/<level>/<leaf> path",
			src,
		)
	}

	prefix := parts[0]
	leaf := parts[2]

	if newLeaf == "" {
		newLeaf = leaf
	}

	requiredLevel, err := r.computeRequiredLevel(src, prefix, mapper)
	if err != nil {
		return fmt.Errorf("computing required level for %s: %w", src, err)
	}

	dst := prefix + "/" + requiredLevel + "/" + newLeaf

	if dst == src {
		if opts.Verbose || opts.DryRun {
			fmt.Fprintf(
				os.Stderr,
				"dagnabit: %s already at level %q with leaf %q; nothing to do\n",
				src, requiredLevel, newLeaf,
			)
		}

		return nil
	}

	return r.MoveRename(src, dst, opts)
}

// computeRequiredLevel reads the workspace dependency edges for prefix,
// restricts them to the subgraph reachable from src, and converts src's
// topological height to a level name. Mirrors dagnabit's
// computeRequiredLevel (rename.go) with cargo metadata edges in place
// of packages.Load imports.
func (r *Renamer) computeRequiredLevel(
	src, prefix string,
	mapper LevelMapper,
) (string, error) {
	reader := cargo_metadata.Reader{
		Dir:             r.WorkspaceRoot,
		PackagePrefixes: []string{prefix},
	}

	edgesByPrefix, err := reader.ReadDependencies()
	if err != nil {
		return "", err
	}

	adjacency := make(map[string][]string)

	for _, edge := range edgesByPrefix[prefix] {
		adjacency[edge.Source] = append(adjacency[edge.Source], edge.Target)
	}

	// Walk transitively from src, building the constrained edge set.
	var edges []topological_sort.Edge
	visited := make(map[string]bool)

	var walk func(node string)
	walk = func(node string) {
		if visited[node] {
			return
		}
		visited[node] = true

		for _, target := range adjacency[node] {
			edges = append(edges, topological_sort.Edge{Source: node, Target: target})
			walk(target)
		}
	}

	walk(src)

	heights, err := topological_sort.Sort(edges)
	if err != nil {
		return "", fmt.Errorf("topological sort: %w", err)
	}

	height, ok := heights[src]
	if !ok {
		// src has no in-workspace deps; it sits at height 0.
		height = 0
	}

	return mapper.LevelName(height)
}
