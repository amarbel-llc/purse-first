package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/amarbel-llc/purse-first/libs/go-mcp/command"
	"github.com/amarbel-llc/purse-first/libs/go-mcp/protocol"
	"github.com/friedenberg/get-hubbed/internal/gh"
)

func registerContentCommands(app *command.App) {
	app.AddCommand(&command.Command{
		Name:        "content_tree",
		Title:       "List Directory Contents",
		Description: command.Description{Short: "List directory contents of a repository at a given path and ref"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(true),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(true),
			OpenWorldHint:   protocol.BoolPtr(true),
		},
		Params: []command.Param{
			{Name: "repo", Type: command.String, Description: "Repository in OWNER/REPO format", Required: true},
			{Name: "ref", Type: command.String, Description: "Git ref (branch, tag, or SHA). Defaults to the repo's default branch"},
			{Name: "path", Type: command.String, Description: "Directory path within the repo (e.g. 'src/lib'). Defaults to repo root"},
			{Name: "recursive", Type: command.Bool, Description: "List tree recursively (all nested files/dirs)"},
			{Name: "limit", Type: command.Int, Description: "Maximum number of entries to return"},
			{Name: "offset", Type: command.Int, Description: "Number of entries to skip for pagination"},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"gh api repos"}, UseWhen: "listing repository directory contents"},
		},
		Run: handleContentTree,
	})

	app.AddCommand(&command.Command{
		Name:        "content_read",
		Title:       "Read File Contents",
		Description: command.Description{Short: "Read file contents from a repository at a given path and ref. Limited to files under 1 MB by the GitHub API"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(true),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(true),
			OpenWorldHint:   protocol.BoolPtr(true),
		},
		Params: []command.Param{
			{Name: "repo", Type: command.String, Description: "Repository in OWNER/REPO format", Required: true},
			{Name: "path", Type: command.String, Description: "File path within the repo (e.g. 'src/main.go')", Required: true},
			{Name: "ref", Type: command.String, Description: "Git ref (branch, tag, or SHA). Defaults to the repo's default branch. Verify exact tag names with api_get(endpoint: \"repos/OWNER/REPO/tags\") if unsure"},
			{Name: "line_offset", Type: command.Int, Description: "Start reading from this line number (1-based). Defaults to 1"},
			{Name: "line_limit", Type: command.Int, Description: "Maximum number of lines to return. Defaults to all lines"},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"gh api repos"}, UseWhen: "reading file contents from a repository"},
		},
		Run: handleContentRead,
	})

	app.AddCommand(&command.Command{
		Name:        "content_blame",
		Title:       "Show File Blame",
		Description: command.Description{Short: "Show line-by-line authorship of a file in a repository"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(true),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(true),
			OpenWorldHint:   protocol.BoolPtr(true),
		},
		Params: []command.Param{
			{Name: "repo", Type: command.String, Description: "Repository in OWNER/REPO format", Required: true},
			{Name: "path", Type: command.String, Description: "File path within the repo", Required: true},
			{Name: "ref", Type: command.String, Description: "Git ref (branch, tag, or SHA). Defaults to HEAD"},
			{Name: "start_line", Type: command.Int, Description: "Start line of the range to blame (1-based)"},
			{Name: "end_line", Type: command.Int, Description: "End line of the range to blame (1-based, inclusive)"},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"gh api graphql"}, UseWhen: "viewing file blame information"},
		},
		Run: handleContentBlame,
	})

	app.AddCommand(&command.Command{
		Name:        "content_commits",
		Title:       "List File Commits",
		Description: command.Description{Short: "List commits for a specific file or directory path"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(true),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(true),
			OpenWorldHint:   protocol.BoolPtr(true),
		},
		Params: []command.Param{
			{Name: "repo", Type: command.String, Description: "Repository in OWNER/REPO format", Required: true},
			{Name: "path", Type: command.String, Description: "File or directory path to get commit history for", Required: true},
			{Name: "ref", Type: command.String, Description: "Branch or tag name to list commits from. Defaults to the repo's default branch"},
			{Name: "per_page", Type: command.Int, Description: "Number of commits per page (max 100, default 30)"},
			{Name: "page", Type: command.Int, Description: "Page number for pagination (default 1)"},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"gh api repos"}, UseWhen: "listing commits for a file or directory"},
		},
		Run: handleContentCommits,
	})

	app.AddCommand(&command.Command{
		Name:        "content_compare",
		Title:       "Compare Refs",
		Description: command.Description{Short: "Compare two refs (branches, tags, or commits) showing commits and file changes"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(true),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(true),
			OpenWorldHint:   protocol.BoolPtr(true),
		},
		Params: []command.Param{
			{Name: "repo", Type: command.String, Description: "Repository in OWNER/REPO format", Required: true},
			{Name: "base", Type: command.String, Description: "Base ref (branch, tag, or SHA)", Required: true},
			{Name: "head", Type: command.String, Description: "Head ref (branch, tag, or SHA)", Required: true},
			{Name: "per_page", Type: command.Int, Description: "Number of file entries per page (max 100, default 30)"},
			{Name: "page", Type: command.Int, Description: "Page number for pagination (default 1)"},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"gh api repos"}, UseWhen: "comparing two refs in a repository"},
		},
		Run: handleContentCompare,
	})

	app.AddCommand(&command.Command{
		Name:        "content_search",
		Title:       "Search Code",
		Description: command.Description{Short: "Search for code within a repository"},
		Annotations: &protocol.ToolAnnotations{
			ReadOnlyHint:    protocol.BoolPtr(true),
			DestructiveHint: protocol.BoolPtr(false),
			IdempotentHint:  protocol.BoolPtr(true),
			OpenWorldHint:   protocol.BoolPtr(true),
		},
		Params: []command.Param{
			{Name: "repo", Type: command.String, Description: "Repository in OWNER/REPO format", Required: true},
			{Name: "query", Type: command.String, Description: "Search query (code to search for)", Required: true},
			{Name: "path", Type: command.String, Description: "Restrict search to a file path or directory prefix"},
			{Name: "extension", Type: command.String, Description: "Restrict search to a file extension (e.g. 'go', 'py')"},
			{Name: "per_page", Type: command.Int, Description: "Number of results per page (max 100, default 30)"},
			{Name: "page", Type: command.Int, Description: "Page number for pagination (default 1)"},
		},
		MapsTools: []command.ToolMapping{
			{Replaces: "Bash", CommandPrefixes: []string{"gh api search/code"}, UseWhen: "searching for code in a repository"},
		},
		Run: handleContentSearch,
	})
}

