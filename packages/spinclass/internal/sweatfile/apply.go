package sweatfile

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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

func (sweatfile Sweatfile) Apply(worktreePath string) error {
	defaults := HardcodedDefaults()
	merged := Merge(sweatfile, defaults)

	if len(merged.GitExcludes) > 0 {
		excludePath, err := resolveExcludePath(worktreePath)
		if err != nil {
			return fmt.Errorf("resolving git exclude path: %w", err)
		}

		if err := applyGitExcludes(excludePath, merged.GitExcludes); err != nil {
			return fmt.Errorf("applying git excludes: %w", err)
		}
	}

	if err := ApplyClaudeSettings(worktreePath, merged); err != nil {
		return fmt.Errorf("applying claude settings: %w", err)
	}

	if err := prepareDirenvIfNecessary(worktreePath); err != nil {
		return err
	}

	return nil
}

func prepareDirenvIfNecessary(worktreePath string) error {
	if _, ok := fileExists(filepath.Join(worktreePath, "flake.nix")); !ok {
		return nil
	}

	direnvPath, err := exec.LookPath("direnv")
	if err != nil {
		// TODO output skip
		return nil
	}

	// write .envrc
	{
		file, err := os.OpenFile(
			filepath.Join(worktreePath, ".envrc"),
			os.O_TRUNC|os.O_CREATE|os.O_WRONLY,
			0o644,
		)
		if err != nil {
			return err
		}

		// TODO capture error
		defer file.Close()

		bufferedWriter := bufio.NewWriter(file)

		// TODO capture error
		defer bufferedWriter.Flush()

		if _, err := fmt.Fprintln(bufferedWriter, "source_up"); err != nil {
			return err
		}

		if _, err := fmt.Fprintln(bufferedWriter, "use flake"); err != nil {
			return err
		}
	}

	// direnv allow
	{
		cmd := exec.Command(direnvPath, "allow")

		cmd.Dir = worktreePath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		return cmd.Run()
	}
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

	// TODO use bufio
	file, err := os.OpenFile(excludePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	// TODO capture error
	defer file.Close()

	for _, p := range patterns {
		if _, err := fmt.Fprintln(file, p); err != nil {
			return err
		}
	}

	return nil
}

func ApplyClaudeSettings(worktreePath string, sf Sweatfile) error {
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

	allRules := append([]string{}, sf.ClaudeAllow...)
	allRules = append(allRules,
		"Read("+worktreePath+"/*)",
		"Edit("+worktreePath+"/*)",
		"Write("+worktreePath+"/*)",
	)

	permsMap["defaultMode"] = "acceptEdits"
	permsMap["allow"] = allRules

	doc["permissions"] = permsMap

	if git.IsWorktree(worktreePath) {
		hooksMap := map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Read|Write|Edit|Glob|Grep|Bash|Task",
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "spinclass hooks --worktree-boundary-violations-notification",
						},
					},
				},
			},
		}

		if sf.StopHook != nil && *sf.StopHook != "" {
			hooksMap["Stop"] = []any{
				map[string]any{
					"matcher": "*",
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "spinclass hooks",
						},
					},
				},
			}
		}

		doc["hooks"] = hooksMap
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
