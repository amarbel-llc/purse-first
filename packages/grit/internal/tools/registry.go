package tools

import "github.com/amarbel-llc/purse-first/libs/go-mcp/command"

func RegisterAll() *command.App {
	app := command.NewApp("grit", "MCP server exposing git operations")
	app.Version = "0.1.0"

	registerStatusCommands(app)
	registerLogCommands(app)
	registerStagingCommands(app)
	registerCommitCommands(app)
	registerBranchCommands(app)
	registerRemoteCommands(app)
	registerRevParseCommands(app)
	registerRebaseCommands(app)

	return app
}