func handleContentTree(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		Repo      string `json:"repo"`
		Ref       string `json:"ref"`
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
		Limit     int    `json:"limit"`
		Offset    int    `json:"offset"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	ref := params.Ref
	if ref == "" {
		ref = "HEAD"
	}

	treeSha := ref
	if params.Path != "" {
		treeSha = ref + ":" + params.Path
	}

	ghArgs := []string{
		"api",
		fmt.Sprintf("repos/%s/git/trees/%s", params.Repo, treeSha),
		"--method", "GET",
	}

	if params.Recursive {
		ghArgs = append(ghArgs, "-f", "recursive=1")
	}

	ghArgs = append(ghArgs, "--jq", ".tree")

	out, err := gh.Run(ctx, ghArgs...)
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("gh api git/trees: %v", err)), nil
	}

	if params.Offset > 0 || params.Limit > 0 {
		var entries []json.RawMessage
		if err := json.Unmarshal([]byte(out), &entries); err != nil {
			return command.TextResult(out), nil
		}

		total := len(entries)
		start := params.Offset
		if start > total {
			start = total
		}

		end := total
		if params.Limit > 0 && start+params.Limit < end {
			end = start + params.Limit
		}

		paginated := entries[start:end]

		result := struct {
			Entries []json.RawMessage `json:"entries"`
			Total   int               `json:"total"`
			Offset  int               `json:"offset"`
			Count   int               `json:"count"`
		}{
			Entries: paginated,
			Total:   total,
			Offset:  start,
			Count:   len(paginated),
		}

		return command.JSONResult(result), nil
	}

	return command.TextResult(out), nil
}

func handleContentRead(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		Repo       string `json:"repo"`
		Path       string `json:"path"`
		Ref        string `json:"ref"`
		LineOffset int    `json:"line_offset"`
		LineLimit  int    `json:"line_limit"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	ghArgs := []string{
		"api",
		fmt.Sprintf("repos/%s/contents/%s", params.Repo, params.Path),
		"--method", "GET",
	}

	if params.Ref != "" {
		ghArgs = append(ghArgs, "-f", fmt.Sprintf("ref=%s", params.Ref))
	}

	out, err := gh.Run(ctx, ghArgs...)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "404") || strings.Contains(errMsg, "Not Found") {
			hint := fmt.Sprintf(
				"404 Not Found: %s at path %q",
				params.Repo, params.Path,
			)
			if params.Ref != "" {
				hint += fmt.Sprintf(
					" (ref %q). The ref may not exist. "+
						"To verify tag names, use api_get(endpoint: \"repos/%s/tags\") "+
						"or api_get(endpoint: \"repos/%s/branches\")",
					params.Ref, params.Repo, params.Repo,
				)
			}
			return command.TextErrorResult(hint), nil
		}
		return command.TextErrorResult(fmt.Sprintf("gh api contents: %v", err)), nil
	}

	// GitHub API returns an array for directories, detect this before unmarshaling
	trimmed := strings.TrimSpace(out)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		return command.TextResult(fmt.Sprintf("Path '%s' is a directory. Use content_tree to list its contents.", params.Path)), nil
	}

	var contentResp struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
		Size     int    `json:"size"`
		Name     string `json:"name"`
		Path     string `json:"path"`
		Type     string `json:"type"`
		SHA      string `json:"sha"`
	}

	if err := json.Unmarshal([]byte(out), &contentResp); err != nil {
		return command.TextErrorResult(fmt.Sprintf("parsing content response: %v", err)), nil
	}

	if contentResp.Type == "dir" {
		return command.TextResult(fmt.Sprintf("Path '%s' is a directory. Use content_tree to list its contents.", params.Path)), nil
	}

	if contentResp.Encoding != "base64" {
		return command.TextErrorResult(fmt.Sprintf("unexpected encoding: %s", contentResp.Encoding)), nil
	}

	decoded, err := base64.StdEncoding.DecodeString(
		strings.ReplaceAll(contentResp.Content, "\n", ""),
	)
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("decoding base64 content: %v", err)), nil
	}

	text := string(decoded)
	lines := strings.Split(text, "\n")
	totalLines := len(lines)

	startLine := 1
	if params.LineOffset > 0 {
		startLine = params.LineOffset
	}

	if startLine > totalLines {
		startLine = totalLines
	}

	endLine := totalLines
	if params.LineLimit > 0 && startLine-1+params.LineLimit < endLine {
		endLine = startLine - 1 + params.LineLimit
	}

	selectedLines := lines[startLine-1 : endLine]

	sha := contentResp.SHA
	if len(sha) > 8 {
		sha = sha[:8]
	}

	header := fmt.Sprintf("File: %s (SHA: %s, %d bytes, %d total lines)\n",
		contentResp.Path, sha, contentResp.Size, totalLines)

	if params.LineOffset > 0 || params.LineLimit > 0 {
		header += fmt.Sprintf("Showing lines %d-%d of %d\n", startLine, endLine, totalLines)
	}

	return command.TextResult(header + "\n" + strings.Join(selectedLines, "\n")), nil
}

