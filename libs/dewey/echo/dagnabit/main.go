package dagnabit

import (
	topological_sort "github.com/amarbel-llc/purse-first/libs/dewey/0/topological_sort"
)

// DependencyReader produces directed edges from a codebase,
// grouped by tree prefix (e.g., "lib", "internal").
type DependencyReader interface {
	ReadDependencies() (map[string][]topological_sort.Edge, error)
}

// LevelMapper assigns names to topological heights.
// Height 0 is the lowest level (no internal dependencies).
type LevelMapper interface {
	LevelName(height int) (string, error)
}

// PackageMover executes the move of a package from one path to another.
type PackageMover interface {
	MovePackage(src, dst string) error
}
