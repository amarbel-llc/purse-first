package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/go-lib-mcp/protocol"
	"github.com/amarbel-llc/go-lib-mcp/server"
	"github.com/friedenberg/get-hubbed/internal/gh"
)

func registerRepoTools(r *server.ToolRegistry) {
	r.Register(
		"repo_view",
		"View repository details",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"repo": {
					"type": "string",
					"description": "Repository in OWNER/REPO format"
				}
			},
			"required": ["repo"]
		}`),
		handleRepoView,
	)

	r.Register(
		"repo_list",
		"List repositories for an owner",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"owner": {
					"type": "string",
					"description": "GitHub user or organization"
				},
				"limit": {
					"type": "integer",
					"description": "Maximum number of repositories to list (default 30)"
				}
			},
			"required": ["owner"]
		}`),
		handleRepoList,
	)
}

func handleRepoView(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
	var params struct {
		Repo string `json:"repo"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	out, err := gh.Run(ctx,
		"repo", "view", params.Repo,
		"--json", "name,owner,description,url,defaultBranchRef,stargazerCount,forkCount,isPrivate,createdAt,updatedAt",
	)
	if err != nil {
		return protocol.ErrorResult(fmt.Sprintf("gh repo view: %v", err)), nil
	}

	return &protocol.ToolCallResult{
		Content: []protocol.ContentBlock{
			protocol.TextContent(out),
		},
	}, nil
}

func handleRepoList(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
	var params struct {
		Owner string `json:"owner"`
		Limit int    `json:"limit"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	ghArgs := []string{
		"repo", "list", params.Owner,
		"--json", "name,owner,description,url,isPrivate,stargazerCount,updatedAt",
	}

	if params.Limit > 0 {
		ghArgs = append(ghArgs, "--limit", fmt.Sprintf("%d", params.Limit))
	}

	out, err := gh.Run(ctx, ghArgs...)
	if err != nil {
		return protocol.ErrorResult(fmt.Sprintf("gh repo list: %v", err)), nil
	}

	return &protocol.ToolCallResult{
		Content: []protocol.ContentBlock{
			protocol.TextContent(out),
		},
	}, nil
}
