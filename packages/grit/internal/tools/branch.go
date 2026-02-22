package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/friedenberg/grit/internal/git"
)

func registerBranchCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "branch_list",
		Description: command.Description{Short: "List branches"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "remote", Type: command.Bool, Description: "List remote-tracking branches"},
			{Name: "all", Type: command.Bool, Description: "List both local and remote-tracking branches"},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git branch"}, UseWhen: "listing branches"},
		},
		Run: handleGitBranchList,
	})

	app.AddCommand(&command.Command{
		Name:        "branch_create",
		Description: command.Description{Short: "Create a new branch"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "name", Type: command.String, Description: "Name for the new branch", Required: true},
			{Name: "start_point", Type: command.String, Description: "Starting point for the new branch (commit, branch, tag)"},
		},
		Run: handleGitBranchCreate,
	})

	app.AddCommand(&command.Command{
		Name:        "checkout",
		Description: command.Description{Short: "Switch branches or restore working tree files"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "ref", Type: command.String, Description: "Branch name or ref to check out", Required: true},
			{Name: "create", Type: command.Bool, Description: "Create a new branch and check it out (-b)"},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git checkout", "git switch"}, UseWhen: "switching branches"},
		},
		Run: handleGitCheckout,
	})
}

func handleGitBranchList(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		RepoPath string `json:"repo_path"`
		Remote   bool   `json:"remote"`
		All      bool   `json:"all"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	gitArgs := []string{
		"branch",
		"--format=%(HEAD)\x1f%(refname:short)\x1f%(objectname:short)\x1f%(subject)\x1f%(upstream:short)\x1f%(upstream:track)\x1e",
	}

	if params.All {
		gitArgs = append(gitArgs, "-a")
	} else if params.Remote {
		gitArgs = append(gitArgs, "-r")
	}

	out, err := git.Run(ctx, params.RepoPath, gitArgs...)
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("git branch: %v", err)), nil
	}

	branches := git.ParseBranchList(out)

	return command.JSONResult(branches), nil
}

func handleGitBranchCreate(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		RepoPath   string `json:"repo_path"`
		Name       string `json:"name"`
		StartPoint string `json:"start_point"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	gitArgs := []string{"branch", params.Name}

	if params.StartPoint != "" {
		gitArgs = append(gitArgs, params.StartPoint)
	}

	if _, err := git.Run(ctx, params.RepoPath, gitArgs...); err != nil {
		return command.TextErrorResult(fmt.Sprintf("git branch create: %v", err)), nil
	}

	return command.JSONResult(git.MutationResult{
		Status:     "created",
		Name:       params.Name,
		StartPoint: params.StartPoint,
	}), nil
}

func handleGitCheckout(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		RepoPath string `json:"repo_path"`
		Ref      string `json:"ref"`
		Create   bool   `json:"create"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	gitArgs := []string{"checkout"}

	if params.Create {
		gitArgs = append(gitArgs, "-b")
	}

	gitArgs = append(gitArgs, params.Ref)

	if _, err := git.Run(ctx, params.RepoPath, gitArgs...); err != nil {
		return command.TextErrorResult(fmt.Sprintf("git checkout: %v", err)), nil
	}

	return command.JSONResult(git.MutationResult{
		Status: "switched",
		Ref:    params.Ref,
		Create: params.Create,
	}), nil
}
