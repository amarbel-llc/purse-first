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

func (sweatfile Sweatfile) Apply(worktreePath string) error {
	defaults := GetDefault()
	merged := Merge(sweatfile, defaults)

	if err := ApplyClaudeSettings(worktreePath, merged); err != nil {
		return fmt.Errorf("applying claude settings: %w", err)
	}

	if err := sweatfile.prepareLocalBin(); err != nil {
		return err
	}

	if err := prepareDirenv(worktreePath); err != nil {
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

func writeEnvrc(worktreePath string) error {
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

	if _, err := fmt.Fprintln(bufferedWriter, "source_up"); err != nil {
		return err
	}

	if _, ok := fileExists(filepath.Join(worktreePath, "flake.nix")); ok {
		if _, err := fmt.Fprintln(bufferedWriter, "use flake"); err != nil {
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

func prepareDirenv(worktreePath string) error {
	direnvPath, err := exec.LookPath("direnv")
	if err != nil {
		// TODO output skip
		return nil
	}

	if err := writeEnvrc(worktreePath); err != nil {
		return err
	}

	cmd := exec.Command(direnvPath, "allow")
	cmd.Dir = worktreePath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

func ApplyClaudeSettings(worktreePath string, sweatfile Sweatfile) error {
	settingsPath := filepath.Join(worktreePath, ".claude", "settings.local.json")

	doc := make(map[string]any)

	permsMap, _ := doc["permissions"].(map[string]any)

	if permsMap == nil {
		permsMap = make(map[string]any)
	}

	allRules := append([]string{}, sweatfile.ClaudeAllow...)

	// TODO rewrite as sprintf
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

		if sweatfile.StopHook != nil && *sweatfile.StopHook != "" {
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
