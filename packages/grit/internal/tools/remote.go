package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/friedenberg/grit/internal/git"
)

func registerRemoteCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "fetch",
		Description: command.Description{Short: "Fetch from a remote repository"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "remote", Type: command.String, Description: "Remote name (default origin)"},
			{Name: "prune", Type: command.Bool, Description: "Prune remote-tracking branches no longer on remote"},
			{Name: "all", Type: command.Bool, Description: "Fetch from all remotes"},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git fetch"}, UseWhen: "fetching from a remote"},
		},
		Run: handleGitFetch,
	})

	app.AddCommand(&command.Command{
		Name:        "pull",
		Description: command.Description{Short: "Pull changes from a remote repository"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "remote", Type: command.String, Description: "Remote name (default origin)"},
			{Name: "branch", Type: command.String, Description: "Remote branch to pull"},
			{Name: "rebase", Type: command.Bool, Description: "Rebase instead of merge"},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git pull"}, UseWhen: "pulling changes from a remote"},
		},
		Run: handleGitPull,
	})

	app.AddCommand(&command.Command{
		Name:        "push",
		Description: command.Description{Short: "Push commits to a remote repository (force push blocked on main/master)"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "remote", Type: command.String, Description: "Remote name (default origin)"},
			{Name: "branch", Type: command.String, Description: "Branch to push"},
			{Name: "set_upstream", Type: command.Bool, Description: "Set upstream tracking reference (-u)"},
			{Name: "force", Type: command.Bool, Description: "Force push (blocked on main/master branches)"},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git push"}, UseWhen: "pushing commits to a remote"},
		},
		Run: handleGitPush,
	})

	app.AddCommand(&command.Command{
		Name:        "remote_list",
		Description: command.Description{Short: "List remotes with their URLs"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git remote"}, UseWhen: "listing remotes"},
		},
		Run: handleGitRemoteList,
	})
}

func handleGitFetch(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		RepoPath string `json:"repo_path"`
		Remote   string `json:"remote"`
		Prune    bool   `json:"prune"`
		All      bool   `json:"all"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	gitArgs := []string{"fetch"}

	if params.Prune {
		gitArgs = append(gitArgs, "--prune")
	}

	if params.All {
		gitArgs = append(gitArgs, "--all")
	} else if params.Remote != "" {
		gitArgs = append(gitArgs, params.Remote)
	}

	if _, err := git.Run(ctx, params.RepoPath, gitArgs...); err != nil {
		return command.TextErrorResult(fmt.Sprintf("git fetch: %v", err)), nil
	}

	remote := params.Remote
	if remote == "" && !params.All {
		remote = "origin"
	}

	return command.JSONResult(git.MutationResult{
		Status: "fetched",
		Remote: remote,
		All:    params.All,
		Prune:  params.Prune,
	}), nil
}

func handleGitPull(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		RepoPath string `json:"repo_path"`
		Remote   string `json:"remote"`
		Branch   string `json:"branch"`
		Rebase   bool   `json:"rebase"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	gitArgs := []string{"pull"}

	if params.Rebase {
		gitArgs = append(gitArgs, "--rebase")
	}

	if params.Remote != "" {
		gitArgs = append(gitArgs, params.Remote)
	}

	if params.Branch != "" {
		gitArgs = append(gitArgs, params.Branch)
	}

	out, err := git.Run(ctx, params.RepoPath, gitArgs...)
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("git pull: %v", err)), nil
	}

	result := git.PullResult{
		Status:  "pulled",
		Summary: strings.TrimSpace(out),
	}

	if strings.Contains(out, "Already up to date") {
		result.Status = "already_up_to_date"
		result.Summary = ""
	}

	return command.JSONResult(result), nil
}

func handleGitPush(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		RepoPath    string `json:"repo_path"`
		Remote      string `json:"remote"`
		Branch      string `json:"branch"`
		SetUpstream bool   `json:"set_upstream"`
		Force       bool   `json:"force"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	if params.Force {
		branch := params.Branch
		if branch == "" {
			// Determine current branch to check protection
			branchOut, err := git.Run(ctx, params.RepoPath, "rev-parse", "--abbrev-ref", "HEAD")
			if err == nil {
				branch = strings.TrimSpace(branchOut)
			}
		}

		if branch == "main" || branch == "master" {
			return command.TextErrorResult("force push to main/master is blocked for safety"), nil
		}
	}

	gitArgs := []string{"push"}

	if params.Force {
		gitArgs = append(gitArgs, "--force")
	}

	if params.SetUpstream {
		gitArgs = append(gitArgs, "-u")
	}

	if params.Remote != "" {
		gitArgs = append(gitArgs, params.Remote)
	}

	if params.Branch != "" {
		gitArgs = append(gitArgs, params.Branch)
	}

	if _, err := git.Run(ctx, params.RepoPath, gitArgs...); err != nil {
		return command.TextErrorResult(fmt.Sprintf("git push: %v", err)), nil
	}

	return command.JSONResult(git.MutationResult{
		Status:      "pushed",
		Remote:      params.Remote,
		Branch:      params.Branch,
		SetUpstream: params.SetUpstream,
		Force:       params.Force,
	}), nil
}

func handleGitRemoteList(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		RepoPath string `json:"repo_path"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	out, err := git.Run(ctx, params.RepoPath, "remote", "-v")
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("git remote: %v", err)), nil
	}

	remotes := git.ParseRemoteList(out)

	return command.JSONResult(remotes), nil
}
