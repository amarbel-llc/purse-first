package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/go-lib-mcp/protocol"
	"github.com/amarbel-llc/go-lib-mcp/server"
	"github.com/friedenberg/get-hubbed/internal/gh"
)

func registerIssueTools(r *server.ToolRegistry) {
	r.Register(
		"issue_list",
		"List issues in a repository",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"repo": {
					"type": "string",
					"description": "Repository in OWNER/REPO format"
				},
				"state": {
					"type": "string",
					"description": "Filter by state: open, closed, all (default open)",
					"enum": ["open", "closed", "all"]
				},
				"limit": {
					"type": "integer",
					"description": "Maximum number of issues to list (default 30)"
				},
				"labels": {
					"type": "array",
					"items": {"type": "string"},
					"description": "Filter by labels"
				}
			},
			"required": ["repo"]
		}`),
		handleIssueList,
	)

	r.Register(
		"issue_view",
		"View issue details",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"repo": {
					"type": "string",
					"description": "Repository in OWNER/REPO format"
				},
				"number": {
					"type": "integer",
					"description": "Issue number"
				}
			},
			"required": ["repo", "number"]
		}`),
		handleIssueView,
	)

	r.Register(
		"issue_create",
		"Create a new issue",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"repo": {
					"type": "string",
					"description": "Repository in OWNER/REPO format"
				},
				"title": {
					"type": "string",
					"description": "Issue title"
				},
				"body": {
					"type": "string",
					"description": "Issue body"
				},
				"labels": {
					"type": "array",
					"items": {"type": "string"},
					"description": "Labels to add"
				}
			},
			"required": ["repo", "title"]
		}`),
		handleIssueCreate,
	)
}

func handleIssueList(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
	var params struct {
		Repo   string   `json:"repo"`
		State  string   `json:"state"`
		Limit  int      `json:"limit"`
		Labels []string `json:"labels"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	ghArgs := []string{
		"issue", "list",
		"-R", params.Repo,
		"--json", "number,title,state,author,labels,createdAt,updatedAt,url",
	}

	if params.State != "" {
		ghArgs = append(ghArgs, "--state", params.State)
	}

	if params.Limit > 0 {
		ghArgs = append(ghArgs, "--limit", fmt.Sprintf("%d", params.Limit))
	}

	for _, label := range params.Labels {
		ghArgs = append(ghArgs, "--label", label)
	}

	out, err := gh.Run(ctx, ghArgs...)
	if err != nil {
		return protocol.ErrorResult(fmt.Sprintf("gh issue list: %v", err)), nil
	}

	return &protocol.ToolCallResult{
		Content: []protocol.ContentBlock{
			protocol.TextContent(out),
		},
	}, nil
}

func handleIssueView(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
	var params struct {
		Repo   string `json:"repo"`
		Number int    `json:"number"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	out, err := gh.Run(ctx,
		"issue", "view", fmt.Sprintf("%d", params.Number),
		"-R", params.Repo,
		"--json", "number,title,state,body,author,labels,assignees,comments,createdAt,updatedAt,url",
	)
	if err != nil {
		return protocol.ErrorResult(fmt.Sprintf("gh issue view: %v", err)), nil
	}

	return &protocol.ToolCallResult{
		Content: []protocol.ContentBlock{
			protocol.TextContent(out),
		},
	}, nil
}

func handleIssueCreate(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
	var params struct {
		Repo   string   `json:"repo"`
		Title  string   `json:"title"`
		Body   string   `json:"body"`
		Labels []string `json:"labels"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	ghArgs := []string{
		"issue", "create",
		"-R", params.Repo,
		"--title", params.Title,
	}

	if params.Body != "" {
		ghArgs = append(ghArgs, "--body", params.Body)
	}

	for _, label := range params.Labels {
		ghArgs = append(ghArgs, "--label", label)
	}

	out, err := gh.Run(ctx, ghArgs...)
	if err != nil {
		return protocol.ErrorResult(fmt.Sprintf("gh issue create: %v", err)), nil
	}

	return &protocol.ToolCallResult{
		Content: []protocol.ContentBlock{
			protocol.TextContent(out),
		},
	}, nil
}
