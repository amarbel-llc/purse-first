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

type Experimental struct {
	BoundaryNotify *bool `toml:"boundary-notify"`
}

type Sweatfile struct {
	SystemPrompt       *string  `toml:"system-prompt"`        // TODO replace with PathOrString struct
	SystemPromptAppend *string  `toml:"system-prompt-append"` // TODO replace with PathOrString struct
	BranchNameCommand  string   `toml:"branch-name-command"`  // TODO add tests
	GitSkipIndex       []string `toml:"git_excludes"`         // TODO rename toml to git-skip-index

	// TODO turn ClaudeAllows into struct
	ClaudeAllow  []string      `toml:"claude_allow"` // TODO rename toml to claude-allow
	StopHook     *string       `toml:"stop_hook"`    // TODO rename toml to stop-hook
	Experimental *Experimental `toml:"experimental"`
}

func (sf Sweatfile) BoundaryNotifyEnabled() bool {
	return sf.Experimental != nil &&
		sf.Experimental.BoundaryNotify != nil &&
		*sf.Experimental.BoundaryNotify
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

	cmd := exec.Command(cmdComponents[0], cmdComponents[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if replacementBytes, err := cmd.Output(); err != nil {
		return "", err
	} else {
		return string(bytes.TrimSpace(replacementBytes)), nil
	}
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
