package perms

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRouteDecisions(t *testing.T) {
	tmpDir := t.TempDir()
	tiersDir := filepath.Join(tmpDir, "tiers")
	os.MkdirAll(filepath.Join(tiersDir, "repos"), 0o755)

	// Seed global tier with ["Read"]
	globalPath := filepath.Join(tiersDir, "global.json")
	globalTier := Tier{Allow: []string{"Read"}}
	globalData, _ := json.MarshalIndent(globalTier, "", "  ")
	os.WriteFile(globalPath, globalData, 0o644)

	// Seed repo tier with []
	repoPath := filepath.Join(tiersDir, "repos", "myrepo.json")
	repoTier := Tier{Allow: []string{}}
	repoData, _ := json.MarshalIndent(repoTier, "", "  ")
	os.WriteFile(repoPath, repoData, 0o644)

	// Seed settings.local.json with ["Read", "Edit", "Bash(go test:*)"]
	settingsPath := filepath.Join(tmpDir, ".claude", "settings.local.json")
	err := SaveClaudeSettings(settingsPath, []string{"Read", "Edit", "Bash(go test:*)"})
	if err != nil {
		t.Fatalf("failed to seed settings: %v", err)
	}

	decisions := []ReviewDecision{
		{Rule: "Edit", Action: ReviewPromoteGlobal},
		{Rule: "Bash(go test:*)", Action: ReviewPromoteRepo},
	}

	err = RouteDecisions(tiersDir, "myrepo", settingsPath, decisions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify global tier now contains "Edit"
	global, err := LoadTierFile(globalPath)
	if err != nil {
		t.Fatalf("failed to load global tier: %v", err)
	}
	globalFound := map[string]bool{}
	for _, r := range global.Allow {
		globalFound[r] = true
	}
	if !globalFound["Read"] {
		t.Error("expected Read to remain in global tier")
	}
	if !globalFound["Edit"] {
		t.Error("expected Edit to be promoted to global tier")
	}

	// Verify repo tier now contains "Bash(go test:*)"
	repo, err := LoadTierFile(repoPath)
	if err != nil {
		t.Fatalf("failed to load repo tier: %v", err)
	}
	repoFound := map[string]bool{}
	for _, r := range repo.Allow {
		repoFound[r] = true
	}
	if !repoFound["Bash(go test:*)"] {
		t.Error("expected Bash(go test:*) to be promoted to repo tier")
	}

	// Verify settings.local.json no longer contains promoted rules but still has "Read"
	remaining, err := LoadClaudeSettings(settingsPath)
	if err != nil {
		t.Fatalf("failed to load settings: %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining rule in settings, got %d: %v", len(remaining), remaining)
	}
	if remaining[0] != "Read" {
		t.Errorf("expected Read to remain in settings, got %q", remaining[0])
	}
}

func TestRouteDecisionsDiscard(t *testing.T) {
	tmpDir := t.TempDir()
	tiersDir := filepath.Join(tmpDir, "tiers")
	os.MkdirAll(filepath.Join(tiersDir, "repos"), 0o755)

	settingsPath := filepath.Join(tmpDir, ".claude", "settings.local.json")
	err := SaveClaudeSettings(settingsPath, []string{"Read", "Bash(rm -rf:*)"})
	if err != nil {
		t.Fatalf("failed to seed settings: %v", err)
	}

	decisions := []ReviewDecision{
		{Rule: "Bash(rm -rf:*)", Action: ReviewDiscard},
	}

	err = RouteDecisions(tiersDir, "myrepo", settingsPath, decisions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	remaining, err := LoadClaudeSettings(settingsPath)
	if err != nil {
		t.Fatalf("failed to load settings: %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining rule, got %d: %v", len(remaining), remaining)
	}
	if remaining[0] != "Read" {
		t.Errorf("expected Read, got %q", remaining[0])
	}
}

func TestRouteDecisionsKeep(t *testing.T) {
	tmpDir := t.TempDir()
	tiersDir := filepath.Join(tmpDir, "tiers")
	os.MkdirAll(filepath.Join(tiersDir, "repos"), 0o755)

	settingsPath := filepath.Join(tmpDir, ".claude", "settings.local.json")
	err := SaveClaudeSettings(settingsPath, []string{"Read", "Edit"})
	if err != nil {
		t.Fatalf("failed to seed settings: %v", err)
	}

	decisions := []ReviewDecision{
		{Rule: "Edit", Action: ReviewKeep},
	}

	err = RouteDecisions(tiersDir, "myrepo", settingsPath, decisions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	remaining, err := LoadClaudeSettings(settingsPath)
	if err != nil {
		t.Fatalf("failed to load settings: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("expected 2 remaining rules, got %d: %v", len(remaining), remaining)
	}
	if remaining[0] != "Read" {
		t.Errorf("expected Read, got %q", remaining[0])
	}
	if remaining[1] != "Edit" {
		t.Errorf("expected Edit, got %q", remaining[1])
	}
}
