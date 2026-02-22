package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	"github.com/friedenberg/get-hubbed/internal/gh"
)

func registerRunCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "run_list",
		Title:       "List Workflow Runs",
		Description: command.Description{Short: "List recent workflow runs"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(true),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(true),
			OpenWorldHint:   protocol.BoolPtr(true),
		},
		Params: []command.Param{
			{Name: "repo", Type: command.String, Description: "Repository in OWNER/REPO format", Required: true},
			{Name: "branch", Type: command.String, Description: "Filter runs by branch"},
			{Name: "status", Type: command.String, Description: "Filter runs by status: queued, completed, in_progress, requested, waiting, pending, action_required, cancelled, failure, neutral, skipped, stale, startup_failure, success, timed_out"},
			{Name: "workflow", Type: command.String, Description: "Filter runs by workflow name or filename"},
			{Name: "event", Type: command.String, Description: "Filter runs by triggering event (e.g. push, pull_request)"},
			{Name: "commit", Type: command.String, Description: "Filter runs by commit SHA"},
			{Name: "user", Type: command.String, Description: "Filter runs by user who triggered the run"},
			{Name: "limit", Type: command.Int, Description: "Maximum number of runs to fetch (default 20)"},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"gh run list"}, UseWhen: "listing workflow runs"},
		},
		Run: handleRunList,
	})

	app.AddCommand(&command.Command{
		Name:        "run_view",
		Title:       "View Workflow Run",
		Description: command.Description{Short: "View a workflow run with jobs and steps"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(true),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(true),
			OpenWorldHint:   protocol.BoolPtr(true),
		},
		Params: []command.Param{
			{Name: "repo", Type: command.String, Description: "Repository in OWNER/REPO format", Required: true},
			{Name: "run_id", Type: command.Int, Description: "Workflow run ID", Required: true},
			{Name: "attempt", Type: command.Int, Description: "The attempt number of the workflow run"},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"gh run view"}, UseWhen: "viewing workflow run details"},
		},
		Run: handleRunView,
	})

	app.AddCommand(&command.Command{
		Name:        "run_log",
		Title:       "View Run Logs",
		Description: command.Description{Short: "Get logs for failed steps in a workflow run or specific job"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(true),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(true),
			OpenWorldHint:   protocol.BoolPtr(true),
		},
		Params: []command.Param{
			{Name: "repo", Type: command.String, Description: "Repository in OWNER/REPO format", Required: true},
			{Name: "run_id", Type: command.Int, Description: "Workflow run ID", Required: true},
			{Name: "job_id", Type: command.Int, Description: "Specific job ID to get logs for (if omitted, shows all failed step logs)"},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"gh run view"}, UseWhen: "viewing workflow run logs"},
		},
		Run: handleRunLog,
	})
}

func handleRunList(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
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
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
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
		return command.TextErrorResult(fmt.Sprintf("gh run list: %v", err)), nil
	}

	return command.TextResult(out), nil
}

func handleRunView(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		Repo    string `json:"repo"`
		RunID   int64  `json:"run_id"`
		Attempt int    `json:"attempt"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
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
		return command.TextErrorResult(fmt.Sprintf("gh run view: %v", err)), nil
	}

	return command.TextResult(out), nil
}

func handleRunLog(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		Repo  string `json:"repo"`
		RunID int64  `json:"run_id"`
		JobID int64  `json:"job_id"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
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
		return command.TextErrorResult(fmt.Sprintf("gh run view log: %v", err)), nil
	}

	if out == "" {
		return command.TextResult("No failed step logs found for this run."), nil
	}

	return command.TextResult(out), nil
}
