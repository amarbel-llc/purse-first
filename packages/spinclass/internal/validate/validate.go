package validate

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/amarbel-llc/spinclass/internal/sweatfile"
)

const (
	SeverityError   = "error"
	SeverityWarning = "warning"
)

type Issue struct {
	Message  string
	Severity string
	Field    string
	Value    string
}

var KnownTools = []string{
	"Bash", "Read", "Write", "Edit", "Glob", "Grep",
	"WebFetch", "WebSearch", "NotebookEdit", "Task", "Skill", "LSP",
}

func isKnownTool(name string) bool {
	for _, t := range KnownTools {
		if t == name {
			return true
		}
	}
	return false
}

func parseRuleSyntax(rule string) (string, error) {
	if rule == "" {
		return "", fmt.Errorf("empty rule")
	}

	parenIdx := strings.Index(rule, "(")
	if parenIdx < 0 {
		return rule, nil
	}

	if !strings.HasSuffix(rule, ")") {
		return "", fmt.Errorf("unmatched parenthesis in rule %q", rule)
	}

	toolName := rule[:parenIdx]
	if toolName == "" {
		return "", fmt.Errorf("empty tool name in rule %q", rule)
	}

	return toolName, nil
}

func CheckClaudeAllow(sf sweatfile.Sweatfile) []Issue {
	var issues []Issue
	for _, rule := range sf.ClaudeAllow {
		toolName, err := parseRuleSyntax(rule)
		if err != nil {
			issues = append(issues, Issue{
				Message:  err.Error(),
				Severity: SeverityError,
				Field:    "claude_allow",
				Value:    rule,
			})
			continue
		}
		if !isKnownTool(toolName) {
			issues = append(issues, Issue{
				Message:  fmt.Sprintf("unknown tool name %q", toolName),
				Severity: SeverityWarning,
				Field:    "claude_allow",
				Value:    rule,
			})
		}
	}
	return issues
}

func CheckGitExcludes(sf sweatfile.Sweatfile) []Issue {
	var issues []Issue
	for _, exc := range sf.GitExcludes {
		if exc == "" {
			issues = append(issues, Issue{
				Message:  "empty exclude pattern",
				Severity: SeverityError,
				Field:    "git_excludes",
			})
		} else if filepath.IsAbs(exc) {
			issues = append(issues, Issue{
				Message:  fmt.Sprintf("absolute path %q in git_excludes", exc),
				Severity: SeverityError,
				Field:    "git_excludes",
				Value:    exc,
			})
		}
	}
	return issues
}

func CheckMerged(sf sweatfile.Sweatfile) []Issue {
	var issues []Issue

	if dups := findDuplicates(sf.GitExcludes); len(dups) > 0 {
		issues = append(issues, Issue{
			Message:  fmt.Sprintf("duplicate git_excludes: %s", strings.Join(dups, ", ")),
			Severity: SeverityWarning,
			Field:    "git_excludes",
		})
	}

	if dups := findDuplicates(sf.ClaudeAllow); len(dups) > 0 {
		issues = append(issues, Issue{
			Message:  fmt.Sprintf("duplicate claude_allow: %s", strings.Join(dups, ", ")),
			Severity: SeverityWarning,
			Field:    "claude_allow",
		})
	}

	return issues
}

func findDuplicates(items []string) []string {
	seen := make(map[string]bool)
	var dups []string
	for _, item := range items {
		if seen[item] {
			dups = append(dups, item)
		}
		seen[item] = true
	}
	return dups
}

func CheckUnknownFields(data []byte) []Issue {
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil
	}

	known := map[string]bool{
		"git_excludes": true,
		"claude_allow": true,
	}

	var issues []Issue
	for key := range raw {
		if !known[key] {
			issues = append(issues, Issue{
				Message:  fmt.Sprintf("unknown field %q", key),
				Severity: SeverityError,
				Field:    key,
			})
		}
	}
	return issues
}
