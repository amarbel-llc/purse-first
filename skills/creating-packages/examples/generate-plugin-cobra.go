package main

import (
	"github.com/amarbel-llc/purse-first/purse"
	"github.com/spf13/cobra"
)

// generatePluginCmd generates a purse-first plugin manifest.
// Register with: rootCmd.AddCommand(generatePluginCmd)
var generatePluginCmd = &cobra.Command{
	Use:    "generate-plugin <output-dir>",
	Short:  "Generate purse-first plugin manifest",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := purse.NewPluginBuilder("my-mcp").
			Command("my-mcp").
			StdioTransport().
			Build()

		return purse.WritePlugin(args[0], p)
	},
}

// For MCP servers where the MCP mode is a subcommand:
var generatePluginWithArgsCmd = &cobra.Command{
	Use:    "generate-plugin <output-dir>",
	Short:  "Generate purse-first plugin manifest",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := purse.NewPluginBuilder("my-mcp").
			Command("my-mcp", "serve", "stdio").
			StdioTransport().
			Build()

		return purse.WritePlugin(args[0], p)
	},
}
