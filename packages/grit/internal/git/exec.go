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

func RunWithEnv(ctx context.Context, dir string, extraEnv []string, args ...string) (string, error) {
	if strings.ContainsRune(dir, 0) {
		return "", fmt.Errorf("dir contains null byte")
	}

	for _, arg := range args {
		if strings.ContainsRune(arg, 0) {
			return "", fmt.Errorf("argument contains null byte")
		}
	}

	// Build set of env var names that extraEnv overrides
	overridden := make(map[string]bool)
	for _, env := range extraEnv {
		if k, _, ok := strings.Cut(env, "="); ok {
			overridden[k] = true
		}
	}

	// Base defaults — skip any that extraEnv overrides
	baseDefaults := []string{
		"GIT_TERMINAL_PROMPT=0",
		"GIT_EDITOR=true",
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	for _, d := range baseDefaults {
		if k, _, ok := strings.Cut(d, "="); ok && overridden[k] {
			continue
		}
		cmd.Env = append(cmd.Env, d)
	}
	cmd.Env = append(cmd.Env, extraEnv...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		limited := output.LimitStderr(stderr.String())
		return "", fmt.Errorf("git %v: %w: %s", args, err, limited.Content)
	}

	return stdout.String(), nil
}

func Run(ctx context.Context, dir string, args ...string) (string, error) {
	return RunWithEnv(ctx, dir, nil, args...)
}

func RunBothOutputs(ctx context.Context, dir string, args ...string) (string, string, error) {
	if strings.ContainsRune(dir, 0) {
		return "", "", fmt.Errorf("dir contains null byte")
	}

	for _, arg := range args {
		if strings.ContainsRune(arg, 0) {
			return "", "", fmt.Errorf("argument contains null byte")
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

	err := cmd.Run()

	return stdout.String(), stderr.String(), err
}
