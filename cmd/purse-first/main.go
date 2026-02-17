package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/amarbel-llc/purse-first/internal/hook"
	"github.com/amarbel-llc/purse-first/internal/install"
	"github.com/amarbel-llc/purse-first/internal/localplugin"
	"github.com/amarbel-llc/purse-first/internal/marketplace"
	"github.com/amarbel-llc/purse-first/internal/validate"
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

	uninstallHooksCmd := &cobra.Command{
		Use:   "uninstall-hooks",
		Short: "Remove purse-first hook entries from Claude Code settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			return hook.Uninstall(false)
		},
	}

	var installHooks bool

	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install purse-first marketplace and plugins into Claude Code",
		RunE: func(cmd *cobra.Command, args []string) error {
			return install.Run(os.Stderr, install.Options{NoHooks: !installHooks})
		},
	}

	installCmd.Flags().BoolVar(&installHooks, "hooks", false, "install purse-first hooks into settings (default: hooks are removed)")

	var (
		pluginsDir    string
		configPath    string
		outputPath    string
		genNoHooks    bool
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

			m := marketplace.Generate(config, discovered, marketplace.GenerateOptions{
				StripHooks: genNoHooks,
			})

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
	genMarketplaceCmd.Flags().BoolVar(&genNoHooks, "no-hooks", false, "strip hooks from generated marketplace plugins")
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

	var (
		validateType   string
		validateStrict bool
	)

	validateCmd := &cobra.Command{
		Use:   "validate [path]",
		Short: "Validate plugin, mapping, or marketplace documents",
		Long: `Validate Claude Code plugin documents.

Accepts a file path, directory, or "-" for stdin.
Auto-detects document type from filename or content.
Use --type to override detection. Use --strict to promote warnings to errors.`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			docType := parseDocType(validateType)
			if validateType != "" && docType == validate.Unknown {
				return fmt.Errorf("unknown type %q; use plugin, mapping, or marketplace", validateType)
			}

			path := "."
			if len(args) > 0 {
				path = args[0]
			}

			if path == "-" {
				return validateStdin(docType, validateStrict)
			}

			info, err := os.Stat(path)
			if err != nil {
				return fmt.Errorf("cannot access %s: %w", path, err)
			}

			if info.IsDir() {
				return validateDir(path, validateStrict)
			}

			return validateFile(path, docType, validateStrict)
		},
	}

	validateCmd.Flags().StringVar(&validateType, "type", "", "document type: plugin, mapping, marketplace")
	validateCmd.Flags().BoolVar(&validateStrict, "strict", false, "promote warnings to errors")

	root.AddCommand(hookCmd, postHookCmd, sessionEndCmd, installCmd, uninstallHooksCmd, genMarketplaceCmd, genLocalPluginCmd, validateCmd)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func parseDocType(s string) validate.DocType {
	switch s {
	case "plugin":
		return validate.PluginDoc
	case "mapping":
		return validate.MappingDoc
	case "marketplace":
		return validate.MarketplaceDoc
	default:
		return validate.Unknown
	}
}

func validateStdin(docType validate.DocType, strict bool) error {
	if docType == validate.Unknown {
		return fmt.Errorf("--type is required when reading from stdin")
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}

	r, dt, err := validate.ValidateBytes(data, docType, strict)
	if err != nil {
		return err
	}

	return reportResult(r, dt, "<stdin>", strict)
}

func validateFile(path string, docType validate.DocType, strict bool) error {
	r, dt, err := validate.ValidateFile(path, docType, strict)
	if err != nil {
		return err
	}

	return reportResult(r, dt, path, strict)
}

func validateDir(dir string, strict bool) error {
	r, err := validate.ValidateDirectory(dir, strict)
	if err != nil {
		return err
	}

	for _, issue := range r.Issues() {
		fmt.Fprintf(os.Stderr, "%s\n", issue)
	}

	if r.HasErrors() {
		return fmt.Errorf("validation failed")
	}

	if strict && r.HasWarnings() {
		return fmt.Errorf("validation failed (strict mode)")
	}

	fmt.Fprintf(os.Stderr, "valid: %s\n", dir)
	return nil
}

func reportResult(r *validate.Result, dt validate.DocType, path string, strict bool) error {
	for _, issue := range r.Issues() {
		fmt.Fprintf(os.Stderr, "%s: %s\n", path, issue)
	}

	if r.HasErrors() {
		return fmt.Errorf("validation failed")
	}

	if strict && r.HasWarnings() {
		return fmt.Errorf("validation failed (strict mode)")
	}

	fmt.Fprintf(os.Stderr, "valid %s: %s\n", dt, path)
	return nil
}
