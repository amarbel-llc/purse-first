package sweatfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMinimal(t *testing.T) {
	input := `
git_excludes = [".claude/"]
`
	sf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sf.GitExcludes) != 1 || sf.GitExcludes[0] != ".claude/" {
		t.Errorf("git_excludes: got %v", sf.GitExcludes)
	}
}

func TestParseEmpty(t *testing.T) {
	sf, err := Parse([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sf.GitExcludes != nil {
		t.Errorf("expected nil git_excludes, got %v", sf.GitExcludes)
	}
}

func TestLoadFromPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sweatfile")
	os.WriteFile(path, []byte(`git_excludes = [".direnv/"]`), 0o644)

	sf, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sf.GitExcludes) != 1 || sf.GitExcludes[0] != ".direnv/" {
		t.Errorf("git_excludes: got %v", sf.GitExcludes)
	}
}

func TestLoadMissing(t *testing.T) {
	sf, err := Load("/nonexistent/sweatfile")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sf.GitExcludes != nil {
		t.Errorf("expected nil git_excludes, got %v", sf.GitExcludes)
	}
}

func TestMergeConcatenatesArrays(t *testing.T) {
	base := Sweatfile{
		GitExcludes: []string{".claude/"},
	}
	repo := Sweatfile{
		GitExcludes: []string{".direnv/"},
	}
	merged := Merge(base, repo)
	if len(merged.GitExcludes) != 2 {
		t.Fatalf("expected 2 git_excludes, got %v", merged.GitExcludes)
	}
	if merged.GitExcludes[0] != ".claude/" || merged.GitExcludes[1] != ".direnv/" {
		t.Errorf("git_excludes: got %v", merged.GitExcludes)
	}
}

func TestMergeClearSentinel(t *testing.T) {
	base := Sweatfile{
		GitExcludes: []string{".claude/"},
	}
	repo := Sweatfile{
		GitExcludes: []string{},
	}
	merged := Merge(base, repo)
	if len(merged.GitExcludes) != 0 {
		t.Errorf("expected cleared git_excludes, got %v", merged.GitExcludes)
	}
}

func TestMergeBaseOnly(t *testing.T) {
	base := Sweatfile{GitExcludes: []string{".claude/"}}
	merged := Merge(base, Sweatfile{})
	if len(merged.GitExcludes) != 1 || merged.GitExcludes[0] != ".claude/" {
		t.Errorf("expected inherited git_excludes, got %v", merged.GitExcludes)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sweatfile")

	sf := Sweatfile{
		GitExcludes: []string{".claude/"},
	}

	err := Save(path, sf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error loading: %v", err)
	}
	if len(loaded.GitExcludes) != 1 || loaded.GitExcludes[0] != ".claude/" {
		t.Errorf("git_excludes roundtrip: got %v", loaded.GitExcludes)
	}
}

func TestParseClaudeAllow(t *testing.T) {
	input := `
claude_allow = ["Read", "Bash(git *)"]
`
	sf, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sf.ClaudeAllow) != 2 {
		t.Fatalf("expected 2 claude_allow rules, got %v", sf.ClaudeAllow)
	}
	if sf.ClaudeAllow[0] != "Read" || sf.ClaudeAllow[1] != "Bash(git *)" {
		t.Errorf("claude_allow: got %v", sf.ClaudeAllow)
	}
}

func TestMergeClaudeAllowAppends(t *testing.T) {
	base := Sweatfile{ClaudeAllow: []string{"Read", "Glob"}}
	repo := Sweatfile{ClaudeAllow: []string{"Bash(go test:*)"}}
	merged := Merge(base, repo)
	if len(merged.ClaudeAllow) != 3 {
		t.Fatalf("expected 3 claude_allow rules, got %v", merged.ClaudeAllow)
	}
	if merged.ClaudeAllow[2] != "Bash(go test:*)" {
		t.Errorf("expected appended rule, got %v", merged.ClaudeAllow)
	}
}

