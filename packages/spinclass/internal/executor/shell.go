package executor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tap "github.com/amarbel-llc/purse-first/packages/tap-dancer/go"
)

type ShellExecutor struct{}

func (s ShellExecutor) Attach(dir string, key string, command []string, dryRun bool, tp *tap.TestPoint) error {
	if len(command) == 0 {
		command = []string{os.Getenv("SHELL")}
	}

	if dryRun {
		tp.Skip = "dry run"
		tp.Diagnostics = &tap.Diagnostics{
			Extras: map[string]any{
				"command": strings.Join(command, " "),
			},
		}
		return nil
	}

	tmpDir := filepath.Join(dir, ".tmp")

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"SPINCLASS_SESSION="+key,
		"TMPDIR="+tmpDir,
		"CLAUDE_CODE_TMPDIR="+tmpDir,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (s ShellExecutor) Detach() error {
	return nil
}
