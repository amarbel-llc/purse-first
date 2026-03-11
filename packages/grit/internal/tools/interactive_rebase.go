package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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

	app.AddCommand(&command.Command{
		Name:        "interactive_rebase_execute",
		Title:       "Execute Interactive Rebase",
		Description: command.Description{Short: "Execute an interactive rebase with a structured todo list (blocked on main/master for safety)"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(false),
			DestructiveHint: protocol.BoolPtr(true),
			IdempotentHint:  protocol.BoolPtr(false),
			OpenWorldHint:   protocol.BoolPtr(false),
		},
		Params: []command.Param{
			{Name: "repo_path", Type: command.String, Description: "Path to the git repository", Required: true},
			{Name: "upstream", Type: command.String, Description: "Ref to rebase onto", Required: true},
			{
				Name: "todo", Type: command.Array,
				Description: "Ordered list of {action, hash, message?} objects. Actions: pick, reword, squash, fixup, drop",
				Required: true,
				Items: []command.Param{
					{Name: "action", Type: command.String, Description: "Rebase action: pick, reword, squash, fixup, drop", Required: true},
					{Name: "hash", Type: command.String, Description: "Commit hash", Required: true},
					{Name: "message", Type: command.String, Description: "New commit message (required for reword)"},
				},
			},
			{Name: "autostash", Type: command.Bool, Description: "Automatically stash/unstash uncommitted changes"},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"git rebase -i", "git rebase --interactive"}, UseWhen: "performing an interactive rebase"},
		},
		Run: handleInteractiveRebaseExecute,
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

var validActions = map[string]bool{
	"pick": true, "reword": true, "squash": true, "fixup": true, "drop": true,
}