func TestMergeClaudeAllowClear(t *testing.T) {
	base := Sweatfile{ClaudeAllow: []string{"Read", "Glob"}}
	repo := Sweatfile{ClaudeAllow: []string{}}
	merged := Merge(base, repo)
	if len(merged.ClaudeAllow) != 0 {
		t.Errorf("expected cleared claude_allow, got %v", merged.ClaudeAllow)
	}
}

func writeSweatfile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creating directory for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func TestLoadHierarchyGlobalOnly(t *testing.T) {
	home := t.TempDir()
	repoDir := filepath.Join(home, "eng", "repos", "myrepo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	globalPath := filepath.Join(home, ".config", "spinclass", "sweatfile")
	writeSweatfile(t, globalPath, `
git_excludes = [".DS_Store"]
claude_allow = ["/docs"]
`)

	result, err := LoadHierarchy(home, repoDir)
	if err != nil {
		t.Fatalf("LoadHierarchy returned error: %v", err)
	}

	// Should have checked: global, eng/sweatfile, eng/repos/sweatfile, myrepo/sweatfile
	if len(result.Sources) != 4 {
		t.Fatalf("expected 4 sources, got %d", len(result.Sources))
	}

	// Only global should be found
	if !result.Sources[0].Found {
		t.Error("expected global source to be found")
	}
	for i := 1; i < len(result.Sources); i++ {
		if result.Sources[i].Found {
			t.Errorf("expected source %d (%s) to not be found", i, result.Sources[i].Path)
		}
	}

	if len(result.Merged.GitExcludes) != 1 || result.Merged.GitExcludes[0] != ".DS_Store" {
		t.Errorf("expected GitExcludes=[.DS_Store], got %v", result.Merged.GitExcludes)
	}
	if len(result.Merged.ClaudeAllow) != 1 || result.Merged.ClaudeAllow[0] != "/docs" {
		t.Errorf("expected ClaudeAllow=[/docs], got %v", result.Merged.ClaudeAllow)
	}
}

func TestLoadHierarchyGlobalAndRepo(t *testing.T) {
	home := t.TempDir()
	repoDir := filepath.Join(home, "eng", "repos", "myrepo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	globalPath := filepath.Join(home, ".config", "spinclass", "sweatfile")
	writeSweatfile(t, globalPath, `
git_excludes = [".DS_Store"]
`)

	repoSweatfile := filepath.Join(repoDir, "sweatfile")
	writeSweatfile(t, repoSweatfile, `
git_excludes = [".idea"]
claude_allow = ["/src"]
`)

	result, err := LoadHierarchy(home, repoDir)
	if err != nil {
		t.Fatalf("LoadHierarchy returned error: %v", err)
	}

	// Merged should have both git_excludes appended
	if len(result.Merged.GitExcludes) != 2 {
		t.Fatalf("expected 2 GitExcludes, got %v", result.Merged.GitExcludes)
	}
	if result.Merged.GitExcludes[0] != ".DS_Store" || result.Merged.GitExcludes[1] != ".idea" {
		t.Errorf("expected GitExcludes=[.DS_Store, .idea], got %v", result.Merged.GitExcludes)
	}

	// ClaudeAllow from repo only
	if len(result.Merged.ClaudeAllow) != 1 || result.Merged.ClaudeAllow[0] != "/src" {
		t.Errorf("expected ClaudeAllow=[/src], got %v", result.Merged.ClaudeAllow)
	}
}

func TestLoadHierarchyParentDir(t *testing.T) {
	home := t.TempDir()
	repoDir := filepath.Join(home, "eng", "repos", "myrepo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	globalPath := filepath.Join(home, ".config", "spinclass", "sweatfile")
	writeSweatfile(t, globalPath, `
git_excludes = [".DS_Store"]
`)

	parentPath := filepath.Join(home, "eng", "sweatfile")
	writeSweatfile(t, parentPath, `
git_excludes = [".envrc"]
claude_allow = ["/eng-docs"]
`)

	repoSweatfile := filepath.Join(repoDir, "sweatfile")
	writeSweatfile(t, repoSweatfile, `
claude_allow = ["/src"]
`)

	result, err := LoadHierarchy(home, repoDir)
	if err != nil {
		t.Fatalf("LoadHierarchy returned error: %v", err)
	}

	// git_excludes: global .DS_Store + parent .envrc = [.DS_Store, .envrc]
	// repo has nil git_excludes so inherits
	if len(result.Merged.GitExcludes) != 2 {
		t.Fatalf("expected 2 GitExcludes, got %v", result.Merged.GitExcludes)
	}
	if result.Merged.GitExcludes[0] != ".DS_Store" || result.Merged.GitExcludes[1] != ".envrc" {
		t.Errorf("expected GitExcludes=[.DS_Store, .envrc], got %v", result.Merged.GitExcludes)
	}

	// claude_allow: parent /eng-docs + repo /src = [/eng-docs, /src]
	if len(result.Merged.ClaudeAllow) != 2 {
		t.Fatalf("expected 2 ClaudeAllow, got %v", result.Merged.ClaudeAllow)
	}
	if result.Merged.ClaudeAllow[0] != "/eng-docs" || result.Merged.ClaudeAllow[1] != "/src" {
		t.Errorf("expected ClaudeAllow=[/eng-docs, /src], got %v", result.Merged.ClaudeAllow)
	}

	// Verify sources: global found, eng/sweatfile found, eng/repos/sweatfile not found, myrepo/sweatfile found
	if !result.Sources[0].Found {
		t.Error("expected global source to be found")
	}
	if !result.Sources[1].Found {
		t.Error("expected eng/sweatfile source to be found")
	}
	if result.Sources[2].Found {
		t.Error("expected eng/repos/sweatfile source to not be found")
	}
	if !result.Sources[3].Found {
		t.Error("expected repo sweatfile source to be found")
	}
}

func TestLoadHierarchyNoSweatfiles(t *testing.T) {
	home := t.TempDir()
	repoDir := filepath.Join(home, "eng", "repos", "myrepo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := LoadHierarchy(home, repoDir)
	if err != nil {
		t.Fatalf("LoadHierarchy returned error: %v", err)
	}

	// All sources should be not found
	for i, src := range result.Sources {
		if src.Found {
			t.Errorf("expected source %d (%s) to not be found", i, src.Path)
		}
	}

	// Merged should be empty
	if result.Merged.GitExcludes != nil {
		t.Errorf("expected nil GitExcludes, got %v", result.Merged.GitExcludes)
	}
	if result.Merged.ClaudeAllow != nil {
		t.Errorf("expected nil ClaudeAllow, got %v", result.Merged.ClaudeAllow)
	}
}

func TestLoadHierarchyRepoOverridesParent(t *testing.T) {
	home := t.TempDir()
	repoDir := filepath.Join(home, "eng", "repos", "myrepo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	parentPath := filepath.Join(home, "eng", "sweatfile")
	writeSweatfile(t, parentPath, `
git_excludes = [".DS_Store", ".envrc"]
claude_allow = ["/docs"]
`)

	// Repo sweatfile with empty arrays clears parent values
	repoSweatfile := filepath.Join(repoDir, "sweatfile")
	writeSweatfile(t, repoSweatfile, `
git_excludes = []
claude_allow = []
`)

	result, err := LoadHierarchy(home, repoDir)
	if err != nil {
		t.Fatalf("LoadHierarchy returned error: %v", err)
	}

	// Empty arrays should clear parent values
	if result.Merged.GitExcludes == nil || len(result.Merged.GitExcludes) != 0 {
		t.Errorf("expected empty GitExcludes (cleared by repo), got %v", result.Merged.GitExcludes)
	}
	if result.Merged.ClaudeAllow == nil || len(result.Merged.ClaudeAllow) != 0 {
		t.Errorf("expected empty ClaudeAllow (cleared by repo), got %v", result.Merged.ClaudeAllow)
	}
}
