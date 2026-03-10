package sweatfile

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/shlex"
)

type Hooks struct {
	Create               *string `toml:"create"`
	Stop                 *string `toml:"stop"`
	DisallowMainWorktree *bool   `toml:"disallow-main-worktree"`
}

type Sweatfile struct {
	SystemPrompt       *string           `toml:"system-prompt"`        // TODO replace with PathOrString struct
	SystemPromptAppend *string           `toml:"system-prompt-append"` // TODO replace with PathOrString struct
	BranchNameCommand  string            `toml:"branch-name-command"`  // TODO add tests
	GitSkipIndex       []string          `toml:"git-excludes"`
	ClaudeAllow        []string          `toml:"claude-allow"`
	EnvrcDirectives    []string          `toml:"envrc-directives"`
	Env                map[string]string `toml:"env"`
	Hooks              *Hooks            `toml:"hooks"`
}

func (sf Sweatfile) StopHookCommand() *string {
	if sf.Hooks == nil {
		return nil
	}
	return sf.Hooks.Stop
}

func (sf Sweatfile) CreateHookCommand() *string {
	if sf.Hooks == nil {
		return nil
	}
	return sf.Hooks.Create
}

func (sf Sweatfile) DisallowMainWorktreeEnabled() bool {
	return sf.Hooks != nil &&
		sf.Hooks.DisallowMainWorktree != nil &&
		*sf.Hooks.DisallowMainWorktree
}

// baseline excludes and allow rules that are always applied regardless of user
// sweatfile config.
func GetDefault() Sweatfile {
	sweatfile := Sweatfile{
		GitSkipIndex: []string{},
	}

	if home, err := os.UserHomeDir(); err == nil && home != "" {
		claudeDir := filepath.Join(home, ".claude")
		sweatfile.ClaudeAllow = []string{fmt.Sprintf("Read(%s/*)", claudeDir)}
	}

	return sweatfile
}

func (sweatfile Sweatfile) CreateBranchName(
	base string,
) (string, error) {
	if sweatfile.BranchNameCommand == "" {
		return base, nil
	}

	cmdComponents, err := shlex.Split(sweatfile.BranchNameCommand)
	if err != nil {
		return "", err
	}

	cmdComponents = append(cmdComponents, base)
	cmd := exec.Command(cmdComponents[0], cmdComponents[1:]...)
	cmd.Stderr = os.Stderr

	replacementBytes, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(bytes.TrimSpace(replacementBytes)), nil
}

func (sweatfile Sweatfile) ExecClaude(
	args ...string,
) error {
	if sweatfile.SystemPromptAppend != nil {
		args = append(
			[]string{
				"--append-system-prompt",
				*sweatfile.SystemPromptAppend,
			},
			args...,
		)
	}

	if sweatfile.SystemPrompt != nil {
		args = append(
			[]string{
				"--system-prompt",
				*sweatfile.SystemPrompt,
			},
			args...,
		)
	}

	pathGitDirCommon, err := getGitDirCommon()
	if err != nil {
		return err
	}

	pathSweatfileBin := filepath.Join(pathGitDirCommon, "spinclass/bin/")

	envVarPath := filepath.SplitList(os.Getenv("PATH"))
	envVarPath = slices.DeleteFunc(envVarPath, func(value string) bool {
		return filepath.Clean(value) == pathSweatfileBin
	})
	os.Setenv("PATH", strings.Join(envVarPath, string(filepath.ListSeparator)))

	cmdClaude := exec.Command("claude", args...)
	cmdClaude.Stdout = os.Stdout
	cmdClaude.Stderr = os.Stderr
	cmdClaude.Stdin = os.Stdin

	if err := cmdClaude.Run(); err != nil {
		return err
	}

	return nil
}
