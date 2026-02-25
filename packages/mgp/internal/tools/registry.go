package tools

import (
	"github.com/amarbel-llc/mgp/internal/catalog"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
)

func RegisterAll(cat *catalog.Catalog) *command.App {
	app := command.NewApp("mgp", "Model graph protocol — query and execute MCP tools via GraphQL")
	app.Version = "0.1.0"
	return app
}
