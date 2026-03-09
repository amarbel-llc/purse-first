package validate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/amarbel-llc/spinclass/internal/sweatfile"
	"github.com/amarbel-llc/spinclass/internal/tap"
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
	if strings.HasPrefix(name, "mcp__") {
		return true
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
				Field:    "claude-allow",
				Value:    rule,
			})
			continue
		}
		if !isKnownTool(toolName) {
			issues = append(issues, Issue{
				Message:  fmt.Sprintf("unknown tool name %q", toolName),
				Severity: SeverityWarning,
				Field:    "claude-allow",
				Value:    rule,
			})
		}
	}
	return issues
}

func CheckGitExcludes(sf sweatfile.Sweatfile) []Issue {
	var issues []Issue
	for _, exc := range sf.GitSkipIndex {
		if exc == "" {
			issues = append(issues, Issue{
				Message:  "empty exclude pattern",
				Severity: SeverityError,
				Field:    "git-excludes",
			})
		} else if filepath.IsAbs(exc) {
			issues = append(issues, Issue{
				Message:  fmt.Sprintf("absolute path %q in git-excludes", exc),
				Severity: SeverityError,
				Field:    "git-excludes",
				Value:    exc,
			})
		}
	}
	return issues
}

func CheckMerged(sf sweatfile.Sweatfile) []Issue {
	var issues []Issue

	if dups := findDuplicates(sf.GitSkipIndex); len(dups) > 0 {
		issues = append(issues, Issue{
			Message:  fmt.Sprintf("duplicate git-excludes: %s", strings.Join(dups, ", ")),
			Severity: SeverityWarning,
			Field:    "git-excludes",
		})
	}

	if dups := findDuplicates(sf.ClaudeAllow); len(dups) > 0 {
		issues = append(issues, Issue{
			Message:  fmt.Sprintf("duplicate claude-allow: %s", strings.Join(dups, ", ")),
			Severity: SeverityWarning,
			Field:    "claude-allow",
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
	var sf sweatfile.Sweatfile
	md, err := toml.Decode(string(data), &sf)
	if err != nil {
		return nil
	}

	var issues []Issue
	for _, key := range md.Undecoded() {
		issues = append(issues, Issue{
			Message:  fmt.Sprintf("unknown field %q", key.String()),
			Severity: SeverityError,
			Field:    key.String(),
		})
	}
	return issues
}

func Run(w io.Writer, home, repoDir string) int {
	tw := tap.NewWriter(w)

	result, err := sweatfile.LoadHierarchy(home, repoDir)
	if err != nil {
		tw.NotOk("load hierarchy", map[string]string{
			"severity": SeverityError,
			"message":  err.Error(),
		})
		tw.Plan()
		return 1
	}

	for _, src := range result.Sources {
		if !src.Found {
			tw.Skip(src.Path, "not found")
			continue
		}

		sub := tw.Subtest(src.Path)

		data, readErr := os.ReadFile(src.Path)
		if readErr != nil {
			sub.NotOk("readable", map[string]string{
				"severity": SeverityError,
				"message":  readErr.Error(),
			})
			sub.Plan()
			tw.EndSubtest(src.Path, sub)
			continue
		}

		_, parseErr := sweatfile.Parse(data)
		if parseErr != nil {
			sub.NotOk("valid TOML", map[string]string{
				"severity": SeverityError,
				"message":  parseErr.Error(),
			})
			sub.Plan()
			tw.EndSubtest(src.Path, sub)
			continue
		}
		sub.Ok("valid TOML")

		if issues := CheckUnknownFields(data); len(issues) > 0 {
			for _, iss := range issues {
				sub.NotOk("no unknown fields", map[string]string{
					"severity": iss.Severity,
					"message":  iss.Message,
					"field":    iss.Field,
				})
			}
		} else {
			sub.Ok("no unknown fields")
		}

		if len(src.File.ClaudeAllow) > 0 {
			if issues := CheckClaudeAllow(src.File); len(issues) > 0 {
				for _, iss := range issues {
					if iss.Severity == SeverityError {
						diag := map[string]string{
							"severity": iss.Severity,
							"message":  iss.Message,
						}
						if iss.Value != "" {
							diag["rule"] = iss.Value
						}
						sub.NotOk("claude-allow valid", diag)
					} else {
						sub.Ok(fmt.Sprintf("claude-allow valid # warning: %s", iss.Message))
					}
				}
			} else {
				sub.Ok("claude-allow valid")
			}
		}

		if len(src.File.GitSkipIndex) > 0 {
			if issues := CheckGitExcludes(src.File); len(issues) > 0 {
				for _, iss := range issues {
					diag := map[string]string{
						"severity": iss.Severity,
						"message":  iss.Message,
					}
					if iss.Value != "" {
						diag["value"] = iss.Value
					}
					sub.NotOk("git-excludes valid", diag)
				}
			} else {
				sub.Ok("git-excludes valid")
			}
		}

		sub.Plan()
		tw.EndSubtest(src.Path, sub)
	}

	sub := tw.Subtest("merged result")
	if issues := CheckMerged(result.Merged); len(issues) > 0 {
		for _, iss := range issues {
			if iss.Severity == SeverityError {
				sub.NotOk(iss.Field+" unique", map[string]string{
					"severity": iss.Severity,
					"message":  iss.Message,
				})
			} else {
				sub.Ok(fmt.Sprintf("%s unique # warning: %s", iss.Field, iss.Message))
			}
		}
	} else {
		sub.Ok("no duplicate entries")
	}
	sub.Plan()
	tw.EndSubtest("merged result", sub)

	applySub := tw.Subtest("apply (dry-run)")
	merged := sweatfile.Merge(result.Merged, sweatfile.GetDefault())
	if issues := CheckGitExcludes(sweatfile.Sweatfile{GitSkipIndex: merged.GitSkipIndex}); len(issues) > 0 {
		for _, iss := range issues {
			applySub.NotOk("git excludes structure valid", map[string]string{
				"severity": iss.Severity,
				"message":  iss.Message,
			})
		}
	} else {
		applySub.Ok("git excludes structure valid")
	}

	if issues := CheckClaudeAllow(result.Merged); len(issues) > 0 {
		hasErrors := false
		for _, iss := range issues {
			if iss.Severity == SeverityError {
				hasErrors = true
				applySub.NotOk("claude settings structure valid", map[string]string{
					"severity": iss.Severity,
					"message":  iss.Message,
				})
			}
		}
		if !hasErrors {
			applySub.Ok("claude settings structure valid")
		}
	} else {
		applySub.Ok("claude settings structure valid")
	}
	applySub.Plan()
	tw.EndSubtest("apply (dry-run)", applySub)

	tw.Plan()

	if tw.HasFailures() {
		return 1
	}
	return 0
}