func handleInteractiveRebaseExecute(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		RepoPath  string          `json:"repo_path"`
		Upstream  string          `json:"upstream"`
		Todo      []git.TodoEntry `json:"todo"`
		Autostash bool            `json:"autostash"`
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

	// Check for existing rebase state
	state, err := git.DetectInProgressState(ctx, params.RepoPath)
	if err == nil && state != nil && state.Operation == "rebase" {
		return command.TextErrorResult("a rebase operation is already in progress; use rebase tool with continue, abort, or skip"), nil
	}

	// Validate todo entries
	if len(params.Todo) == 0 {
		return command.TextErrorResult("todo list must not be empty"), nil
	}

	// Collect reword messages keyed by index
	rewordMessages := make(map[int]string)

	for i, entry := range params.Todo {
		if !validActions[entry.Action] {
			return command.TextErrorResult(fmt.Sprintf("invalid action %q at index %d; must be one of: pick, reword, squash, fixup, drop", entry.Action, i)), nil
		}

		if entry.Hash == "" {
			return command.TextErrorResult(fmt.Sprintf("missing hash at index %d", i)), nil
		}

		if (entry.Action == "squash" || entry.Action == "fixup") && i == 0 {
			return command.TextErrorResult(fmt.Sprintf("%s cannot be the first action", entry.Action)), nil
		}

		if entry.Action == "reword" {
			if entry.Message == "" {
				return command.TextErrorResult(fmt.Sprintf("reword at index %d requires a message", i)), nil
			}
			rewordMessages[i] = entry.Message
		}

		// Validate hash exists
		_, err := git.Run(ctx, params.RepoPath, "rev-parse", "--verify", entry.Hash)
		if err != nil {
			return command.TextErrorResult(fmt.Sprintf("invalid commit hash %q at index %d", entry.Hash, i)), nil
		}
	}

	// Build the todo file content
	var todoContent strings.Builder
	for _, entry := range params.Todo {
		shortOut, err := git.Run(ctx, params.RepoPath, "rev-parse", "--short", entry.Hash)
		if err != nil {
			return command.TextErrorResult(fmt.Sprintf("failed to resolve hash %q: %v", entry.Hash, err)), nil
		}
		shortHash := strings.TrimSpace(shortOut)
		fmt.Fprintf(&todoContent, "%s %s\n", entry.Action, shortHash)
	}

	// Write the sequence editor script
	seqScript, err := os.CreateTemp("", "grit-rebase-seq-*.sh")
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("failed to create temp script: %v", err)), nil
	}
	defer os.Remove(seqScript.Name())

	todoStr := todoContent.String()
	scriptContent := fmt.Sprintf("#!/bin/sh\ncat > \"$1\" << 'GRIT_EOF'\n%sGRIT_EOF\n", todoStr)
	if _, err := seqScript.WriteString(scriptContent); err != nil {
		return command.TextErrorResult(fmt.Sprintf("failed to write temp script: %v", err)), nil
	}
	seqScript.Close()
	os.Chmod(seqScript.Name(), 0o755)

	// Build env vars
	extraEnv := []string{
		fmt.Sprintf("GIT_SEQUENCE_EDITOR=%s", seqScript.Name()),
	}

	// Handle reword messages via GIT_EDITOR
	if len(rewordMessages) > 0 {
		editorScript, err := os.CreateTemp("", "grit-rebase-editor-*.sh")
		if err != nil {
			return command.TextErrorResult(fmt.Sprintf("failed to create editor script: %v", err)), nil
		}
		defer os.Remove(editorScript.Name())

		counterFile := editorScript.Name() + ".counter"
		defer os.Remove(counterFile)

		var editorContent strings.Builder
		editorContent.WriteString("#!/bin/sh\n")
		editorContent.WriteString(fmt.Sprintf("COUNTER_FILE='%s'\n", counterFile))
		editorContent.WriteString("if [ ! -f \"$COUNTER_FILE\" ]; then echo 0 > \"$COUNTER_FILE\"; fi\n")
		editorContent.WriteString("COUNT=$(cat \"$COUNTER_FILE\")\n")
		editorContent.WriteString("NEXT=$((COUNT + 1))\n")
		editorContent.WriteString("echo $NEXT > \"$COUNTER_FILE\"\n")

		rewordIndex := 0
		editorContent.WriteString("case $COUNT in\n")
		for i, entry := range params.Todo {
			if entry.Action == "reword" {
				msg := rewordMessages[i]
				escaped := strings.ReplaceAll(msg, "'", "'\\''")
				editorContent.WriteString(fmt.Sprintf("  %d) printf '%%s' '%s' > \"$1\" ;;\n", rewordIndex, escaped))
				rewordIndex++
			}
		}
		editorContent.WriteString("esac\n")

		if _, err := editorScript.WriteString(editorContent.String()); err != nil {
			return command.TextErrorResult(fmt.Sprintf("failed to write editor script: %v", err)), nil
		}
		editorScript.Close()
		os.Chmod(editorScript.Name(), 0o755)

		extraEnv = append(extraEnv, fmt.Sprintf("GIT_EDITOR=%s", editorScript.Name()))
	}

	// Build git args
	gitArgs := []string{"rebase", "-i"}
	if params.Autostash {
		gitArgs = append(gitArgs, "--autostash")
	}
	gitArgs = append(gitArgs, params.Upstream)

	// Execute the rebase
	out, err := git.RunWithEnv(ctx, params.RepoPath, extraEnv, gitArgs...)
	if err != nil {
		if strings.Contains(err.Error(), "CONFLICT") || strings.Contains(err.Error(), "could not apply") {
			conflicts := extractConflictFiles(ctx, params.RepoPath)
			return command.JSONResult(git.RebaseResult{
				Status:    "conflict",
				Branch:    branch,
				Upstream:  params.Upstream,
				Conflicts: conflicts,
			}), nil
		}
		return command.TextErrorResult(fmt.Sprintf("git rebase -i: %v", err)), nil
	}

	result := git.RebaseResult{
		Status:   "completed",
		Branch:   branch,
		Upstream: params.Upstream,
		Summary:  strings.TrimSpace(out),
	}

	if strings.Contains(out, "is up to date") {
		result.Status = "up_to_date"
		result.Summary = ""
	}

	return command.JSONResult(result), nil
}
