package graphqlschema

import (
	"encoding/json"
	"strings"

	"github.com/amarbel-llc/mgp/internal/catalog"
	"github.com/graphql-go/graphql"
)

func BuildSchema(cat *catalog.Catalog) (graphql.Schema, error) {
	toolType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Tool",
		Fields: graphql.Fields{
			"name":    &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"title":   &graphql.Field{Type: graphql.String},
			"description": &graphql.Field{Type: graphql.String},
			"package": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"inputSchema": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (any, error) {
					tool := p.Source.(catalog.CatalogTool)
					if tool.InputSchema == nil {
						return nil, nil
					}
					return string(tool.InputSchema), nil
				},
			},
			"readOnly": &graphql.Field{
				Type: graphql.Boolean,
				Resolve: func(p graphql.ResolveParams) (any, error) {
					tool := p.Source.(catalog.CatalogTool)
					if tool.ReadOnly == nil {
						return nil, nil
					}
					return *tool.ReadOnly, nil
				},
			},
			"destructive": &graphql.Field{
				Type: graphql.Boolean,
				Resolve: func(p graphql.ResolveParams) (any, error) {
					tool := p.Source.(catalog.CatalogTool)
					if tool.Destructive == nil {
						return nil, nil
					}
					return *tool.Destructive, nil
				},
			},
			"idempotent": &graphql.Field{
				Type: graphql.Boolean,
				Resolve: func(p graphql.ResolveParams) (any, error) {
					tool := p.Source.(catalog.CatalogTool)
					if tool.Idempotent == nil {
						return nil, nil
					}
					return *tool.Idempotent, nil
				},
			},
			"openWorld": &graphql.Field{
				Type: graphql.Boolean,
				Resolve: func(p graphql.ResolveParams) (any, error) {
					tool := p.Source.(catalog.CatalogTool)
					if tool.OpenWorld == nil {
						return nil, nil
					}
					return *tool.OpenWorld, nil
				},
			},
		},
	})

	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"tools": &graphql.Field{
				Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(toolType))),
				Args: graphql.FieldConfigArgument{
					"package":     &graphql.ArgumentConfig{Type: graphql.String},
					"name":        &graphql.ArgumentConfig{Type: graphql.String},
					"readOnly":    &graphql.ArgumentConfig{Type: graphql.Boolean},
					"destructive": &graphql.ArgumentConfig{Type: graphql.Boolean},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					var result []catalog.CatalogTool

					pkgFilter, _ := p.Args["package"].(string)
					nameFilter, _ := p.Args["name"].(string)
					readOnlyFilter, hasReadOnly := p.Args["readOnly"].(bool)
					destructiveFilter, hasDestructive := p.Args["destructive"].(bool)

					for _, tool := range cat.Tools {
						if pkgFilter != "" && tool.Package != pkgFilter {
							continue
						}
						if nameFilter != "" && !strings.Contains(tool.Name, nameFilter) {
							continue
						}
						if hasReadOnly && (tool.ReadOnly == nil || *tool.ReadOnly != readOnlyFilter) {
							continue
						}
						if hasDestructive && (tool.Destructive == nil || *tool.Destructive != destructiveFilter) {
							continue
						}
						result = append(result, tool)
					}

					if result == nil {
						result = []catalog.CatalogTool{}
					}

					return result, nil
				},
			},
		},
	})

	return graphql.NewSchema(graphql.SchemaConfig{
		Query: queryType,
	})
}

func Execute(schema graphql.Schema, query string) (json.RawMessage, error) {
	result := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: query,
	})

	return json.Marshal(result)
}
