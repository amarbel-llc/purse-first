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
	"arrow", "badge", "bloom", "brook", "cedar",
	"cliff", "cloud", "coral", "crane", "creek",
	"crown", "delta", "ember", "fable", "fern",
	"finch", "flame", "frost", "glade", "grove",
	"haven", "heron", "ivory", "jewel", "larch",
	"lemon", "lily", "maple", "marsh", "moon",
	"oak", "olive", "otter", "pearl", "pine",
	"plume", "pond", "quail", "ridge", "river",
	"robin", "sage", "shore", "spark", "spire",
	"stone", "storm", "trail", "vine", "wolf",
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
