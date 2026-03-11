package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	"github.com/friedenberg/grit/internal/git"
)

func registerTryCommitCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:  "try_commit",
		Title: "Try Commit",
		Description: command.Description{
			Short: "Stage, commit, and return context in a single call. Replaces the status, diff, log, add, commit multi-tool cycle. Use this instead of calling those tools individually when creating commits in independent agent loops.",
		},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(false),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(false),
			OpenWorldHint:   protocol.BoolPtr(false),
		},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "message", Type: command.String, Description: "Commit message", Required: true},
			{Name: "paths", Type: command.Array, Description: "File paths to stage before committing", Required: true},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git commit"}, UseWhen: "creating a new commit"},
		},
		Run: handleTryCommit,
	})
}

func handleTryCommit(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		RepoPath string   `json:"repo_path"`
		Message  string   `json:"message"`
		Paths    []string `json:"paths"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	// Stage files
	addArgs := []string{"add", "--"}
	addArgs = append(addArgs, params.Paths...)

	if _, err := git.Run(ctx, params.RepoPath, addArgs...); err != nil {
		return command.TextErrorResult(fmt.Sprintf("git add: %v", err)), nil
	}

	// Capture staged diff stats before committing
	numstatOut, err := git.Run(ctx, params.RepoPath, "diff", "--numstat", "--cached")
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("git diff --numstat: %v", err)), nil
	}

	staged := git.ParseDiffNumstat(numstatOut)

	// Commit
	commitOut, commitErr := git.Run(ctx, params.RepoPath, "commit", "-m", params.Message)

	// Capture post-commit (or post-failure) status
	statusOut, err := git.Run(ctx, params.RepoPath, "status", "--porcelain=v2", "--branch")
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("git status: %v", err)), nil
	}

	status := git.ParseStatus(statusOut)

	state, err := git.DetectInProgressState(ctx, params.RepoPath)
	if err == nil && state != nil {
		status.State = state
	}

	// If commit failed, return structured result with empty commit + status context
	if commitErr != nil {
		return command.JSONResult(git.TryCommitResult{
			Staged: staged,
			Status: status,
		}), nil
	}

	return command.JSONResult(git.TryCommitResult{
		Commit: git.ParseCommit(commitOut),
		Staged: staged,
		Status: status,
	}), nil
}
