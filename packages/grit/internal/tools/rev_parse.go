package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/friedenberg/grit/internal/git"
)

func registerRevParseCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "git_rev_parse",
		Description: command.Description{Short: "Resolve a git revision to its full SHA, or resolve special names like HEAD, branch names, tags, and relative refs (e.g. HEAD~3, main^2)"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "ref", Type: command.String, Description: "Ref to resolve (e.g. HEAD, main, v1.0, HEAD~3, abc1234)", Required: true},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git rev-parse"}, UseWhen: "resolving a git revision to its full SHA"},
		},
		Run: handleGitRevParse,
	})
}

func handleGitRevParse(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		RepoPath string `json:"repo_path"`
		Ref      string `json:"ref"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	out, err := git.Run(ctx, params.RepoPath, "rev-parse", "--verify", params.Ref)
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("git rev-parse: %v", err)), nil
	}

	return command.JSONResult(git.RevParseResult{
		Resolved: strings.TrimSpace(out),
		Ref:      params.Ref,
	}), nil
}
