package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/friedenberg/grit/internal/git"
)

func registerCommitCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "commit",
		Description: command.Description{Short: "Create a new commit with staged changes"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "message", Type: command.String, Description: "Commit message", Required: true},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git commit"}, UseWhen: "creating a new commit"},
		},
		Run: handleGitCommit,
	})
}

func handleGitCommit(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		RepoPath string `json:"repo_path"`
		Message  string `json:"message"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	out, err := git.Run(ctx, params.RepoPath, "commit", "-m", params.Message)
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("git commit: %v", err)), nil
	}

	result := git.ParseCommit(out)

	return command.JSONResult(result), nil
}
