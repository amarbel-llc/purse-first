// Package cargo_workspace discovers a cargo workspace root by walking up
// from a directory to the nearest Cargo.toml containing a [workspace]
// table. Rust analog of go_module's module-path discovery.
package cargo_workspace

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Workspace is a located cargo workspace.
type Workspace struct {
	// RootDir is the absolute directory containing the workspace Cargo.toml.
	RootDir string
	// Members is the [workspace] members list exactly as written
	// (relative dirs or globs).
	Members []string
}

type manifestWorkspace struct {
	Workspace *struct {
		Members []string `toml:"members"`
	} `toml:"workspace"`
}

// FindRoot walks up from dir to the nearest Cargo.toml whose contents
// include a [workspace] table. Crate manifests without [workspace] are
// skipped (the walk continues upward). Errors when the filesystem root
// is reached without finding one.
func FindRoot(dir string) (Workspace, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Workspace{}, err
	}

	for current := abs; ; {
		manifestPath := filepath.Join(current, "Cargo.toml")

		if _, err := os.Stat(manifestPath); err == nil {
			var m manifestWorkspace
			if _, err := toml.DecodeFile(manifestPath, &m); err != nil {
				return Workspace{}, fmt.Errorf("parsing %s: %w", manifestPath, err)
			}

			if m.Workspace != nil {
				return Workspace{RootDir: current, Members: m.Workspace.Members}, nil
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			return Workspace{}, fmt.Errorf(
				"no Cargo.toml with a [workspace] table found walking up from %s", abs,
			)
		}
		current = parent
	}
}
