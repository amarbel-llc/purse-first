package perms

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

type Tier struct {
	Allow []string `json:"allow"`
}

func LoadTierFile(path string) (Tier, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Tier{Allow: []string{}}, nil
		}
		return Tier{}, err
	}

	var tier Tier
	if err := json.Unmarshal(data, &tier); err != nil {
		return Tier{}, err
	}

	if tier.Allow == nil {
		tier.Allow = []string{}
	}

	return tier, nil
}

func SaveTierFile(path string, tier Tier) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(tier, "", "  ")
	if err != nil {
		return err
	}

	data = append(data, '\n')

	return os.WriteFile(path, data, 0o644)
}

func AppendToTierFile(path string, rule string) error {
	tier, err := LoadTierFile(path)
	if err != nil {
		return err
	}

	for _, existing := range tier.Allow {
		if existing == rule {
			return nil
		}
	}

	tier.Allow = append(tier.Allow, rule)

	return SaveTierFile(path, tier)
}

func TiersDir() string {
	if dir := os.Getenv("SWEATSHOP_PERMS_DIR"); dir != "" {
		return dir
	}

	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "spinclass", "permissions")
}

func LoadTiers(tiersDir string, repo string) []string {
	globalPath := filepath.Join(tiersDir, "global.json")
	repoPath := filepath.Join(tiersDir, "repos", repo+".json")

	global, _ := LoadTierFile(globalPath)
	repoTier, _ := LoadTierFile(repoPath)

	seen := map[string]bool{}
	var merged []string

	for _, r := range global.Allow {
		if !seen[r] {
			seen[r] = true
			merged = append(merged, r)
		}
	}

	for _, r := range repoTier.Allow {
		if !seen[r] {
			seen[r] = true
			merged = append(merged, r)
		}
	}

	if merged == nil {
		merged = []string{}
	}

	return merged
}
