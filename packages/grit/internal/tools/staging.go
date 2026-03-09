package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	"github.com/friedenberg/grit/internal/git"
)

func registerStagingCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "add",
		Title:       "Stage Files",
		Description: command.Description{Short: "Stage files for commit"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(false),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(true),
			OpenWorldHint:   protocol.BoolPtr(false),
		},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "paths", Type: command.Array, Description: "File paths to stage (relative to repo root)", Required: true},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git add"}, UseWhen: "staging files for commit"},
		},
		Run: handleGitAdd,
	})

	app.AddCommand(&command.Command{
		Name:        "reset",
		Title:       "Unstage Files",
		Description: command.Description{Short: "Unstage files (soft reset only, does not modify working tree)"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(false),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(true),
			OpenWorldHint:   protocol.BoolPtr(false),
		},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "paths", Type: command.Array, Description: "File paths to unstage (relative to repo root)"},
			{Name: "soft", Type: command.Bool, Description: "Soft reset: move HEAD back without changing index or working tree (use with ref)"},
			{Name: "ref", Type: command.String, Description: "Target ref for soft reset (e.g. HEAD~1, HEAD~3, a branch). Defaults to HEAD~1"},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git reset"}, UseWhen: "unstaging files"},
		},
		Run: handleGitReset,
	})
}

func handleGitAdd(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		RepoPath string   `json:"repo_path"`
		Paths    []string `json:"paths"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	gitArgs := []string{"add", "--"}
	gitArgs = append(gitArgs, params.Paths...)

	if _, err := git.Run(ctx, params.RepoPath, gitArgs...); err != nil {
		return command.TextErrorResult(fmt.Sprintf("git add: %v", err)), nil
	}

	return command.JSONResult(git.MutationResult{
		Status: "staged",
		Paths:  params.Paths,
	}), nil
}

func handleGitReset(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		RepoPath string   `json:"repo_path"`
		Paths    []string `json:"paths"`
		Soft     bool     `json:"soft"`
		Ref      string   `json:"ref"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	if params.Soft {
		ref := params.Ref
		if ref == "" {
			ref = "HEAD~1"
		}

		if _, err := git.Run(ctx, params.RepoPath, "reset", "--soft", ref); err != nil {
			return command.TextErrorResult(fmt.Sprintf("git reset --soft: %v", err)), nil
		}

		return command.JSONResult(git.MutationResult{
			Status: "soft_reset",
			Ref:    ref,
		}), nil
	}

	if len(params.Paths) == 0 {
		return command.TextErrorResult("paths is required when not using soft reset"), nil
	}

	gitArgs := []string{"reset", "HEAD", "--"}
	gitArgs = append(gitArgs, params.Paths...)

	if _, err := git.Run(ctx, params.RepoPath, gitArgs...); err != nil {
		return command.TextErrorResult(fmt.Sprintf("git reset: %v", err)), nil
	}

	return command.JSONResult(git.MutationResult{
		Status: "unstaged",
		Paths:  params.Paths,
	}), nil
}
