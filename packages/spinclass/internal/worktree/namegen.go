package worktree

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
)

var adjectives = []string{
	"bold", "brave", "bright", "calm", "clear",
	"cool", "crisp", "deft", "eager", "fair",
	"fast", "firm", "fond", "free", "fresh",
	"glad", "grand", "green", "keen", "kind",
	"light", "live", "loud", "lucid", "merry",
	"mild", "neat", "noble", "plain", "prime",
	"proud", "pure", "quick", "quiet", "rapid",
	"rare", "ready", "rich", "sharp", "sleek",
	"slim", "smart", "smooth", "snug", "solid",
	"stark", "still", "sunny", "swift", "vivid",
}

var nouns = []string{
	"alder", "aspen", "beech", "birch", "cedar",
	"cherry", "chestnut", "cypress", "elder", "elm",
	"fir", "hazel", "hemlock", "hickory", "holly",
	"juniper", "larch", "laurel", "linden", "locust",
	"magnolia", "mahogany", "maple", "mulberry", "myrtle",
	"oak", "olive", "palm", "pecan", "pine",
	"plum", "poplar", "redwood", "rowan", "sequoia",
	"spruce", "sumac", "sycamore", "teak", "walnut",
	"willow", "yew", "acacia", "banyan", "baobab",
	"buckeye", "catalpa", "dogwood", "ebony", "fig",
}

// RandomName generates a random adjective-noun name that does not collide
// with existing directories in <repoPath>/.worktrees/.
func RandomName(repoPath string) string {
	wtDir := filepath.Join(repoPath, WorktreesDir)
	for {
		candidate := fmt.Sprintf(
			"%s-%s",
			adjectives[rand.IntN(len(adjectives))],
			nouns[rand.IntN(len(nouns))],
		)
		_, err := os.Stat(filepath.Join(wtDir, candidate))
		if os.IsNotExist(err) {
			return candidate
		}
	}
}
