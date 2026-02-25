package tools

import (
	"log"

	"github.com/amarbel-llc/mgp/internal/catalog"
	"github.com/amarbel-llc/mgp/internal/graphqlschema"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
)

func RegisterAll(cat *catalog.Catalog) *command.App {
	app := command.NewApp("mgp", "Model graph protocol — query and execute MCP tools via GraphQL")
	app.Version = "0.1.0"

	schema, err := graphqlschema.BuildSchema(cat)
	if err != nil {
		log.Fatalf("building graphql schema: %v", err)
	}

	registerQueryCommand(app, cat, schema)
	registerExecCommand(app, cat)

	return app
}
