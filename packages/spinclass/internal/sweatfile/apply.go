package sweatfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/amarbel-llc/spinclass/internal/git"
)

// HardcodedExcludes are always written to .git/info/exclude regardless of sweatfile config.
var HardcodedExcludes = []string{
	".claude",
}

func Apply(worktreePath string, sf Sweatfile) error {
	allExcludes := append(sf.GitExcludes, HardcodedExcludes...)
	if len(allExcludes) > 0 {
		excludePath, err := resolveExcludePath(worktreePath)
		if err != nil {
			return fmt.Errorf("resolving git exclude path: %w", err)
		}
		if err := applyGitExcludes(excludePath, allExcludes); err != nil {
			return fmt.Errorf("applying git excludes: %w", err)
		}
	}

	if err := ApplyClaudeSettings(worktreePath, sf.ClaudeAllow); err != nil {
		return fmt.Errorf("applying claude settings: %w", err)
	}

	return nil
}

func resolveExcludePath(worktreePath string) (string, error) {
	rel, err := git.Run(worktreePath, "rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(rel) {
		rel = filepath.Join(worktreePath, rel)
	}
	return rel, nil
}

func applyGitExcludes(excludePath string, patterns []string) error {
	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(excludePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, p := range patterns {
		if _, err := fmt.Fprintln(f, p); err != nil {
			return err
		}
	}
	return nil
}

func ApplyClaudeSettings(worktreePath string, rules []string) error {
	settingsPath := filepath.Join(worktreePath, ".claude", "settings.local.json")

	var doc map[string]any
	if data, err := os.ReadFile(settingsPath); err == nil {
		json.Unmarshal(data, &doc)
	}
	if doc == nil {
		doc = make(map[string]any)
	}

	permsMap, _ := doc["permissions"].(map[string]any)
	if permsMap == nil {
		permsMap = make(map[string]any)
	}

	allRules := append([]string{}, rules...)
	allRules = append(allRules,
		"Edit(//"+worktreePath+"/**)",
		"Write(//"+worktreePath+"/**)",
	)

	permsMap["defaultMode"] = "acceptEdits"
	permsMap["allow"] = allRules
	doc["permissions"] = permsMap

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(settingsPath, append(data, '\n'), 0o644)
}
