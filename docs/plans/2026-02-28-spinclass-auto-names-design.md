# Spinclass Auto-Generated Session Names — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Allow `spinclass new` to generate a random human-readable session name when no target is specified.

**Architecture:** Add `RandomName()` to `internal/worktree/` alongside `ForkName()`. Both are collision-avoiding name generators. The CLI changes `MinimumNArgs(1)` to `MinimumNArgs(0)` and calls `RandomName()` when no target is given.

**Tech Stack:** Go, `math/rand/v2`, filesystem-based collision detection.

---

### Task 1: Add RandomName with word lists and collision avoidance

**Files:**
- Create: `packages/spinclass/internal/worktree/namegen.go`
- Test: `packages/spinclass/internal/worktree/namegen_test.go`

**Step 1: Write the failing tests**

Create `packages/spinclass/internal/worktree/namegen_test.go`:

```go
package worktree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRandomNameFormat(t *testing.T) {
	repoPath := t.TempDir()

	name := RandomName(repoPath)
	parts := strings.SplitN(name, "-", 2)
	if len(parts) != 2 {
		t.Fatalf("RandomName() = %q, want adjective-noun format", name)
	}
	if parts[0] == "" || parts[1] == "" {
		t.Fatalf("RandomName() = %q, has empty adjective or noun", name)
	}
}

func TestRandomNameAvoidsCollision(t *testing.T) {
	repoPath := t.TempDir()
	wtDir := filepath.Join(repoPath, WorktreesDir)
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Generate a name, then create a directory with that name to force collision
	first := RandomName(repoPath)
	if err := os.Mkdir(filepath.Join(wtDir, first), 0o755); err != nil {
		t.Fatal(err)
	}

	// Generate many names — none should equal the taken name
	for range 100 {
		name := RandomName(repoPath)
		if name == first {
			t.Fatalf("RandomName() returned colliding name %q", name)
		}
	}
}

func TestRandomNameUsesValidWords(t *testing.T) {
	repoPath := t.TempDir()

	adjSet := make(map[string]bool, len(adjectives))
	for _, a := range adjectives {
		adjSet[a] = true
	}
	nounSet := make(map[string]bool, len(nouns))
	for _, n := range nouns {
		nounSet[n] = true
	}

	for range 50 {
		name := RandomName(repoPath)
		parts := strings.SplitN(name, "-", 2)
		if !adjSet[parts[0]] {
			t.Errorf("adjective %q not in word list", parts[0])
		}
		if !nounSet[parts[1]] {
			t.Errorf("noun %q not in word list", parts[1])
		}
	}
}
```

**Step 2: Run tests to verify they fail**

Run from repo root:
```
nix develop . --command go test ./packages/spinclass/internal/worktree/ -run 'TestRandomName' -v
```
Expected: FAIL — `RandomName` undefined.

**Step 3: Write the implementation**

Create `packages/spinclass/internal/worktree/namegen.go`:

```go
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
```

**Step 4: Run tests to verify they pass**

Run:
```
nix develop . --command go test ./packages/spinclass/internal/worktree/ -run 'TestRandomName' -v
```
Expected: PASS — all three tests green.

**Step 5: Commit**

```
git add packages/spinclass/internal/worktree/namegen.go packages/spinclass/internal/worktree/namegen_test.go
git commit -m "feat(spinclass): add RandomName for auto-generated session names"
```

---

### Task 2: Wire RandomName into the `new` command

**Files:**
- Modify: `packages/spinclass/cmd/spinclass/main.go:39-88`

**Step 1: Modify `newCmd` to accept zero args and generate a name**

In `packages/spinclass/cmd/spinclass/main.go`, make these changes:

1. Change `Use` from `"new <target> [claude args...]"` to `"new [target] [claude args...]"`
2. Change `Args` from `cobra.MinimumNArgs(1)` to `cobra.MinimumNArgs(0)`
3. In `RunE`, when `len(args) == 0`, generate a name via `worktree.RandomName(repoPath)`:

```go
var newCmd = &cobra.Command{
	Use:   "new [target] [claude args...]",
	Short: "Create (if needed) and attach to a worktree session",
	Long:  `Create a worktree if it doesn't exist, then attach to a session. Target is a branch name or path, resolved relative to the current git repository. If target is omitted, a random name is generated. If additional arguments are provided, claude is launched with those arguments instead of a shell.`,
	Args:  cobra.MinimumNArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		format := outputFormat
		if format == "" {
			format = "tap"
		}

		exec := executor.ZmxExecutor{}

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		repoPath, err := worktree.DetectRepo(cwd)
		if err != nil {
			return err
		}

		var target string
		var claudeArgs []string

		if len(args) == 0 {
			target = worktree.RandomName(repoPath)
		} else {
			target = args[0]
			if len(args) >= 2 {
				claudeArgs = args[1:]
			}
		}

		hierarchy, err := sweatfile.LoadDefaultHierarchy()
		if err != nil {
			return err
		}

		resolvedPath, err := worktree.ResolvePath(hierarchy.Merged, repoPath, target)
		if err != nil {
			return err
		}

		return shop.New(
			os.Stdout,
			exec,
			resolvedPath,
			format,
			claudeArgs,
			newMergeOnClose,
			newNoAttach,
			verbose,
		)
	},
}
```

**Step 2: Run full test suite**

Run:
```
nix develop . --command go test ./packages/spinclass/... -v
```
Expected: All tests pass (including existing ones — no regressions).

**Step 3: Commit**

```
git add packages/spinclass/cmd/spinclass/main.go
git commit -m "feat(spinclass): generate random session name when target omitted"
```

---

### Task 3: Update completions for optional target

**Files:**
- Check: `packages/spinclass/completions/` — shell completion scripts may reference `<target>` as required. Update if needed.

**Step 1: Check completions**

Read `packages/spinclass/completions/` files and verify they don't break with the optional argument change. Cobra-generated completions should adapt automatically, but custom fish/bash completions may need updating.

**Step 2: Update if needed, commit**

If changes are needed:
```
git add packages/spinclass/completions/
git commit -m "fix(spinclass): update completions for optional target in new"
```

---

### Task 4: Run full build and verify

**Step 1: Run nix build**

```
nix build .#spinclass
```
Expected: Builds successfully.

**Step 2: Run full test suite with TAP output**

From `packages/spinclass/`:
```
just test
```
Expected: All tests pass.

**Step 3: Final commit if any formatting changes needed**
