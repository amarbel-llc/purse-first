// Package dagnabit_rust implements dagnabit's PackageMover for cargo
// workspaces. Rust analog of dagnabit's GitMover: git-mv the crate
// directory, rewrite the relative path-dependencies the move
// invalidated, update the [workspace] members entry, and gate on
// `cargo metadata` resolving the workspace afterwards.
package dagnabit_rust

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	cargo_manifest "code.linenisgreat.com/purse-first/libs/dewey/internal/0/cargo_manifest"
	cargo_workspace "code.linenisgreat.com/purse-first/libs/dewey/internal/0/cargo_workspace"
)

// pathDepRE finds candidate `path = "…"` values anywhere in a manifest.
// It deliberately over-collects (non-dependency sections included);
// cargo_manifest.RewritePathDeps only rewrites dependency sections, so
// false candidates are harmless no-ops.
var pathDepRE = regexp.MustCompile(`(?:^|[\s{])path\s*=\s*"([^"]+)"`)

// Mover implements dagnabit.PackageMover for cargo workspaces: git-mv
// the crate directory, then rewrite every relative path-dependency in
// the workspace that the move invalidated (references TO the moved
// crate from other members, and the moved crate's OWN path-deps whose
// relative base changed), plus the [workspace] members entry. No .rs
// file is touched — crate names do not change in a pure move.
type Mover struct {
	// WorkspaceRoot is the cargo workspace root directory (containing
	// the [workspace] Cargo.toml). src/dst given to MovePackage are
	// relative to it.
	WorkspaceRoot string
}

// MovePackage moves the crate at src to dst (both slash-relative to
// WorkspaceRoot), rewrites invalidated path-deps and the [workspace]
// members entry, then verifies the workspace still resolves via
// `cargo metadata`.
func (m *Mover) MovePackage(src, dst string) error {
	if err := m.gitMove(src, dst); err != nil {
		return err
	}

	workspace, err := cargo_workspace.FindRoot(m.WorkspaceRoot)
	if err != nil {
		return fmt.Errorf("locating cargo workspace: %w", err)
	}

	manifestDirs, err := m.manifestDirs(workspace, dst)
	if err != nil {
		return err
	}

	for _, relDir := range manifestDirs {
		if err := m.rewriteManifest(workspace.RootDir, relDir, src, dst); err != nil {
			return err
		}
	}

	if err := m.replaceMember(workspace.RootDir, src, dst); err != nil {
		return err
	}

	if err := m.verifyWorkspace(workspace.RootDir); err != nil {
		return err
	}

	return nil
}

// gitMove mirrors dagnabit.GitMover.gitMove: create dst's parent dirs,
// then run git mv in the workspace root.
func (m *Mover) gitMove(src, dst string) error {
	dstAbs := filepath.Join(m.WorkspaceRoot, filepath.FromSlash(dst))
	parentDir := filepath.Dir(dstAbs)

	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", parentDir, err)
	}

	cmd := exec.Command("git", "mv", src, dst)
	cmd.Dir = m.WorkspaceRoot

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git mv %s %s: %w: %s", src, dst, err, out)
	}

	return nil
}

// manifestDirs enumerates manifest directories (slash-relative to the
// workspace root; "." for the root manifest) from the POST-move
// filesystem state: the members list still names src, whose glob
// expansion now matches nothing, so dst is added explicitly.
func (m *Mover) manifestDirs(
	workspace cargo_workspace.Workspace,
	dst string,
) ([]string, error) {
	seen := map[string]struct{}{".": {}}
	dirs := []string{"."}

	add := func(relDir string) {
		if _, ok := seen[relDir]; ok {
			return
		}

		seen[relDir] = struct{}{}
		dirs = append(dirs, relDir)
	}

	for _, member := range workspace.Members {
		matches, err := filepath.Glob(
			filepath.Join(workspace.RootDir, filepath.FromSlash(member)),
		)
		if err != nil {
			return nil, fmt.Errorf("expanding member glob %q: %w", member, err)
		}

		for _, match := range matches {
			if _, err := os.Stat(filepath.Join(match, "Cargo.toml")); err != nil {
				continue
			}

			rel, err := filepath.Rel(workspace.RootDir, match)
			if err != nil {
				return nil, err
			}

			add(filepath.ToSlash(rel))
		}
	}

	if _, err := os.Stat(
		filepath.Join(workspace.RootDir, filepath.FromSlash(dst), "Cargo.toml"),
	); err == nil {
		add(dst)
	}

	sort.Strings(dirs[1:]) // keep "." first; rest deterministic

	return dirs, nil
}