func handleContentBlame(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		Repo      string `json:"repo"`
		Path      string `json:"path"`
		Ref       string `json:"ref"`
		StartLine int    `json:"start_line"`
		EndLine   int    `json:"end_line"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	parts := strings.SplitN(params.Repo, "/", 2)
	owner, name := parts[0], parts[1]

	ref := params.Ref
	if ref == "" {
		ref = "HEAD"
	}

	query := fmt.Sprintf(`query {
		repository(owner: %q, name: %q) {
			object(expression: %q) {
				... on Commit {
					blame(path: %q) {
						ranges {
							startingLine
							endingLine
							commit {
								oid
								message
								author {
									name
									date
								}
							}
						}
					}
				}
			}
		}
	}`, owner, name, ref, params.Path)

	ghArgs := []string{"api", "graphql", "-f", fmt.Sprintf("query=%s", query)}

	out, err := gh.Run(ctx, ghArgs...)
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("gh api graphql blame: %v", err)), nil
	}

	if params.StartLine > 0 || params.EndLine > 0 {
		var result struct {
			Data struct {
				Repository struct {
					Object struct {
						Blame struct {
							Ranges []json.RawMessage `json:"ranges"`
						} `json:"blame"`
					} `json:"object"`
				} `json:"repository"`
			} `json:"data"`
		}

		if err := json.Unmarshal([]byte(out), &result); err != nil {
			return command.TextResult(out), nil
		}

		var filtered []json.RawMessage
		for _, r := range result.Data.Repository.Object.Blame.Ranges {
			var rangeInfo struct {
				StartingLine int `json:"startingLine"`
				EndingLine   int `json:"endingLine"`
			}

			if err := json.Unmarshal(r, &rangeInfo); err != nil {
				continue
			}

			startFilter := params.StartLine
			if startFilter == 0 {
				startFilter = 1
			}

			endFilter := params.EndLine
			if endFilter == 0 {
				endFilter = rangeInfo.EndingLine
			}

			if rangeInfo.EndingLine >= startFilter && rangeInfo.StartingLine <= endFilter {
				filtered = append(filtered, r)
			}
		}

		filteredJSON, err := json.MarshalIndent(filtered, "", "  ")
		if err != nil {
			return command.TextErrorResult(fmt.Sprintf("marshaling filtered blame: %v", err)), nil
		}

		return command.TextResult(string(filteredJSON)), nil
	}

	return command.TextResult(out), nil
}

func handleContentCommits(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		Repo    string `json:"repo"`
		Path    string `json:"path"`
		Ref     string `json:"ref"`
		PerPage int    `json:"per_page"`
		Page    int    `json:"page"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	endpoint := fmt.Sprintf("repos/%s/commits", params.Repo)

	ghArgs := []string{"api", endpoint, "--method", "GET", "-f", fmt.Sprintf("path=%s", params.Path)}

	if params.Ref != "" {
		ghArgs = append(ghArgs, "-f", fmt.Sprintf("sha=%s", params.Ref))
	}

	if params.PerPage > 0 {
		ghArgs = append(ghArgs, "-f", fmt.Sprintf("per_page=%d", params.PerPage))
	}

	if params.Page > 0 {
		ghArgs = append(ghArgs, "-f", fmt.Sprintf("page=%d", params.Page))
	}

	ghArgs = append(ghArgs, "--jq",
		`[.[] | {sha: .sha, message: .commit.message, author: .commit.author.name, date: .commit.author.date, url: .html_url}]`,
	)

	out, err := gh.Run(ctx, ghArgs...)
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("gh api commits: %v", err)), nil
	}

	return command.TextResult(out), nil
}

