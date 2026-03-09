package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	"github.com/friedenberg/grit/internal/git"
)

func registerCommitCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "commit",
		Title:       "Create Commit",
		Description: command.Description{Short: "Create a new commit with staged changes"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(false),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(false),
			OpenWorldHint:   protocol.BoolPtr(false),
		},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "message", Type: command.String, Description: "Commit message", Required: true},
			{Name: "amend", Type: command.Bool, Description: "Amend the previous commit instead of creating a new one"},
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
		Amend    bool   `json:"amend"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	gitArgs := []string{"commit"}

	if params.Amend {
		gitArgs = append(gitArgs, "--amend")
	}

	gitArgs = append(gitArgs, "-m", params.Message)

	out, err := git.Run(ctx, params.RepoPath, gitArgs...)
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("git commit: %v", err)), nil
	}

	result := git.ParseCommit(out)

	return command.JSONResult(result), nil
}
