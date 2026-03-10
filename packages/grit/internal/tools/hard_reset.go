package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	"github.com/friedenberg/grit/internal/git"
)

func registerHardResetCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "hard_reset",
		Title:       "Hard Reset",
		Description: command.Description{Short: "Discard all changes and reset HEAD, index, and working tree to a ref (blocked on main/master for safety)"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(false),
			DestructiveHint: protocol.BoolPtr(true),
			IdempotentHint:  protocol.BoolPtr(true),
			OpenWorldHint:   protocol.BoolPtr(false),
		},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "ref", Type: command.String, Description: "Target ref (e.g. HEAD, origin/main, HEAD~3, a commit SHA)", Required: true},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git reset --hard"}, UseWhen: "discarding all changes and resetting to a ref"},
		},
		Run: handleGitHardReset,
	})
}

func handleGitHardReset(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		RepoPath string `json:"repo_path"`
		Ref      string `json:"ref"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	branchOut, err := git.Run(ctx, params.RepoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err == nil {
		branch := strings.TrimSpace(branchOut)
		if branch == "main" || branch == "master" {
			return command.TextErrorResult("hard reset on main/master is blocked for safety"), nil
		}
	}

	if _, err := git.Run(ctx, params.RepoPath, "reset", "--hard", params.Ref); err != nil {
		return command.TextErrorResult(fmt.Sprintf("git reset --hard: %v", err)), nil
	}

	return command.JSONResult(git.MutationResult{
		Status: "hard_reset",
		Ref:    params.Ref,
	}), nil
}