func handleContentCompare(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		Repo    string `json:"repo"`
		Base    string `json:"base"`
		Head    string `json:"head"`
		PerPage int    `json:"per_page"`
		Page    int    `json:"page"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	endpoint := fmt.Sprintf("repos/%s/compare/%s...%s", params.Repo, params.Base, params.Head)

	ghArgs := []string{"api", endpoint, "--method", "GET"}

	if params.PerPage > 0 {
		ghArgs = append(ghArgs, "-f", fmt.Sprintf("per_page=%d", params.PerPage))
	}

	if params.Page > 0 {
		ghArgs = append(ghArgs, "-f", fmt.Sprintf("page=%d", params.Page))
	}

	ghArgs = append(ghArgs, "--jq",
		`{status, ahead_by, behind_by, total_commits, commits: [.commits[] | {sha: .sha[:8], message: .commit.message, author: .commit.author.name, date: .commit.author.date}], files: [.files[] | {filename, status, additions, deletions, changes}]}`,
	)

	out, err := gh.Run(ctx, ghArgs...)
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("gh api compare: %v", err)), nil
	}

	return command.TextResult(out), nil
}

func handleContentSearch(ctx context.Context, args json.RawMessage, _ command.Prompter) (*command.Result, error) {
	var params struct {
		Repo      string `json:"repo"`
		Query     string `json:"query"`
		Path      string `json:"path"`
		Extension string `json:"extension"`
		PerPage   int    `json:"per_page"`
		Page      int    `json:"page"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return command.TextErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	q := fmt.Sprintf("%s repo:%s", params.Query, params.Repo)

	if params.Path != "" {
		q += fmt.Sprintf(" path:%s", params.Path)
	}

	if params.Extension != "" {
		q += fmt.Sprintf(" extension:%s", params.Extension)
	}

	ghArgs := []string{
		"api", "search/code",
		"--method", "GET",
		"-H", "Accept: application/vnd.github.text-match+json",
		"-f", fmt.Sprintf("q=%s", q),
	}

	if params.PerPage > 0 {
		ghArgs = append(ghArgs, "-f", fmt.Sprintf("per_page=%d", params.PerPage))
	}

	if params.Page > 0 {
		ghArgs = append(ghArgs, "-f", fmt.Sprintf("page=%d", params.Page))
	}

	ghArgs = append(ghArgs, "--jq",
		`{total_count, items: [.items[] | {name, path, sha, url: .html_url, score, text_matches: [.text_matches[]? | {fragment, matches: .matches}]}]}`,
	)

	out, err := gh.Run(ctx, ghArgs...)
	if err != nil {
		return command.TextErrorResult(fmt.Sprintf("gh api search/code: %v", err)), nil
	}

	return command.TextResult(out), nil
}
