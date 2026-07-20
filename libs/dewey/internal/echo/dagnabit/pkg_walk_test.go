package dagnabit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBranchLeafNode(t *testing.T) {
	const modulePath = "code.linenisgreat.com/purse-first/libs/dewey"

	cases := []struct {
		name     string
		pkgPath  string
		wantNode string
		wantOK   bool
	}{
		{
			name:     "two components after module",
			pkgPath:  modulePath + "/alfa/cmp",
			wantNode: "alfa/cmp",
			wantOK:   true,
		},
		{
			name:     "three components — branch+leaf only",
			pkgPath:  modulePath + "/echo/command/huh",
			wantNode: "echo/command",
			wantOK:   true,
		},
		{
			name:    "outside module",
			pkgPath: "github.com/other/repo/foo/bar",
			wantOK:  false,
		},
		{
			name:    "single component below module",
			pkgPath: modulePath + "/topgroup",
			wantOK:  false,
		},
		{
			name:    "exactly module path",
			pkgPath: modulePath,
			wantOK:  false,
		},
		{
			name:    "module prefix without slash",
			pkgPath: modulePath + "alfa/cmp",
			wantOK:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node, branch, leaf, ok := branchLeafNode(tc.pkgPath, modulePath)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}

			if !ok {
				return
			}

			if node != tc.wantNode {
				t.Errorf("node = %q, want %q", node, tc.wantNode)
			}

			if got := branch + "/" + leaf; got != node {
				t.Errorf("branch+leaf %q does not match node %q", got, node)
			}
		})
	}
}

func TestShouldSkipWalkDir(t *testing.T) {
	skip := []string{
		".git", ".direnv", ".tmp", "build", "node_modules",
		"result", "testdata", "vendor",
	}

	keep := []string{
		"libs", "internal", "pkgs", "cmd", "alfa", "bravo",
		"", "gitignored",
	}

	for _, name := range skip {
		t.Run("skip/"+name, func(t *testing.T) {
			if !shouldSkipWalkDir(name) {
				t.Errorf("shouldSkipWalkDir(%q) = false, want true", name)
			}
		})
	}

	for _, name := range keep {
		t.Run("keep/"+name, func(t *testing.T) {
			if shouldSkipWalkDir(name) {
				t.Errorf("shouldSkipWalkDir(%q) = true, want false", name)
			}
		})
	}
}

func TestFindWorkspaceRoot(t *testing.T) {
	root := t.TempDir()

	deep := filepath.Join(root, "a", "b", "c", "d")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := findWorkspaceRoot(deep); !samePath(got, root) {
		t.Errorf("findWorkspaceRoot(deep) = %q, want %q", got, root)
	}

	if got := findWorkspaceRoot(root); !samePath(got, root) {
		t.Errorf("findWorkspaceRoot(root) = %q, want %q", got, root)
	}
}

// samePath compares two paths after Abs-resolving both, since t.TempDir()
// may return a symlinked path on macOS (/tmp -> /private/tmp).
func samePath(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return a == b
	}
	return aa == bb
}
