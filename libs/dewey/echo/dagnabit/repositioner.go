package dagnabit

import (
	"fmt"
	"sort"
	"strings"

	topological_sort "github.com/amarbel-llc/purse-first/libs/dewey/0/topological_sort"
)

// ModeReposition (default) rebalances packages that already sit in a tiered
// layout — <prefix>/<oldLevel>/<pkg> or <oldLevel>/<pkg> — into the NATO
// level dictated by their dependency height.
const ModeReposition = "reposition"

// ModeInitial inserts a NATO level segment into a flat <prefix>/<pkg> layout,
// producing <prefix>/<newLevel>/<pkg>. Every package is moved; there is no
// "already at the right level" short-circuit. Requires ComponentDepth == 2.
const ModeInitial = "initial"

// Repositioner orchestrates dependency reading, topological sorting,
// level mapping, and package moving.
//
// Mode selects the path-arithmetic strategy:
//   - ModeReposition / "" (default): rebalance a tiered layout.
//   - ModeInitial: insert a level segment into a flat layout (requires
//     ComponentDepth == 2).
//
// ComponentDepth controls node path interpretation in ModeReposition:
//   - 3 (default): prefix/level/package — move rebuilds as prefix/newLevel/package
//   - 2: level/package — move rebuilds as newLevel/package
//
// In ModeInitial, ComponentDepth MUST be 2 and nodes are interpreted as
// <prefix>/<pkg>, producing <prefix>/<newLevel>/<pkg>.
type Repositioner struct {
	Reader         DependencyReader
	Mapper         LevelMapper
	Mover          PackageMover
	DryRun         bool
	Verbose        bool
	ComponentDepth int
	Mode           string
}

func (repositioner *Repositioner) mode() string {
	if repositioner.Mode == "" {
		return ModeReposition
	}

	return repositioner.Mode
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

func (repositioner *Repositioner) runPrefix(prefix string, edges []topological_sort.Edge) error {
	heights, err := topological_sort.Sort(edges)
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
	mode := repositioner.mode()

	if mode == ModeInitial && depth != 2 {
		return fmt.Errorf(
			"mode %q requires ComponentDepth=2 (source layout <prefix>/<pkg>), got %d",
			mode,
			depth,
		)
	}

	for _, nh := range sorted {
		node := nh.node
		height := nh.height
		requiredLevel, err := repositioner.Mapper.LevelName(height)
		if err != nil {
			return fmt.Errorf("mapping height %d for %s: %w", height, node, err)
		}

		var currentLevel, dstPath string

		switch mode {
		case ModeInitial:
			parts := strings.SplitN(node, "/", 2)
			if len(parts) != 2 {
				return fmt.Errorf("initial mode: expected <prefix>/<pkg>, got %q", node)
			}

			// Always move — no level segment exists yet.
			dstPath = parts[0] + "/" + requiredLevel + "/" + parts[1]

		default:
			parts := strings.SplitN(node, "/", depth)

			switch depth {
			case 3:
				// prefix/level/package
				currentLevel = parts[1]
				dstPath = parts[0] + "/" + requiredLevel + "/" + parts[2]
			case 2:
				// level/package
				currentLevel = parts[0]
				dstPath = requiredLevel + "/" + parts[1]
			}

			if currentLevel == requiredLevel {
				continue
			}
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
