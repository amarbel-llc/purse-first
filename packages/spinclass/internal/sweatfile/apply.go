package sweatfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/amarbel-llc/spinclass/internal/git"
)

// HardcodedDefaults returns a Sweatfile with baseline excludes and allow rules
// that are always applied regardless of user sweatfile config.
func HardcodedDefaults() Sweatfile {
	sf := Sweatfile{
		GitExcludes: []string{
			".claude/settings.local.json",
			".tmp",
		},
	}

	if home, err := os.UserHomeDir(); err == nil && home != "" {
		claudeDir := filepath.Join(home, ".claude")
		sf.ClaudeAllow = []string{"Read(" + claudeDir + "/*)"}
	}

	return sf
}

func Apply(worktreePath string, sf Sweatfile) error {
	defaults := HardcodedDefaults()
	merged := Merge(sf, defaults)

	if len(merged.GitExcludes) > 0 {
		excludePath, err := resolveExcludePath(worktreePath)
		if err != nil {
			return fmt.Errorf("resolving git exclude path: %w", err)
		}
		if err := applyGitExcludes(excludePath, merged.GitExcludes); err != nil {
			return fmt.Errorf("applying git excludes: %w", err)
		}
	}

	if err := ApplyClaudeSettings(worktreePath, merged.ClaudeAllow); err != nil {
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
		if err := json.Unmarshal(data, &doc); err != nil {
			return err
		}
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
		"Read("+worktreePath+"/*)",
		"Edit("+worktreePath+"/*)",
		"Write("+worktreePath+"/*)",
	)

	permsMap["defaultMode"] = "acceptEdits"
	permsMap["allow"] = allRules

	doc["permissions"] = permsMap

	if git.IsWorktree(worktreePath) {
		doc["hooks"] = map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Read|Write|Edit|Glob|Grep|Bash|Task",
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "spinclass hooks",
						},
					},
				},
			},
		}
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(doc, "", "  ")

	if err != nil {
		return err
	}

	return os.WriteFile(settingsPath, append(data, '\n'), 0o644)
}

