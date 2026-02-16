package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/amarbel-llc/purse-first/internal/hook"
	"github.com/amarbel-llc/purse-first/internal/localplugin"
	"github.com/amarbel-llc/purse-first/internal/marketplace"
	"github.com/amarbel-llc/purse-first/internal/mcp"
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

	var (
		pluginsDir string
		configPath string
		outputPath string
	)

	genMarketplaceCmd := &cobra.Command{
		Use:   "generate-marketplace",
		Short: "Generate .claude-plugin/marketplace.json from discovered plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := marketplace.ReadConfig(configPath)
			if err != nil {
				return fmt.Errorf("reading config: %w", err)
			}

			discovered, err := marketplace.DiscoverPlugins(pluginsDir)
			if err != nil {
				return fmt.Errorf("discovering plugins: %w", err)
			}

			m := marketplace.Generate(config, discovered)

			if err := marketplace.Write(m, outputPath); err != nil {
				return fmt.Errorf("writing marketplace.json: %w", err)
			}

			fmt.Fprintf(os.Stderr, "wrote %s (%d plugins)\n", outputPath, len(m.Plugins))
			return nil
		},
	}

	genMarketplaceCmd.Flags().StringVar(&pluginsDir, "plugins-dir", "", "directory containing plugin.json files")
	genMarketplaceCmd.Flags().StringVar(&configPath, "config", "", "marketplace config file with metadata")
	genMarketplaceCmd.Flags().StringVar(&outputPath, "output", ".claude-plugin/marketplace.json", "output path")
	genMarketplaceCmd.MarkFlagRequired("plugins-dir")
	genMarketplaceCmd.MarkFlagRequired("config")

	var localPluginRoot string

	genLocalPluginCmd := &cobra.Command{
		Use:   "generate-local-plugin",
		Short: "Discover skills and update .claude-plugin/plugin.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if localPluginRoot == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("getting working directory: %w", err)
				}
				localPluginRoot = cwd
			}

			pluginPath := filepath.Join(localPluginRoot, ".claude-plugin", "plugin.json")
			if err := localplugin.Generate(localPluginRoot, pluginPath); err != nil {
				return fmt.Errorf("generating local plugin: %w", err)
			}

			fmt.Fprintf(os.Stderr, "updated %s\n", pluginPath)
			return nil
		},
	}

	genLocalPluginCmd.Flags().StringVar(&localPluginRoot, "root", "", "repository root (defaults to cwd)")

	root.AddCommand(hookCmd, postHookCmd, sessionEndCmd, installCmd, genMarketplaceCmd, genLocalPluginCmd)

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
