package perms

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// RunCheck reads a PermissionRequest hook payload from r, checks if the tool
// invocation matches any curated tier rule, and writes an allow decision to w
// when matched. When no rule matches, nothing is written to w.
func RunCheck(r io.Reader, w io.Writer, tiersDir string) error {
	var input struct {
		ToolName  string         `json:"tool_name"`
		ToolInput map[string]any `json:"tool_input"`
		CWD       string         `json:"cwd"`
	}

	if err := json.NewDecoder(r).Decode(&input); err != nil {
		return fmt.Errorf("decoding hook input: %w", err)
	}

	repo := repoFromCWD(input.CWD)

	globalPath := filepath.Join(tiersDir, "global.json")
	globalTier, err := LoadTierFile(globalPath)
	if err != nil {
		return fmt.Errorf("loading global tier: %w", err)
	}

	var repoTier Tier
	if repo != "" {
		repoPath := filepath.Join(tiersDir, "repos", repo+".json")
		repoTier, err = LoadTierFile(repoPath)
		if err != nil {
			return fmt.Errorf("loading repo tier %s: %w", repo, err)
		}
	}

	tierName := ""

	if repo != "" {
		if _, ok := MatchingRule(repoTier.Allow, input.ToolName, input.ToolInput); ok {
			tierName = repo
		}
	}

	if tierName == "" {
		if _, ok := MatchingRule(globalTier.Allow, input.ToolName, input.ToolInput); ok {
			tierName = "global"
		}
	}

	if tierName == "" {
		return nil
	}

	permStr := BuildPermissionString(input.ToolName, input.ToolInput)
	sysMsg := fmt.Sprintf("[spinclass] auto-approved: %s (%s tier)", permStr, tierName)

	output := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName": "PermissionRequest",
			"decision":      map[string]any{"behavior": "allow"},
		},
		"systemMessage": sysMsg,
	}

	return json.NewEncoder(w).Encode(output)
}

// repoFromCWD extracts the repository name from a working directory path by
// matching the convention-based patterns: .../worktrees/<repo>/... or
// .../repos/<repo>/...
func repoFromCWD(cwd string) string {
	parts := strings.Split(filepath.ToSlash(cwd), "/")

	for i, part := range parts {
		if (part == "worktrees" || part == "repos") && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	return ""
}
