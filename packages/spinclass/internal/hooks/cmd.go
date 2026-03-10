package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/amarbel-llc/spinclass/internal/git"
	"github.com/amarbel-llc/spinclass/internal/sweatfile"
	"github.com/amarbel-llc/spinclass/internal/worktree"
)

func NewHooksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "hooks",
		Short:  "Handle PreToolUse hook for worktree boundary enforcement",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return nil
			}

			if !worktree.IsWorktree(cwd) {
				toplevel, err := gitToplevel(cwd)
				if err != nil {
					return nil
				}
				if !worktree.IsWorktree(toplevel) {
					// Not in a worktree — run with flag off
					return Run(os.Stdin, os.Stdout, "", false)
				}
				cwd = toplevel
			}

			mainRepoRoot, err := git.CommonDir(cwd)
			if err != nil {
				return nil
			}

			home, _ := os.UserHomeDir()
			var disallowMainWorktree bool
			if home != "" {
				result, err := sweatfile.LoadWorktreeHierarchy(home, mainRepoRoot, cwd)
				if err == nil {
					disallowMainWorktree = result.Merged.DisallowMainWorktreeEnabled()
				}
			}

			return Run(os.Stdin, os.Stdout, mainRepoRoot, disallowMainWorktree)
		},
	}

	return cmd
}

func gitToplevel(dir string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
