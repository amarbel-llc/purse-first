package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/go-lib-mcp/protocol"
	"github.com/amarbel-llc/go-lib-mcp/server"
	"github.com/friedenberg/get-hubbed/internal/gh"
)

func registerPRTools(r *server.ToolRegistry) {
	r.Register(
		"pr_list",
		"List pull requests in a repository",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"repo": {
					"type": "string",
					"description": "Repository in OWNER/REPO format"
				},
				"state": {
					"type": "string",
					"description": "Filter by state: open, closed, merged, all (default open)",
					"enum": ["open", "closed", "merged", "all"]
				},
				"limit": {
					"type": "integer",
					"description": "Maximum number of pull requests to list (default 30)"
				}
			},
			"required": ["repo"]
		}`),
		handlePRList,
	)

	r.Register(
		"pr_view",
		"View pull request details",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"repo": {
					"type": "string",
					"description": "Repository in OWNER/REPO format"
				},
				"number": {
					"type": "integer",
					"description": "Pull request number"
				}
			},
			"required": ["repo", "number"]
		}`),
		handlePRView,
	)
}

func handlePRList(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
	var params struct {
		Repo  string `json:"repo"`
		State string `json:"state"`
		Limit int    `json:"limit"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	ghArgs := []string{
		"pr", "list",
		"-R", params.Repo,
		"--json", "number,title,state,author,baseRefName,headRefName,createdAt,updatedAt,url",
	}

	if params.State != "" {
		ghArgs = append(ghArgs, "--state", params.State)
	}

	if params.Limit > 0 {
		ghArgs = append(ghArgs, "--limit", fmt.Sprintf("%d", params.Limit))
	}

	out, err := gh.Run(ctx, ghArgs...)
	if err != nil {
		return protocol.ErrorResult(fmt.Sprintf("gh pr list: %v", err)), nil
	}

	return &protocol.ToolCallResult{
		Content: []protocol.ContentBlock{
			protocol.TextContent(out),
		},
	}, nil
}

func handlePRView(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
	var params struct {
		Repo   string `json:"repo"`
		Number int    `json:"number"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	out, err := gh.Run(ctx,
		"pr", "view", fmt.Sprintf("%d", params.Number),
		"-R", params.Repo,
		"--json", "number,title,state,body,author,baseRefName,headRefName,labels,reviewDecision,commits,comments,createdAt,updatedAt,url",
	)
	if err != nil {
		return protocol.ErrorResult(fmt.Sprintf("gh pr view: %v", err)), nil
	}

	return &protocol.ToolCallResult{
		Content: []protocol.ContentBlock{
			protocol.TextContent(out),
		},
	}, nil
}
