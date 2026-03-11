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

func registerInteractiveRebaseCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "interactive_rebase_plan",
		Title:       "Plan Interactive Rebase",
		Description: command.Description{Short: "Get the commit list for an interactive rebase (blocked on main/master for safety)"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(true),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(true),
			OpenWorldHint:   protocol.BoolPtr(false),
		},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "upstream", Type: command.String, Description: "Ref to rebase onto (branch, tag, commit)", Required: true},
		},
		Run: handleInteractiveRebasePlan,
	})
}

func handleInteractiveRebasePlan(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		RepoPath string `json:"repo_path"`
		Upstream string `json:"upstream"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	// Determine current branch
	branchOut, err := git.Run(ctx, params.RepoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("failed to determine current branch: %v", err)), nil
	}
	branch := strings.TrimSpace(branchOut)

	// Safety: block on main/master
	if branch == "main" || branch == "master" {
		return command.TextErrorResult("interactive rebase on main/master is blocked for safety"), nil
	}

	// Get commits between upstream and HEAD in chronological order
	out, err := git.Run(ctx, params.RepoPath,
		"log", "--reverse", "--format=%H%x00%s",
		fmt.Sprintf("%s..HEAD", params.Upstream),
	)
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("git log: %v", err)), nil
	}

	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return command.JSONResult(git.InteractiveRebasePlan{
			Status:   "up_to_date",
			Branch:   branch,
			Upstream: params.Upstream,
			Commits:  []git.LogEntry{},
		}), nil
	}

	lines := strings.Split(trimmed, "\n")
	commits := make([]git.LogEntry, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(line, "\x00", 2)
		if len(parts) != 2 {
			continue
		}
		commits = append(commits, git.LogEntry{
			Hash:    parts[0],
			Subject: parts[1],
		})
	}

	return command.JSONResult(git.InteractiveRebasePlan{
		Status:   "plan",
		Branch:   branch,
		Upstream: params.Upstream,
		Commits:  commits,
	}), nil
}
