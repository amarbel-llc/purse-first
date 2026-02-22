package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/friedenberg/grit/internal/git"
)

func registerLogCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "log",
		Description: command.Description{Short: "Show commit history as structured JSON"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "max_count", Type: command.Int, Description: "Maximum number of commits to show (default 10)"},
			{Name: "ref", Type: command.String, Description: "Starting ref (commit, branch, tag)"},
			{Name: "paths", Type: command.Array, Description: "Limit to commits affecting these paths"},
			{Name: "all", Type: command.Bool, Description: "Show commits from all branches"},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git log"}, UseWhen: "viewing commit history"},
		},
		Run: handleGitLog,
	})

	app.AddCommand(&command.Command{
		Name:        "show",
		Description: command.Description{Short: "Show a commit, tag, or other git object"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "ref", Type: command.String, Description: "Ref to show (commit hash, tag, branch, etc.)", Required: true},
			{Name: "context_lines", Type: command.Int, Description: "Number of context lines around each change (git --unified=N, default 3)"},
			{Name: "max_patch_lines", Type: command.Int, Description: "Maximum number of patch output lines. Output is truncated with a truncated flag when exceeded."},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git show"}, UseWhen: "inspecting commits or objects"},
		},
		Run: handleGitShow,
	})

	app.AddCommand(&command.Command{
		Name:        "blame",
		Description: command.Description{Short: "Show line-by-line authorship of a file"},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "path", Type: command.String, Description: "File path to blame (relative to repo root)", Required: true},
			{Name: "ref", Type: command.String, Description: "Blame at a specific ref"},
			{Name: "line_range", Type: command.String, Description: "Line range in format START,END (e.g. '10,20')"},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git blame"}, UseWhen: "viewing line-by-line authorship"},
		},
		Run: handleGitBlame,
	})
}

func handleGitLog(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		RepoPath string   `json:"repo_path"`
		MaxCount int      `json:"max_count"`
		Ref      string   `json:"ref"`
		Paths    []string `json:"paths"`
		All      bool     `json:"all"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	gitArgs := []string{"log"}

	maxCount := params.MaxCount
	if maxCount <= 0 {
		maxCount = 10
	}
	gitArgs = append(gitArgs, fmt.Sprintf("--max-count=%d", maxCount))
	gitArgs = append(gitArgs, fmt.Sprintf("--format=%s", git.LogFormat))

	if params.All {
		gitArgs = append(gitArgs, "--all")
	}

	if params.Ref != "" {
		gitArgs = append(gitArgs, params.Ref)
	}

	if len(params.Paths) > 0 {
		gitArgs = append(gitArgs, "--")
		gitArgs = append(gitArgs, params.Paths...)
	}

	out, err := git.Run(ctx, params.RepoPath, gitArgs...)
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("git log: %v", err)), nil
	}

	entries := git.ParseLog(out)

	return command.JSONResult(entries), nil
}

func handleGitShow(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		RepoPath      string `json:"repo_path"`
		Ref           string `json:"ref"`
		ContextLines  *int   `json:"context_lines"`
		MaxPatchLines int    `json:"max_patch_lines"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	metadataOut, err := git.Run(ctx, params.RepoPath, "show", "--no-patch", fmt.Sprintf("--format=%s", git.ShowFormat), params.Ref)
	if err != nil {
		// Fall back to raw output for non-commit objects (tags, blobs)
		out, fallbackErr := git.Run(ctx, params.RepoPath, "show", params.Ref)
		if fallbackErr != nil {
			return command.TextErrorResult(fmt.Sprintf("git show: %v", err)), nil
		}
		return command.TextResult(out), nil
	}

	numstatOut, err := git.Run(ctx, params.RepoPath, "show", "--numstat", "--format=", params.Ref)
	if err != nil {
		numstatOut = ""
	}

	diffArgs := []string{"diff"}
	if params.ContextLines != nil {
		diffArgs = append(diffArgs, fmt.Sprintf("--unified=%d", *params.ContextLines))
	}
	diffArgs = append(diffArgs, params.Ref+"~1", params.Ref)

	patchOut, err := git.Run(ctx, params.RepoPath, diffArgs...)
	if err != nil {
		patchOut = ""
	}

	result := git.ParseShow(metadataOut, numstatOut, patchOut)

	patch, truncated, truncatedAt := git.TruncatePatch(result.Patch, params.MaxPatchLines)
	result.Patch = patch
	result.Truncated = truncated
	result.TruncatedAtLine = truncatedAt

	return command.JSONResult(result), nil
}

func handleGitBlame(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		RepoPath  string `json:"repo_path"`
		Path      string `json:"path"`
		Ref       string `json:"ref"`
		LineRange string `json:"line_range"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	gitArgs := []string{"blame", "--porcelain"}

	if params.LineRange != "" {
		gitArgs = append(gitArgs, fmt.Sprintf("-L%s", params.LineRange))
	}

	if params.Ref != "" {
		gitArgs = append(gitArgs, params.Ref)
	}

	gitArgs = append(gitArgs, "--", params.Path)

	out, err := git.Run(ctx, params.RepoPath, gitArgs...)
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("git blame: %v", err)), nil
	}

	lines := git.ParseBlame(out)

	return command.JSONResult(lines), nil
}
