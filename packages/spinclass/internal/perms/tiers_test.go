package perms

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTierFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.json")

	tier := Tier{Allow: []string{"Bash(git *)", "Read(~/eng/**)"}}
	data, err := json.MarshalIndent(tier, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	os.WriteFile(path, data, 0o644)

	loaded, err := LoadTierFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(loaded.Allow) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(loaded.Allow))
	}
	if loaded.Allow[0] != "Bash(git *)" {
		t.Errorf("expected Bash(git *), got %q", loaded.Allow[0])
	}
	if loaded.Allow[1] != "Read(~/eng/**)" {
		t.Errorf("expected Read(~/eng/**), got %q", loaded.Allow[1])
	}
}

func TestLoadTierFileMissing(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nonexistent.json")

	loaded, err := LoadTierFile(path)
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if len(loaded.Allow) != 0 {
		t.Errorf("expected empty allow list, got %d rules", len(loaded.Allow))
	}
}

func TestLoadTiers(t *testing.T) {
	tmpDir := t.TempDir()

	globalTier := Tier{Allow: []string{"Bash(git *)"}}
	globalData, _ := json.MarshalIndent(globalTier, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, "global.json"), globalData, 0o644)

	repoDir := filepath.Join(tmpDir, "repos")
	os.MkdirAll(repoDir, 0o755)
	repoTier := Tier{Allow: []string{"Read(~/eng/**)"}}
	repoData, _ := json.MarshalIndent(repoTier, "", "  ")
	os.WriteFile(filepath.Join(repoDir, "myrepo.json"), repoData, 0o644)

	merged := LoadTiers(tmpDir, "myrepo")
	if len(merged) != 2 {
		t.Fatalf("expected 2 merged rules, got %d", len(merged))
	}

	found := map[string]bool{}
	for _, r := range merged {
		found[r] = true
	}
	if !found["Bash(git *)"] {
		t.Error("expected Bash(git *) in merged rules")
	}
	if !found["Read(~/eng/**)"] {
		t.Error("expected Read(~/eng/**) in merged rules")
	}
}

func TestSaveTierFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sub", "tier.json")

	tier := Tier{Allow: []string{"Bash(nix *)"}}
	err := SaveTierFile(path, tier)
	if err != nil {
		t.Fatalf("unexpected error saving: %v", err)
	}

	loaded, err := LoadTierFile(path)
	if err != nil {
		t.Fatalf("unexpected error loading: %v", err)
	}
	if len(loaded.Allow) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(loaded.Allow))
	}
	if loaded.Allow[0] != "Bash(nix *)" {
		t.Errorf("expected Bash(nix *), got %q", loaded.Allow[0])
	}
}

func TestAppendToTierFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "tier.json")

	tier := Tier{Allow: []string{"Bash(git *)"}}
	data, _ := json.MarshalIndent(tier, "", "  ")
	os.WriteFile(path, data, 0o644)

	err := AppendToTierFile(path, "Read(~/eng/**)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, err := LoadTierFile(path)
	if err != nil {
		t.Fatalf("unexpected error loading: %v", err)
	}
	if len(loaded.Allow) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(loaded.Allow))
	}
	if loaded.Allow[1] != "Read(~/eng/**)" {
		t.Errorf("expected Read(~/eng/**), got %q", loaded.Allow[1])
	}
}

func TestAppendToTierFileNoDuplicates(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "tier.json")

	tier := Tier{Allow: []string{"Bash(git *)"}}
	data, _ := json.MarshalIndent(tier, "", "  ")
	os.WriteFile(path, data, 0o644)

	err := AppendToTierFile(path, "Bash(git *)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, err := LoadTierFile(path)
	if err != nil {
		t.Fatalf("unexpected error loading: %v", err)
	}
	if len(loaded.Allow) != 1 {
		t.Fatalf("expected 1 rule (no duplicate), got %d", len(loaded.Allow))
	}
}
