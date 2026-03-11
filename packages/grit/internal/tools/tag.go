package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	"github.com/friedenberg/grit/internal/git"
)

func registerTagCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "tag_list",
		Title:       "List Tags",
		Description: command.Description{Short: "List tags with metadata"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(true),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(true),
			OpenWorldHint:   protocol.BoolPtr(false),
		},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "pattern", Type: command.String, Description: "Only list tags matching pattern (e.g. 'v1.*')"},
			{Name: "sort", Type: command.String, Description: "Sort key (e.g. '-creatordate' for newest first, 'creatordate' for oldest first)"},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git tag"}, UseWhen: "listing tags"},
		},
		Run: handleGitTagList,
	})

	app.AddCommand(&command.Command{
		Name:        "tag_verify",
		Title:       "Verify Tag Signature",
		Description: command.Description{Short: "Verify the GPG signature of a tag"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(true),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(true),
			OpenWorldHint:   protocol.BoolPtr(false),
		},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "name", Type: command.String, Description: "Tag name to verify", Required: true},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git tag -v", "git verify-tag"}, UseWhen: "verifying tag signatures"},
		},
		Run: handleGitTagVerify,
	})
}

func handleGitTagList(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		RepoPath string `json:"repo_path"`
		Pattern  string `json:"pattern"`
		Sort     string `json:"sort"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	gitArgs := []string{
		"tag",
		"--list",
		fmt.Sprintf("--format=%s", git.TagListFormat),
	}

	if params.Sort != "" {
		gitArgs = append(gitArgs, fmt.Sprintf("--sort=%s", params.Sort))
	}

	if params.Pattern != "" {
		gitArgs = append(gitArgs, params.Pattern)
	}

	out, err := git.Run(ctx, params.RepoPath, gitArgs...)
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("git tag: %v", err)), nil
	}

	tags := git.ParseTagList(out)

	return command.JSONResult(tags), nil
}

func handleGitTagVerify(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		RepoPath string `json:"repo_path"`
		Name     string `json:"name"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	stdout, stderr, err := git.RunBothOutputs(ctx, params.RepoPath, "tag", "-v", params.Name)

	result := git.ParseTagVerify(stdout, stderr, err)
	result.Name = params.Name

	return command.JSONResult(result), nil
}
