package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/friedenberg/get-hubbed/internal/gh"
)

func registerPRCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "pr_list",
		Description: command.Description{Short: "List pull requests in a repository"},
		Params: []command.Param{
			{Name: "repo", Type: command.String, Description: "Repository in OWNER/REPO format", Required: true},
			{Name: "state", Type: command.String, Description: "Filter by state: open, closed, merged, all (default open)"},
			{Name: "limit", Type: command.Int, Description: "Maximum number of pull requests to list (default 30)"},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"gh pr list"}, UseWhen: "listing pull requests"},
		},
		Run: handlePRList,
	})

	app.AddCommand(&command.Command{
		Name:        "pr_view",
		Description: command.Description{Short: "View pull request details"},
		Params: []command.Param{
			{Name: "repo", Type: command.String, Description: "Repository in OWNER/REPO format", Required: true},
			{Name: "number", Type: command.Int, Description: "Pull request number", Required: true},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"gh pr view"}, UseWhen: "viewing pull request details"},
		},
		Run: handlePRView,
	})
}

func handlePRList(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		Repo  string `json:"repo"`
		State string `json:"state"`
		Limit int    `json:"limit"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
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
		return command.TextErrorResult(fmt.Sprintf("gh pr list: %v", err)), nil
	}

	return command.TextResult(out), nil
}

func handlePRView(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		Repo   string `json:"repo"`
		Number int    `json:"number"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	out, err := gh.Run(ctx,
		"pr", "view", fmt.Sprintf("%d", params.Number),
		"-R", params.Repo,
		"--json", "number,title,state,body,author,baseRefName,headRefName,labels,reviewDecision,commits,comments,createdAt,updatedAt,url",
	)
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("gh pr view: %v", err)), nil
	}

	return command.TextResult(out), nil
}
