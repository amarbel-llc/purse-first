package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/friedenberg/purse-first/internal/hook"
)

func main() {
	root := &cobra.Command{
		Use:   "purse-first",
		Short: "MCP-first tool routing for Claude Code",
	}

	hookCmd := &cobra.Command{
		Use:   "hook",
		Short: "PreToolUse hook handler (reads JSON from stdin)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			return hook.HandlePreToolUse(os.Stdin, os.Stdout, cwd)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	postHookCmd := &cobra.Command{
		Use:   "post-hook",
		Short: "PostToolUse hook handler (notifies lux of opened files)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return hook.HandlePostToolUse(os.Stdin, os.Stdout)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	sessionEndCmd := &cobra.Command{
		Use:   "session-end",
		Short: "SessionEnd hook handler (closes all lux documents)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return hook.HandleSessionEnd(os.Stdin, os.Stdout)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	var projectFlag bool

	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install purse-first hook into Claude Code settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			binaryPath, err := selfPath()
			if err != nil {
				return fmt.Errorf("finding binary path: %w", err)
			}

			if err := hook.Install(binaryPath, projectFlag); err != nil {
				return err
			}

			scope := "global (~/.claude/settings.json)"
			if projectFlag {
				scope = "project (.claude/settings.json)"
			}

			fmt.Fprintf(os.Stderr, "purse-first hook installed to %s\n", scope)
			return nil
		},
	}

	installCmd.Flags().BoolVar(&projectFlag, "project", false, "install to project settings instead of global")

	root.AddCommand(hookCmd, postHookCmd, sessionEndCmd, installCmd)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func selfPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}

	// Resolve symlinks to get the actual binary path
	resolved, err := exec.LookPath(exe)
	if err != nil {
		return exe, nil
	}

	return resolved, nil
}
