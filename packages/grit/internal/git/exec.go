package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/output"
)

func Run(ctx context.Context, dir string, args ...string) (string, error) {
	if strings.ContainsRune(dir, 0) {
		return "", fmt.Errorf("dir contains null byte")
	}

	for _, arg := range args {
		if strings.ContainsRune(arg, 0) {
			return "", fmt.Errorf("argument contains null byte")
		}
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_EDITOR=true",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		limited := output.LimitStderr(stderr.String())
		return "", fmt.Errorf("git %v: %w: %s", args, err, limited.Content)
	}

	return stdout.String(), nil
}
