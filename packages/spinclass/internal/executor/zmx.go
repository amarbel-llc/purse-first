package executor

import (
	"os"
	"os/exec"
)

type ZmxExecutor struct{}

func (z ZmxExecutor) Attach(dir string, key string, command []string) error {
	args := []string{"-g", "spinclass", "attach", key}
	args = append(args, command...)

	cmd := exec.Command("zmx", args...)
	cmd.Env = append(os.Environ(), "SPINCLASS_SESSION="+key)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (z ZmxExecutor) Detach() error {
	cmd := exec.Command("zmx", "-g", "spinclass", "detach")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
