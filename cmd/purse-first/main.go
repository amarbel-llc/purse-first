package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/friedenberg/purse-first/internal/hook"
	"github.com/friedenberg/purse-first/internal/mcp"
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
		Short: "PostToolUse hook handler (fires plugin notifications)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return hook.HandlePostToolUse(os.Stdin, os.Stdout)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	sessionEndCmd := &cobra.Command{
		Use:   "session-end",
		Short: "SessionEnd hook handler (fires plugin stop notifications)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return hook.HandleSessionEnd(os.Stdin, os.Stdout)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	var projectFlag bool

	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install MCP servers and purse-first hook into Claude Code settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			binaryPath, err := selfPath()
			if err != nil {
				return fmt.Errorf("finding binary path: %w", err)
			}

			plugins, err := mcp.DiscoverPlugins()
			if err != nil {
				fmt.Fprintf(os.Stderr, "no plugins found, skipping MCP server install: %v\n", err)
			} else {
				count, err := mcp.InstallFromPlugins(plugins)
				if err != nil {
					return fmt.Errorf("installing MCP servers: %w", err)
				}
				fmt.Fprintf(os.Stderr, "installed %d MCP server(s) to ~/.claude.json\n", count)
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