// rewriteManifest recomputes every relative path-dep in the manifest at
// relDir that the src→dst move invalidated. A dep's PRE-move resolution
// is computed from the manifest's PRE-move directory (src-based when
// the manifest itself moved); when that resolution points into the old
// src tree, the target moved to the dst tree. The new value is the
// relative path from the manifest's NEW directory to the target's NEW
// location.
func (m *Mover) rewriteManifest(rootDir, relDir, src, dst string) error {
	manifestPath := filepath.Join(rootDir, filepath.FromSlash(relDir), "Cargo.toml")

	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", manifestPath, err)
	}

	preMoveRelDir, _ := swapPrefix(relDir, dst, src)

	rewrites, err := pathDepRewrites(string(manifest), relDir, preMoveRelDir, src, dst)
	if err != nil {
		return err
	}

	if len(rewrites) == 0 {
		return nil
	}

	out, changed, err := applyRewrites(manifest, rewrites)
	if err != nil {
		return fmt.Errorf("rewriting path deps in %s: %w", manifestPath, err)
	}

	if !changed {
		return nil
	}

	return writePreservingMode(manifestPath, out)
}

// pathDepRewrite is one exact-string path-dep replacement.
type pathDepRewrite struct {
	old string
	new string
}

// pathDepRewrites scans manifest text for relative `path = "…"` values
// and returns the (old, new) pairs whose value must change, sorted by
// old value for determinism.
func pathDepRewrites(
	manifestText, relDir, preMoveRelDir, src, dst string,
) ([]pathDepRewrite, error) {
	candidates := map[string]struct{}{}

	for _, match := range pathDepRE.FindAllStringSubmatch(manifestText, -1) {
		if p := match[1]; !filepath.IsAbs(p) {
			candidates[p] = struct{}{}
		}
	}

	var rewrites []pathDepRewrite

	for old := range candidates {
		preTarget := filepath.ToSlash(filepath.Clean(
			filepath.Join(filepath.FromSlash(preMoveRelDir), filepath.FromSlash(old)),
		))

		newTarget, _ := swapPrefix(preTarget, src, dst)

		newRel, err := filepath.Rel(
			filepath.FromSlash(relDir),
			filepath.FromSlash(newTarget),
		)
		if err != nil {
			return nil, fmt.Errorf(
				"computing path-dep %q relative to %s: %w", old, relDir, err,
			)
		}

		if newVal := filepath.ToSlash(newRel); newVal != old {
			rewrites = append(rewrites, pathDepRewrite{old: old, new: newVal})
		}
	}

	sort.Slice(rewrites, func(i, j int) bool {
		return rewrites[i].old < rewrites[j].old
	})

	return rewrites, nil
}

// applyRewrites applies rewrites in two phases through unique
// placeholders so a rewrite's new value can never be re-rewritten when
// it collides with another rewrite's old value (possible when the
// manifest itself moved and both base and targets changed).
func applyRewrites(
	manifest []byte,
	rewrites []pathDepRewrite,
) (out []byte, changed bool, err error) {
	out = manifest

	for i, rewrite := range rewrites {
		placeholder := fmt.Sprintf("\x00dagnabit-rust-%d\x00", i)

		var n int

		out, n, err = cargo_manifest.RewritePathDeps(out, rewrite.old, placeholder)
		if err != nil {
			return nil, false, err
		}

		if n == 0 {
			continue
		}

		changed = true

		out, _, err = cargo_manifest.RewritePathDeps(out, placeholder, rewrite.new)
		if err != nil {
			return nil, false, err
		}
	}

	return out, changed, nil
}

// replaceMember swaps the [workspace] members entry src→dst in the root
// manifest. A missing src entry is not an error: the member may be
// covered by a glob entry that also covers dst, and a genuinely stale
// members list fails the cargo metadata gate with cargo's own
// diagnostic.
func (m *Mover) replaceMember(rootDir, src, dst string) error {
	manifestPath := filepath.Join(rootDir, "Cargo.toml")

	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", manifestPath, err)
	}

	out, replaced, err := cargo_manifest.ReplaceMember(manifest, src, dst)
	if err != nil {
		return fmt.Errorf("replacing workspace member in %s: %w", manifestPath, err)
	}

	if !replaced {
		return nil
	}

	return writePreservingMode(manifestPath, out)
}

// verifyWorkspace gates the move on `cargo metadata` resolving the
// workspace, returning cargo's stderr on failure.
func (m *Mover) verifyWorkspace(rootDir string) error {
	args := []string{"metadata", "--format-version", "1", "--no-deps"}

	cmd := exec.Command("cargo", args...)
	cmd.Dir = rootDir

	if _, err := cmd.Output(); err != nil {
		var exitErr *exec.ExitError
		var stderr []byte

		if errors.As(err, &exitErr) {
			stderr = exitErr.Stderr
		}

		return fmt.Errorf(
			"workspace no longer resolves after move: cargo %s: %w: %s",
			strings.Join(args, " "),
			err,
			stderr,
		)
	}

	return nil
}

// swapPrefix replaces oldPrefix at the start of slash-relative rel with
// newPrefix, matching only on whole path components. Reports whether a
// swap happened.
func swapPrefix(rel, oldPrefix, newPrefix string) (string, bool) {
	if rel == oldPrefix {
		return newPrefix, true
	}

	if strings.HasPrefix(rel, oldPrefix+"/") {
		return newPrefix + strings.TrimPrefix(rel, oldPrefix), true
	}

	return rel, false
}

// writePreservingMode writes data to path keeping the file's existing
// permission bits.
func writePreservingMode(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	if err := os.WriteFile(path, data, info.Mode().Perm()); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}
