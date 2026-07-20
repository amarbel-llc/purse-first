package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"code.linenisgreat.com/purse-first/internal/packagetoml"
	"code.linenisgreat.com/purse-first/internal/validate"
)

func main() {
	root := &cobra.Command{
		Use:   "purse-first",
		Short: "Package framework for Claude Code",
	}

	var (
		genPluginRoot      string
		genPluginOutput    string
		genPluginSkillsDir string
	)

	genPluginCmd := &cobra.Command{
		Use:           "generate-plugin",
		Short:         "Generate plugin.json from package.toml",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := genPluginRoot
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("getting working directory: %w", err)
				}
				root = cwd
			}

			pkg, err := packagetoml.ParseFile(filepath.Join(root, "package.toml"))
			if err != nil {
				return fmt.Errorf("parsing package.toml: %w", err)
			}

			output := genPluginOutput
			if output == "" {
				output = root
			}

			if err := packagetoml.GeneratePluginJSON(pkg, output, genPluginSkillsDir); err != nil {
				return fmt.Errorf("generating plugin.json: %w", err)
			}

			pluginPath := filepath.Join(output, "share", "purse-first", pkg.Name, "plugin.json")
			fmt.Fprintf(os.Stderr, "generated %s\n", pluginPath)
			return nil
		},
	}

	genPluginCmd.Flags().StringVar(&genPluginRoot, "root", "", "package root containing package.toml (defaults to cwd)")
	genPluginCmd.Flags().StringVar(&genPluginOutput, "output", "", "output directory (defaults to root)")
	genPluginCmd.Flags().StringVar(&genPluginSkillsDir, "skills-dir", "", "directory containing skills to discover and copy")

	var (
		validateType   string
		validateStrict bool
	)

	validateCmd := &cobra.Command{
		Use:   "validate [path]",
		Short: "Validate package or mapping documents",
		Long: `Validate purse-first package documents.

Accepts a file path, directory, or "-" for stdin.
Auto-detects document type from filename or content.
Use --type to override detection. Use --strict to promote warnings to errors.`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			docType := parseDocType(validateType)
			if validateType != "" && docType == validate.Unknown {
				return fmt.Errorf("unknown type %q; use plugin, mapping, or mcp", validateType)
			}

			if docType == validate.MCPDoc {
				if len(args) == 0 {
					return fmt.Errorf("validate --type mcp requires a binary path")
				}
				return runValidateMCP(args[0])
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

	validateCmd.Flags().StringVar(&validateType, "type", "", "document type: plugin, mapping, mcp")
	validateCmd.Flags().BoolVar(&validateStrict, "strict", false, "promote warnings to errors")

	validateMCPCmd := &cobra.Command{
		Use:   "validate-mcp <binary> [args...]",
		Short: "Validate a running MCP server over stdio",
		Long: `Spawn an MCP server binary and validate its protocol responses.

Checks: initialize handshake, tools/list (non-empty, annotations present),
resources/list (schema), and resources/templates/list (schema).`,
		Args:          cobra.MinimumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runValidateMCP(args[0], args[1:]...)
		},
	}

	root.AddCommand(genPluginCmd, validateCmd, validateMCPCmd)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func runValidateMCP(binary string, args ...string) error {
	r, err := validate.ValidateMCP(context.Background(), binary, args...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		return err
	}

	for _, issue := range r.Issues() {
		fmt.Fprintf(os.Stderr, "%s\n", issue)
	}

	if r.HasErrors() {
		return fmt.Errorf("MCP validation failed")
	}

	fmt.Fprintf(os.Stderr, "valid mcp: %s\n", binary)
	return nil
}

func parseDocType(s string) validate.DocType {
	switch s {
	case "plugin":
		return validate.PluginDoc
	case "mapping":
		return validate.MappingDoc
	case "mcp":
		return validate.MCPDoc
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
