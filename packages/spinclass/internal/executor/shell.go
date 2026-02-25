package executor

import (
	"os"
	"os/exec"
	"strings"

	tap "github.com/amarbel-llc/tap-dancer/go"
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

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "SPINCLASS_SESSION="+key)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (s ShellExecutor) Detach() error {
	return nil
}
