package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/go-lib-mcp/protocol"
	"github.com/amarbel-llc/go-lib-mcp/server"
	"github.com/friedenberg/get-hubbed/internal/gh"
)

func registerRunTools(r *server.ToolRegistry) {
	r.Register(
		"run_list",
		"List recent workflow runs",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"repo": {
					"type": "string",
					"description": "Repository in OWNER/REPO format"
				},
				"branch": {
					"type": "string",
					"description": "Filter runs by branch"
				},
				"status": {
					"type": "string",
					"description": "Filter runs by status: queued, completed, in_progress, requested, waiting, pending, action_required, cancelled, failure, neutral, skipped, stale, startup_failure, success, timed_out"
				},
				"workflow": {
					"type": "string",
					"description": "Filter runs by workflow name or filename"
				},
				"event": {
					"type": "string",
					"description": "Filter runs by triggering event (e.g. push, pull_request)"
				},
				"commit": {
					"type": "string",
					"description": "Filter runs by commit SHA"
				},
				"user": {
					"type": "string",
					"description": "Filter runs by user who triggered the run"
				},
				"limit": {
					"type": "integer",
					"description": "Maximum number of runs to fetch (default 20)"
				}
			},
			"required": ["repo"]
		}`),
		handleRunList,
	)

	r.Register(
		"run_view",
		"View a workflow run with jobs and steps",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"repo": {
					"type": "string",
					"description": "Repository in OWNER/REPO format"
				},
				"run_id": {
					"type": "integer",
					"description": "Workflow run ID"
				},
				"attempt": {
					"type": "integer",
					"description": "The attempt number of the workflow run"
				}
			},
			"required": ["repo", "run_id"]
		}`),
		handleRunView,
	)

	r.Register(
		"run_log",
		"Get logs for failed steps in a workflow run or specific job",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"repo": {
					"type": "string",
					"description": "Repository in OWNER/REPO format"
				},
				"run_id": {
					"type": "integer",
					"description": "Workflow run ID"
				},
				"job_id": {
					"type": "integer",
					"description": "Specific job ID to get logs for (if omitted, shows all failed step logs)"
				}
			},
			"required": ["repo", "run_id"]
		}`),
		handleRunLog,
	)
}

func handleRunList(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
	var params struct {
		Repo     string `json:"repo"`
		Branch   string `json:"branch"`
		Status   string `json:"status"`
		Workflow string `json:"workflow"`
		Event    string `json:"event"`
		Commit   string `json:"commit"`
		User     string `json:"user"`
		Limit    int    `json:"limit"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	ghArgs := []string{
		"run", "list",
		"-R", params.Repo,
		"--json", "attempt,conclusion,createdAt,databaseId,displayTitle,event,headBranch,headSha,name,number,startedAt,status,updatedAt,url,workflowName",
	}

	if params.Branch != "" {
		ghArgs = append(ghArgs, "--branch", params.Branch)
	}

	if params.Status != "" {
		ghArgs = append(ghArgs, "--status", params.Status)
	}

	if params.Workflow != "" {
		ghArgs = append(ghArgs, "--workflow", params.Workflow)
	}

	if params.Event != "" {
		ghArgs = append(ghArgs, "--event", params.Event)
	}

	if params.Commit != "" {
		ghArgs = append(ghArgs, "--commit", params.Commit)
	}

	if params.User != "" {
		ghArgs = append(ghArgs, "--user", params.User)
	}

	if params.Limit > 0 {
		ghArgs = append(ghArgs, "--limit", fmt.Sprintf("%d", params.Limit))
	}

	out, err := gh.Run(ctx, ghArgs...)
	if err != nil {
		return protocol.ErrorResult(fmt.Sprintf("gh run list: %v", err)), nil
	}

	return &protocol.ToolCallResult{
		Content: []protocol.ContentBlock{
			protocol.TextContent(out),
		},
	}, nil
}

func handleRunView(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
	var params struct {
		Repo    string `json:"repo"`
		RunID   int64  `json:"run_id"`
		Attempt int    `json:"attempt"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	ghArgs := []string{
		"run", "view", fmt.Sprintf("%d", params.RunID),
		"-R", params.Repo,
		"--json", "attempt,conclusion,createdAt,databaseId,displayTitle,event,headBranch,headSha,jobs,name,number,startedAt,status,updatedAt,url,workflowDatabaseId,workflowName",
	}

	if params.Attempt > 0 {
		ghArgs = append(ghArgs, "--attempt", fmt.Sprintf("%d", params.Attempt))
	}

	out, err := gh.Run(ctx, ghArgs...)
	if err != nil {
		return protocol.ErrorResult(fmt.Sprintf("gh run view: %v", err)), nil
	}

	return &protocol.ToolCallResult{
		Content: []protocol.ContentBlock{
			protocol.TextContent(out),
		},
	}, nil
}

func handleRunLog(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
	var params struct {
		Repo  string `json:"repo"`
		RunID int64  `json:"run_id"`
		JobID int64  `json:"job_id"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	ghArgs := []string{
		"run", "view", fmt.Sprintf("%d", params.RunID),
		"-R", params.Repo,
		"--log-failed",
	}

	if params.JobID > 0 {
		ghArgs = append(ghArgs, "--job", fmt.Sprintf("%d", params.JobID))
	}

	out, err := gh.Run(ctx, ghArgs...)
	if err != nil {
		return protocol.ErrorResult(fmt.Sprintf("gh run view log: %v", err)), nil
	}

	if out == "" {
		return &protocol.ToolCallResult{
			Content: []protocol.ContentBlock{
				protocol.TextContent("No failed step logs found for this run."),
			},
		}, nil
	}

	return &protocol.ToolCallResult{
		Content: []protocol.ContentBlock{
			protocol.TextContent(out),
		},
	}, nil
}
