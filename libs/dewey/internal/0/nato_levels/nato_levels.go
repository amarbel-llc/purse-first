// Package nato_levels maps non-negative integer heights to NATO phonetic
// level names ("0", "alfa", "bravo", ... "zulu"), used by dewey tooling to
// tier Go packages by dependency depth.
package nato_levels

import "fmt"

var natoLevels = []string{
	"0",
	"alfa",
	"bravo",
	"charlie",
	"delta",
	"echo",
	"foxtrot",
	"golf",
	"hotel",
	"india",
	"juliett",
	"kilo",
	"lima",
	"mike",
	"november",
	"oscar",
	"papa",
	"quebec",
	"romeo",
	"sierra",
	"tango",
	"uniform",
	"victor",
	"whiskey",
	"xray",
	"yankee",
	"zulu",
}

type NATOLevelMapper struct{}

func MakeNATOLevelMapper() NATOLevelMapper {
	return NATOLevelMapper{}
}

func (m NATOLevelMapper) LevelName(height int) (string, error) {
	if height < 0 || height >= len(natoLevels) {
		return "", fmt.Errorf(
			"height %d out of range (max %d)",
			height,
			len(natoLevels)-1,
		)
	}

	return natoLevels[height], nil
}

// LevelIndex returns the inverse of LevelName: the height (0-based index)
// for the given level name, or an error if the name is not a NATO level.
func (m NATOLevelMapper) LevelIndex(name string) (int, error) {
	for i, n := range natoLevels {
		if n == name {
			return i, nil
		}
	}

	return -1, fmt.Errorf("unknown NATO level %q", name)
}
