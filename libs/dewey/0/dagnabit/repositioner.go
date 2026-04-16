package dagnabit

import (
	"fmt"
	"sort"
	"strings"
)

// Repositioner orchestrates dependency reading, topological sorting,
// level mapping, and package moving.
//
// ComponentDepth controls node path interpretation:
//   - 3 (default): prefix/level/package — move rebuilds as prefix/newLevel/package
//   - 2: level/package — move rebuilds as newLevel/package
type Repositioner struct {
	Reader         DependencyReader
	Mapper         LevelMapper
	Mover          PackageMover
	DryRun         bool
	Verbose        bool
	ComponentDepth int
}

func (repositioner *Repositioner) componentDepth() int {
	if repositioner.ComponentDepth < 2 {
		return 3
	}

	return repositioner.ComponentDepth
}

func (repositioner *Repositioner) Run() error {
	edgesByPrefix, err := repositioner.Reader.ReadDependencies()
	if err != nil {
		return fmt.Errorf("reading dependencies: %w", err)
	}

	prefixes := make([]string, 0, len(edgesByPrefix))
	for prefix := range edgesByPrefix {
		prefixes = append(prefixes, prefix)
	}

	sort.Strings(prefixes)

	for _, prefix := range prefixes {
		if err := repositioner.runPrefix(prefix, edgesByPrefix[prefix]); err != nil {
			return err
		}
	}

	return nil
}

func (repositioner *Repositioner) runPrefix(prefix string, edges []Edge) error {
	heights, err := TopologicalSort(edges)
	if err != nil {
		return fmt.Errorf("topological sort for %s: %w", prefix, err)
	}

	type nodeHeight struct {
		node   string
		height int
	}

	sorted := make([]nodeHeight, 0, len(heights))
	for node, height := range heights {
		sorted = append(sorted, nodeHeight{node: node, height: height})
	}

	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].height != sorted[j].height {
			return sorted[i].height < sorted[j].height
		}
		return sorted[i].node < sorted[j].node
	})

	depth := repositioner.componentDepth()

	for _, nh := range sorted {
		node := nh.node
		height := nh.height
		requiredLevel, err := repositioner.Mapper.LevelName(height)
		if err != nil {
			return fmt.Errorf("mapping height %d for %s: %w", height, node, err)
		}

		parts := strings.SplitN(node, "/", depth)

		var currentLevel, packageName, dstPath string

		switch depth {
		case 3:
			// prefix/level/package
			currentLevel = parts[1]
			packageName = parts[2]
			dstPath = parts[0] + "/" + requiredLevel + "/" + packageName
		case 2:
			// level/package
			currentLevel = parts[0]
			packageName = parts[1]
			dstPath = requiredLevel + "/" + packageName
		}

		if currentLevel == requiredLevel {
			continue
		}

		if repositioner.DryRun {
			fmt.Printf("would move: %s -> %s\n", node, dstPath)
			continue
		}

		if err := repositioner.Mover.MovePackage(node, dstPath); err != nil {
			return fmt.Errorf("moving %s to %s: %w", node, dstPath, err)
		}
	}

	return nil
}
