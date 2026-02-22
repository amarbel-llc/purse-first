package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/amarbel-llc/go-lib-mcp/protocol"
	"github.com/amarbel-llc/go-lib-mcp/server"
	"github.com/friedenberg/get-hubbed/internal/gh"
)

func registerContentTools(r *server.ToolRegistry) {
	r.Register(
		"content_tree",
		"List directory contents of a repository at a given path and ref",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"repo": {
					"type": "string",
					"description": "Repository in OWNER/REPO format"
				},
				"ref": {
					"type": "string",
					"description": "Git ref (branch, tag, or SHA). Defaults to the repo's default branch"
				},
				"path": {
					"type": "string",
					"description": "Directory path within the repo (e.g. 'src/lib'). Defaults to repo root"
				},
				"recursive": {
					"type": "boolean",
					"description": "List tree recursively (all nested files/dirs)"
				},
				"limit": {
					"type": "integer",
					"description": "Maximum number of entries to return"
				},
				"offset": {
					"type": "integer",
					"description": "Number of entries to skip for pagination"
				}
			},
			"required": ["repo"]
		}`),
		handleContentTree,
	)

	r.Register(
		"content_read",
		"Read file contents from a repository at a given path and ref. Limited to files under 1 MB by the GitHub API",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"repo": {
					"type": "string",
					"description": "Repository in OWNER/REPO format"
				},
				"path": {
					"type": "string",
					"description": "File path within the repo (e.g. 'src/main.go')"
				},
				"ref": {
					"type": "string",
					"description": "Git ref (branch, tag, or SHA). Defaults to the repo's default branch"
				},
				"line_offset": {
					"type": "integer",
					"description": "Start reading from this line number (1-based). Defaults to 1"
				},
				"line_limit": {
					"type": "integer",
					"description": "Maximum number of lines to return. Defaults to all lines"
				}
			},
			"required": ["repo", "path"]
		}`),
		handleContentRead,
	)

	r.Register(
		"content_blame",
		"Show line-by-line authorship of a file in a repository",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"repo": {
					"type": "string",
					"description": "Repository in OWNER/REPO format"
				},
				"path": {
					"type": "string",
					"description": "File path within the repo"
				},
				"ref": {
					"type": "string",
					"description": "Git ref (branch, tag, or SHA). Defaults to HEAD"
				},
				"start_line": {
					"type": "integer",
					"description": "Start line of the range to blame (1-based)"
				},
				"end_line": {
					"type": "integer",
					"description": "End line of the range to blame (1-based, inclusive)"
				}
			},
			"required": ["repo", "path"]
		}`),
		handleContentBlame,
	)

	r.Register(
		"content_commits",
		"List commits for a specific file or directory path",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"repo": {
					"type": "string",
					"description": "Repository in OWNER/REPO format"
				},
				"path": {
					"type": "string",
					"description": "File or directory path to get commit history for"
				},
				"ref": {
					"type": "string",
					"description": "Branch or tag name to list commits from. Defaults to the repo's default branch"
				},
				"per_page": {
					"type": "integer",
					"description": "Number of commits per page (max 100, default 30)"
				},
				"page": {
					"type": "integer",
					"description": "Page number for pagination (default 1)"
				}
			},
			"required": ["repo", "path"]
		}`),
		handleContentCommits,
	)

	r.Register(
		"content_compare",
		"Compare two refs (branches, tags, or commits) showing commits and file changes",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"repo": {
					"type": "string",
					"description": "Repository in OWNER/REPO format"
				},
				"base": {
					"type": "string",
					"description": "Base ref (branch, tag, or SHA)"
				},
				"head": {
					"type": "string",
					"description": "Head ref (branch, tag, or SHA)"
				},
				"per_page": {
					"type": "integer",
					"description": "Number of file entries per page (max 100, default 30)"
				},
				"page": {
					"type": "integer",
					"description": "Page number for pagination (default 1)"
				}
			},
			"required": ["repo", "base", "head"]
		}`),
		handleContentCompare,
	)

	r.Register(
		"content_search",
		"Search for code within a repository",
		json.RawMessage(`{
			"type": "object",
			"properties": {
				"repo": {
					"type": "string",
					"description": "Repository in OWNER/REPO format"
				},
				"query": {
					"type": "string",
					"description": "Search query (code to search for)"
				},
				"path": {
					"type": "string",
					"description": "Restrict search to a file path or directory prefix"
				},
				"extension": {
					"type": "string",
					"description": "Restrict search to a file extension (e.g. 'go', 'py')"
				},
				"per_page": {
					"type": "integer",
					"description": "Number of results per page (max 100, default 30)"
				},
				"page": {
					"type": "integer",
					"description": "Page number for pagination (default 1)"
				}
			},
			"required": ["repo", "query"]
		}`),
		handleContentSearch,
	)
}

func handleContentTree(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
	var params struct {
		Repo      string `json:"repo"`
		Ref       string `json:"ref"`
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
		Limit     int    `json:"limit"`
		Offset    int    `json:"offset"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
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
		return protocol.ErrorResult(fmt.Sprintf("gh api git/trees: %v", err)), nil
	}

	if params.Offset > 0 || params.Limit > 0 {
		var entries []json.RawMessage
		if err := json.Unmarshal([]byte(out), &entries); err != nil {
			return &protocol.ToolCallResult{
				Content: []protocol.ContentBlock{
					protocol.TextContent(out),
				},
			}, nil
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

		resultJSON, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return protocol.ErrorResult(fmt.Sprintf("marshaling paginated result: %v", err)), nil
		}

		return &protocol.ToolCallResult{
			Content: []protocol.ContentBlock{
				protocol.TextContent(string(resultJSON)),
			},
		}, nil
	}

	return &protocol.ToolCallResult{
		Content: []protocol.ContentBlock{
			protocol.TextContent(out),
		},
	}, nil
}

