package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	"github.com/friedenberg/get-hubbed/internal/gh"
)

func registerIssueCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "issue_list",
		Title:       "List Issues",
		Description: command.Description{Short: "List issues in a repository"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(true),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(true),
			OpenWorldHint:   protocol.BoolPtr(true),
		},
		Params: []command.Param{
			{Name: "repo", Type: command.String, Description: "Repository in OWNER/REPO format", Required: true},
			{Name: "state", Type: command.String, Description: "Filter by state: open, closed, all (default open)"},
			{Name: "limit", Type: command.Int, Description: "Maximum number of issues to list (default 30)"},
			{Name: "labels", Type: command.Array, Description: "Filter by labels"},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"gh issue list"}, UseWhen: "listing issues"},
		},
		Run: handleIssueList,
	})

	app.AddCommand(&command.Command{
		Name:        "issue_view",
		Title:       "View Issue",
		Description: command.Description{Short: "View issue details"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(true),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(true),
			OpenWorldHint:   protocol.BoolPtr(true),
		},
		Params: []command.Param{
			{Name: "repo", Type: command.String, Description: "Repository in OWNER/REPO format", Required: true},
			{Name: "number", Type: command.Int, Description: "Issue number", Required: true},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"gh issue view"}, UseWhen: "viewing issue details"},
		},
		Run: handleIssueView,
	})

	app.AddCommand(&command.Command{
		Name:        "issue_create",
		Title:       "Create Issue",
		Description: command.Description{Short: "Create a new issue"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(false),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(false),
			OpenWorldHint:   protocol.BoolPtr(true),
		},
		Params: []command.Param{
			{Name: "repo", Type: command.String, Description: "Repository in OWNER/REPO format", Required: true},
			{Name: "title", Type: command.String, Description: "Issue title", Required: true},
			{Name: "body", Type: command.String, Description: "Issue body"},
			{Name: "labels", Type: command.Array, Description: "Labels to add"},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"gh issue create"}, UseWhen: "creating issues"},
		},
		Run: handleIssueCreate,
	})
}

func handleIssueList(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		Repo   string   `json:"repo"`
		State  string   `json:"state"`
		Limit  int      `json:"limit"`
		Labels []string `json:"labels"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
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
		return command.TextErrorResult(fmt.Sprintf("gh issue list: %v", err)), nil
	}

	return command.TextResult(out), nil
}

func handleIssueView(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		Repo   string `json:"repo"`
		Number int    `json:"number"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	out, err := gh.Run(ctx,
		"issue", "view", fmt.Sprintf("%d", params.Number),
		"-R", params.Repo,
		"--json", "number,title,state,body,author,labels,assignees,comments,createdAt,updatedAt,url",
	)
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("gh issue view: %v", err)), nil
	}

	return command.TextResult(out), nil
}

func handleIssueCreate(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		Repo   string   `json:"repo"`
		Title  string   `json:"title"`
		Body   string   `json:"body"`
		Labels []string `json:"labels"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
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
		return command.TextErrorResult(fmt.Sprintf("gh issue create: %v", err)), nil
	}

	return command.TextResult(out), nil
}
