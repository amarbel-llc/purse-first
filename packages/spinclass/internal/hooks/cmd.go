package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/amarbel-llc/spinclass/internal/worktree"
)

func NewHooksCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "hooks",
		Short:  "Handle PreToolUse hook for worktree boundary enforcement",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			boundary, err := detectBoundary(cwd)
			if err != nil {
				return nil
			}

			if boundary == "" {
				return nil
			}

			return Run(os.Stdin, os.Stdout, boundary)
		},
	}
}

func detectBoundary(cwd string) (string, error) {
	if !worktree.IsWorktree(cwd) {
		toplevel, err := gitToplevel(cwd)
		if err != nil {
			return "", err
		}
		if !worktree.IsWorktree(toplevel) {
			return "", nil
		}
		return toplevel, nil
	}

	return gitToplevel(cwd)
}

func gitToplevel(dir string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
