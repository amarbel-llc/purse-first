package command

import (
	"github.com/amarbel-llc/go-lib-mcp/server"
)

// RegisterMCPTools registers all non-hidden commands as MCP tools
// in the given ToolRegistry, using each command's description and
// auto-generated JSON schema.
func (a *App) RegisterMCPTools(registry *server.ToolRegistry) {
	for _, cmd := range a.AllCommands() {
		if cmd.Hidden {
			continue
		}
		if cmd.RunMCP == nil {
			continue
		}

		registry.Register(
			cmd.Name,
			cmd.Description.Short,
			cmd.InputSchema(),
			cmd.RunMCP,
		)
	}
}
