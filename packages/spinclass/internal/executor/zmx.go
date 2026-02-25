package executor

import (
	"os"
	"os/exec"
	"strings"

	tap "github.com/amarbel-llc/tap-dancer/go"
)

type ZmxExecutor struct{}

func (z ZmxExecutor) Attach(dir string, key string, command []string, dryRun bool, tp *tap.TestPoint) error {
	if len(command) == 0 {
		command = []string{os.Getenv("SHELL")}
	}

	args := []string{"-g", "spinclass", "attach", key}
	args = append(args, command...)

	if dryRun {
		tp.Skip = "dry run"
		tp.Diagnostics = &tap.Diagnostics{
			Extras: map[string]any{
				"command": "zmx " + strings.Join(args, " "),
			},
		}
		return nil
	}

	cmd := exec.Command("zmx", args...)
	cmd.Dir = dir
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
