package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/server"
	"github.com/friedenberg/get-hubbed/internal/gh"
)

func registerAPITools(r *server.ToolRegistryV1) {
	r.Register(
		protocol.ToolV1{
			Name:        "api_get",
			Title:       "GitHub API GET",
			Description: "Make an authenticated GET request to the GitHub REST API",
			InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"endpoint": {
					"type": "string",
					"description": "REST API path, e.g. /repos/{owner}/{repo}/actions/runs"
				},
				"params": {
					"type": "object",
					"description": "Query string parameters as key-value pairs",
					"additionalProperties": {"type": "string"}
				},
				"headers": {
					"type": "array",
					"items": {"type": "string"},
					"description": "Additional headers in key:value format"
				},
				"paginate": {
					"type": "boolean",
					"description": "Auto-paginate results"
				}
			},
			"required": ["endpoint"]
		}`),
			Annotations: &protocol.ToolAnnotations{
				ReadOnlyHint:    protocol.BoolPtr(true),
				DestructiveHint: protocol.BoolPtr(false),
				IdempotentHint:  protocol.BoolPtr(true),
				OpenWorldHint:   protocol.BoolPtr(true),
			},
		},
		handleAPIGet,
	)

	r.Register(
		protocol.ToolV1{
			Name:        "graphql_query",
			Title:       "GitHub GraphQL Query",
			Description: "Execute a read-only GraphQL query against the GitHub API",
			InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "The GraphQL query string"
				},
				"variables": {
					"type": "object",
					"description": "GraphQL variables as key-value pairs",
					"additionalProperties": {}
				},
				"paginate": {
					"type": "boolean",
					"description": "Auto-paginate results (requires endCursor/pageInfo in query)"
				}
			},
			"required": ["query"]
		}`),
			Annotations: &protocol.ToolAnnotations{
				ReadOnlyHint:    protocol.BoolPtr(true),
				DestructiveHint: protocol.BoolPtr(false),
				IdempotentHint:  protocol.BoolPtr(true),
				OpenWorldHint:   protocol.BoolPtr(true),
			},
		},
		handleGraphQLQuery,
	)

	r.Register(
		protocol.ToolV1{
			Name:        "graphql_mutation",
			Title:       "GitHub GraphQL Mutation",
			Description: "Execute a GraphQL mutation against the GitHub API",
			InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "The GraphQL mutation string"
				},
				"variables": {
					"type": "object",
					"description": "GraphQL variables as key-value pairs",
					"additionalProperties": {}
				}
			},
			"required": ["query"]
		}`),
			Annotations: &protocol.ToolAnnotations{
				ReadOnlyHint:    protocol.BoolPtr(false),
				DestructiveHint: protocol.BoolPtr(true),
				IdempotentHint:  protocol.BoolPtr(false),
				OpenWorldHint:   protocol.BoolPtr(true),
			},
		},
		handleGraphQLMutation,
	)
}

func handleAPIGet(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResultV1, error) {
	var params struct {
		Endpoint string            `json:"endpoint"`
		Params   map[string]string `json:"params"`
		Headers  []string          `json:"headers"`
		Paginate bool              `json:"paginate"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return protocol.ErrorResultV1(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	ghArgs := []string{"api", params.Endpoint, "--method", "GET"}

	for k, v := range params.Params {
		ghArgs = append(ghArgs, "-f", fmt.Sprintf("%s=%s", k, v))
	}

	for _, h := range params.Headers {
		ghArgs = append(ghArgs, "-H", h)
	}

	if params.Paginate {
		ghArgs = append(ghArgs, "--paginate")
	}

	out, err := gh.Run(ctx, ghArgs...)
	if err != nil {
		return protocol.ErrorResultV1(fmt.Sprintf("gh api: %v", err)), nil
	}

	return &protocol.ToolCallResultV1{
		Content: []protocol.ContentBlockV1{
			protocol.TextContentV1(out),
		},
	}, nil
}

func handleGraphQLQuery(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResultV1, error) {
	var params struct {
		Query     string                 `json:"query"`
		Variables map[string]interface{} `json:"variables"`
		Paginate  bool                   `json:"paginate"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return protocol.ErrorResultV1(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	ghArgs := []string{"api", "graphql", "-f", fmt.Sprintf("query=%s", params.Query)}

	for k, v := range params.Variables {
		ghArgs = append(ghArgs, "-F", fmt.Sprintf("%s=%v", k, v))
	}

	if params.Paginate {
		ghArgs = append(ghArgs, "--paginate")
	}

	out, err := gh.Run(ctx, ghArgs...)
	if err != nil {
		return protocol.ErrorResultV1(fmt.Sprintf("gh api graphql: %v", err)), nil
	}

	return &protocol.ToolCallResultV1{
		Content: []protocol.ContentBlockV1{
			protocol.TextContentV1(out),
		},
	}, nil
}

func handleGraphQLMutation(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResultV1, error) {
	var params struct {
		Query     string                 `json:"query"`
		Variables map[string]interface{} `json:"variables"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return protocol.ErrorResultV1(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	ghArgs := []string{"api", "graphql", "-f", fmt.Sprintf("query=%s", params.Query)}

	for k, v := range params.Variables {
		ghArgs = append(ghArgs, "-F", fmt.Sprintf("%s=%v", k, v))
	}

	out, err := gh.Run(ctx, ghArgs...)
	if err != nil {
		return protocol.ErrorResultV1(fmt.Sprintf("gh api graphql mutation: %v", err)), nil
	}

	return &protocol.ToolCallResultV1{
		Content: []protocol.ContentBlockV1{
			protocol.TextContentV1(out),
		},
	}, nil
}
