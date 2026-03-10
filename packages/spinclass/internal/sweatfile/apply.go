package sweatfile

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/amarbel-llc/spinclass/internal/git"
)

func (sweatfile Sweatfile) Apply(worktreePath string) error {
	defaults := GetDefault()
	merged := Merge(sweatfile, defaults)

	if err := ApplyClaudeSettings(worktreePath, merged); err != nil {
		return fmt.Errorf("applying claude settings: %w", err)
	}

	if err := sweatfile.prepareLocalBin(); err != nil {
		return err
	}

	if err := sweatfile.writeSpinclassEnv(worktreePath); err != nil {
		return fmt.Errorf("writing .spinclass.env: %w", err)
	}

	if err := sweatfile.prepareDirenv(worktreePath); err != nil {
		return err
	}

	return nil
}

func (sweatfile Sweatfile) GetDirSpinclassBin() string {
	return filepath.Join(".git/spinclass/bin/")
}

func (sweatfile Sweatfile) prepareLocalBin() error {
	dirSpinclassBin := sweatfile.GetDirSpinclassBin()

	if err := os.MkdirAll(dirSpinclassBin, 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(
		filepath.Join(dirSpinclassBin, "claude"),
		[]byte(`#! /usr/bin/env -S bash -e
exec spinclass exec-claude "$@"`,
		),
		0o644,
	); err != nil {
		return err
	}

	// TODO write claude bin

	return nil
}

func (sf Sweatfile) writeEnvrc(worktreePath string) error {
	file, err := os.OpenFile(
		filepath.Join(worktreePath, ".envrc"),
		os.O_TRUNC|os.O_CREATE|os.O_WRONLY,
		0o644,
	)
	if err != nil {
		return err
	}
	defer file.Close()

	bufferedWriter := bufio.NewWriter(file)

	directives := sf.EnvrcDirectives
	if directives == nil {
		directives = []string{"source_up"}
		if _, ok := fileExists(filepath.Join(worktreePath, "flake.nix")); ok {
			directives = append(directives, "use flake")
		}
	}

	for _, directive := range directives {
		if _, err := fmt.Fprintln(bufferedWriter, directive); err != nil {
			return err
		}
	}

	if len(sf.Env) > 0 {
		if _, err := fmt.Fprintln(bufferedWriter, "dotenv .spinclass.env"); err != nil {
			return err
		}
	}

	dirSpinclassBinAbs, err := filepath.Abs(".git/spinclass/bin")
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(
		bufferedWriter,
		"PATH_add \"%s\"\n",
		dirSpinclassBinAbs,
	); err != nil {
		return err
	}

	return bufferedWriter.Flush()
}

func (sf Sweatfile) writeSpinclassEnv(worktreePath string) error {
	if len(sf.Env) == 0 {
		return nil
	}

	keys := make([]string, 0, len(sf.Env))
	for k := range sf.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	file, err := os.OpenFile(
		filepath.Join(worktreePath, ".spinclass.env"),
		os.O_TRUNC|os.O_CREATE|os.O_WRONLY,
		0o644,
	)
	if err != nil {
		return err
	}
	defer file.Close()

	expand := func(key string) string {
		if key == "WORKTREE" {
			return worktreePath
		}
		return os.Getenv(key)
	}

	for _, k := range keys {
		expanded := os.Expand(sf.Env[k], expand)
		if _, err := fmt.Fprintf(file, "%s=%s\n", k, expanded); err != nil {
			return err
		}
	}

	return nil
}

func (sf Sweatfile) prepareDirenv(worktreePath string) error {
	direnvPath, err := exec.LookPath("direnv")
	if err != nil {
		return nil
	}

	if err := sf.writeEnvrc(worktreePath); err != nil {
		return err
	}

	cmd := exec.Command(direnvPath, "allow")
	cmd.Dir = worktreePath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

func (sf Sweatfile) RunCreateHook(worktreePath string) error {
	cmd := sf.CreateHookCommand()
	if cmd == nil || *cmd == "" {
		return nil
	}

	c := exec.Command("sh", "-c", *cmd)
	c.Dir = worktreePath
	c.Env = append(os.Environ(), "WORKTREE="+worktreePath)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	return c.Run()
}

func ApplyClaudeSettings(worktreePath string, sweatfile Sweatfile) error {
	settingsPath := filepath.Join(worktreePath, ".claude", "settings.local.json")

	doc := make(map[string]any)

	permsMap, _ := doc["permissions"].(map[string]any)

	if permsMap == nil {
		permsMap = make(map[string]any)
	}

	allRules := append([]string{}, sweatfile.ClaudeAllow...)

	allRules = append(allRules,
		fmt.Sprintf("Read(%s/*)", worktreePath),
		fmt.Sprintf("Edit(%s/*)", worktreePath),
		fmt.Sprintf("Write(%s/*)", worktreePath),
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
							"command": "spinclass hooks",
						},
					},
				},
			},
		}

		if cmd := sweatfile.StopHookCommand(); cmd != nil && *cmd != "" {
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
