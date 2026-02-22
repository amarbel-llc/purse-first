package tools

import (
	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/server"
)

func RegisterAll() *command.App {
	app := command.NewApp("get-hubbed", "GitHub MCP server wrapping the gh CLI")
	app.Version = "0.1.0"

	registerRepoCommands(app)
	registerIssueCommands(app)
	registerPRCommands(app)
	registerRunCommands(app)
	registerContentCommands(app)

	return app
}

func RegisterAPITools(r *server.ToolRegistryV1) {
	registerAPITools(r)
}
