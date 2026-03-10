package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	"github.com/friedenberg/get-hubbed/internal/gh"
)

func registerPRCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "pr_list",
		Title:       "List Pull Requests",
		Description: command.Description{Short: "List pull requests in a repository"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(true),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(true),
			OpenWorldHint:   protocol.BoolPtr(true),
		},
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
		Name:        "pr_create",
		Title:       "Create Pull Request",
		Description: command.Description{Short: "Create a new pull request"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(false),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(false),
			OpenWorldHint:   protocol.BoolPtr(true),
		},
		Params: []command.Param{
			{Name: "repo", Type: command.String, Description: "Repository in OWNER/REPO format", Required: true},
			{Name: "title", Type: command.String, Description: "Pull request title", Required: true},
			{Name: "body", Type: command.String, Description: "Pull request body"},
			{Name: "base", Type: command.String, Description: "Base branch (the branch into which you want your code merged)"},
			{Name: "head", Type: command.String, Description: "Head branch (the branch that contains commits for your pull request). Usually necessary because gh refuses to create PRs when the working tree has untracked files unless --head is explicit. Format: OWNER:BRANCH for cross-repo PRs"},
			{Name: "draft", Type: command.Bool, Description: "Create as draft pull request"},
			{Name: "labels", Type: command.Array, Description: "Labels to add"},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"gh pr create"}, UseWhen: "creating pull requests"},
		},
		Run: handlePRCreate,
	})

	app.AddCommand(&command.Command{
		Name:        "pr_view",
		Title:       "View Pull Request",
		Description: command.Description{Short: "View pull request details"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(true),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(true),
			OpenWorldHint:   protocol.BoolPtr(true),
		},
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

func handlePRCreate(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		Repo   string   `json:"repo"`
		Title  string   `json:"title"`
		Body   string   `json:"body"`
		Base   string   `json:"base"`
		Head   string   `json:"head"`
		Draft  bool     `json:"draft"`
		Labels []string `json:"labels"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	ghArgs := []string{
		"pr", "create",
		"-R", params.Repo,
		"--title", params.Title,
	}

	if params.Body != "" {
		ghArgs = append(ghArgs, "--body", params.Body)
	}

	if params.Base != "" {
		ghArgs = append(ghArgs, "--base", params.Base)
	}

	if params.Head != "" {
		ghArgs = append(ghArgs, "--head", params.Head)
	}

	if params.Draft {
		ghArgs = append(ghArgs, "--draft")
	}

	for _, label := range params.Labels {
		ghArgs = append(ghArgs, "--label", label)
	}

	out, err := gh.Run(ctx, ghArgs...)
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("gh pr create: %v", err)), nil
	}

	return command.TextResult(out), nil
}
