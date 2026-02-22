package executor

import (
	"os"
	"os/exec"
)

type ShellExecutor struct{}

func (s ShellExecutor) Attach(dir string, key string, command []string) error {
	if len(command) == 0 {
		command = []string{os.Getenv("SHELL")}
	}

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (s ShellExecutor) Detach() error {
	return nil
}
