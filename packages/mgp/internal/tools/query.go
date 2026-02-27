package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/mgp/internal/catalog"
	"github.com/amarbel-llc/mgp/internal/graphqlschema"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	"github.com/graphql-go/graphql"
)

func registerQueryCommand(app *command.App, cat *catalog.Catalog, schema graphql.Schema) {
	app.AddCommand(&command.Command{
		Name:        "query",
		Title:       "Query Tool Catalog",
		Description: command.Description{Short: "Query the purse-first tool catalog using GraphQL"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:   protocol.BoolPtr(true),
			IdempotentHint: protocol.BoolPtr(true),
		},
		Params: []command.Param{
			{Name: "query", Type: command.String, Description: "GraphQL query string", Required: true},
		},
		Run: func(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
			var params struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
			}

			// Forward to remote GraphQL server if available
			if cat.GraphQLClient != nil {
				result, err := cat.GraphQLClient.Query(ctx, params.Query, nil)
				if err != nil {
					return command.TextErrorResult(fmt.Sprintf("graphql query error: %v", err)), nil
				}
				return command.TextResult(string(result)), nil
			}

			result, err := graphqlschema.Execute(schema, params.Query)
			if err != nil {
				return command.TextErrorResult(fmt.Sprintf("graphql execution error: %v", err)), nil
			}

			return command.TextResult(string(result)), nil
		},
	})
}
