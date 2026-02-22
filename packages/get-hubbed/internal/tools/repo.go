package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	"github.com/friedenberg/get-hubbed/internal/gh"
)

func registerRepoCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "repo_view",
		Title:       "View Repository",
		Description: command.Description{Short: "View repository details"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(true),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(true),
			OpenWorldHint:   protocol.BoolPtr(true),
		},
		Params: []command.Param{
			{Name: "repo", Type: command.String, Description: "Repository in OWNER/REPO format", Required: true},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"gh repo view"}, UseWhen: "viewing repository details"},
		},
		Run: handleRepoView,
	})

	app.AddCommand(&command.Command{
		Name:        "repo_list",
		Title:       "List Repositories",
		Description: command.Description{Short: "List repositories for an owner"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(true),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(true),
			OpenWorldHint:   protocol.BoolPtr(true),
		},
		Params: []command.Param{
			{Name: "owner", Type: command.String, Description: "GitHub user or organization", Required: true},
			{Name: "limit", Type: command.Int, Description: "Maximum number of repositories to list (default 30)"},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"gh repo list"}, UseWhen: "listing repositories"},
		},
		Run: handleRepoList,
	})
}

func handleRepoView(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		Repo string `json:"repo"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	out, err := gh.Run(ctx,
		"repo", "view", params.Repo,
		"--json", "name,owner,description,url,defaultBranchRef,stargazerCount,forkCount,isPrivate,createdAt,updatedAt",
	)
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("gh repo view: %v", err)), nil
	}

	return command.TextResult(out), nil
}

func handleRepoList(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		Owner string `json:"owner"`
		Limit int    `json:"limit"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
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
		return command.TextErrorResult(fmt.Sprintf("gh repo list: %v", err)), nil
	}

	return command.TextResult(out), nil
}