func handleContentRead(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
	var params struct {
		Repo       string `json:"repo"`
		Path       string `json:"path"`
		Ref        string `json:"ref"`
		LineOffset int    `json:"line_offset"`
		LineLimit  int    `json:"line_limit"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
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
		return protocol.ErrorResult(fmt.Sprintf("gh api contents: %v", err)), nil
	}

	// GitHub API returns an array for directories, detect this before unmarshaling
	trimmed := strings.TrimSpace(out)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		return &protocol.ToolCallResult{
			Content: []protocol.ContentBlock{
				protocol.TextContent(fmt.Sprintf("Path '%s' is a directory. Use content_tree to list its contents.", params.Path)),
			},
		}, nil
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
		return protocol.ErrorResult(fmt.Sprintf("parsing content response: %v", err)), nil
	}

	if contentResp.Type == "dir" {
		return &protocol.ToolCallResult{
			Content: []protocol.ContentBlock{
				protocol.TextContent(fmt.Sprintf("Path '%s' is a directory. Use content_tree to list its contents.", params.Path)),
			},
		}, nil
	}

	if contentResp.Encoding != "base64" {
		return protocol.ErrorResult(fmt.Sprintf("unexpected encoding: %s", contentResp.Encoding)), nil
	}

	decoded, err := base64.StdEncoding.DecodeString(
		strings.ReplaceAll(contentResp.Content, "\n", ""),
	)
	if err != nil {
		return protocol.ErrorResult(fmt.Sprintf("decoding base64 content: %v", err)), nil
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

	return &protocol.ToolCallResult{
		Content: []protocol.ContentBlock{
			protocol.TextContent(header + "\n" + strings.Join(selectedLines, "\n")),
		},
	}, nil
}

func handleContentBlame(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
	var params struct {
		Repo      string `json:"repo"`
		Path      string `json:"path"`
		Ref       string `json:"ref"`
		StartLine int    `json:"start_line"`
		EndLine   int    `json:"end_line"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
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
		return protocol.ErrorResult(fmt.Sprintf("gh api graphql blame: %v", err)), nil
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
			return &protocol.ToolCallResult{
				Content: []protocol.ContentBlock{
					protocol.TextContent(out),
				},
			}, nil
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
			return protocol.ErrorResult(fmt.Sprintf("marshaling filtered blame: %v", err)), nil
		}

		return &protocol.ToolCallResult{
			Content: []protocol.ContentBlock{
				protocol.TextContent(string(filteredJSON)),
			},
		}, nil
	}

	return &protocol.ToolCallResult{
		Content: []protocol.ContentBlock{
			protocol.TextContent(out),
		},
	}, nil
}

func handleContentCommits(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
	var params struct {
		Repo    string `json:"repo"`
		Path    string `json:"path"`
		Ref     string `json:"ref"`
		PerPage int    `json:"per_page"`
		Page    int    `json:"page"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
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
		return protocol.ErrorResult(fmt.Sprintf("gh api commits: %v", err)), nil
	}

	return &protocol.ToolCallResult{
		Content: []protocol.ContentBlock{
			protocol.TextContent(out),
		},
	}, nil
}

func handleContentCompare(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
	var params struct {
		Repo    string `json:"repo"`
		Base    string `json:"base"`
		Head    string `json:"head"`
		PerPage int    `json:"per_page"`
		Page    int    `json:"page"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
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
		return protocol.ErrorResult(fmt.Sprintf("gh api compare: %v", err)), nil
	}

	return &protocol.ToolCallResult{
		Content: []protocol.ContentBlock{
			protocol.TextContent(out),
		},
	}, nil
}

func handleContentSearch(ctx context.Context, args json.RawMessage) (*protocol.ToolCallResult, error) {
	var params struct {
		Repo      string `json:"repo"`
		Query     string `json:"query"`
		Path      string `json:"path"`
		Extension string `json:"extension"`
		PerPage   int    `json:"per_page"`
		Page      int    `json:"page"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return protocol.ErrorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
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
		return protocol.ErrorResult(fmt.Sprintf("gh api search/code: %v", err)), nil
	}

	return &protocol.ToolCallResult{
		Content: []protocol.ContentBlock{
			protocol.TextContent(out),
		},
	}, nil
}
