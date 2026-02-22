package executor

import (
	"os"
	"os/exec"
)

type ZmxExecutor struct{}

func (z ZmxExecutor) Attach(dir string, key string, command []string) error {
	args := []string{"attach", key}
	args = append(args, command...)

	cmd := exec.Command("zmx", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (z ZmxExecutor) Detach() error {
	cmd := exec.Command("zmx", "detach")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
