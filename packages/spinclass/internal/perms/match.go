package perms

import (
	"strings"
)

// BuildPermissionString constructs the permission string Claude Code would use
// for a tool invocation. For Bash: Bash(<command>). For Read/Edit/Write with
// file_path: ToolName(/path). For WebFetch with url: WebFetch(<url>).
// Otherwise just the tool name.
func BuildPermissionString(toolName string, toolInput map[string]any) string {
	switch toolName {
	case "Bash":
		if cmd, ok := stringField(toolInput, "command"); ok {
			return toolName + "(" + cmd + ")"
		}
	case "Read", "Edit", "Write":
		if fp, ok := stringField(toolInput, "file_path"); ok {
			return toolName + "(" + fp + ")"
		}
	case "WebFetch":
		if u, ok := stringField(toolInput, "url"); ok {
			return toolName + "(" + u + ")"
		}
	}

	return toolName
}

// MatchesAnyRule checks if a tool invocation matches any rule in the list.
func MatchesAnyRule(rules []string, toolName string, toolInput map[string]any) bool {
	_, ok := MatchingRule(rules, toolName, toolInput)
	return ok
}

// MatchingRule returns the first rule that matches the tool invocation, or
// empty string and false if none match.
func MatchingRule(rules []string, toolName string, toolInput map[string]any) (string, bool) {
	for _, rule := range rules {
		if matchRule(rule, toolName, toolInput) {
			return rule, true
		}
	}

	return "", false
}

// matchRule checks if a single rule matches the tool invocation.
func matchRule(rule string, toolName string, toolInput map[string]any) bool {
	ruleTool, rulePattern := parseRule(rule)

	if ruleTool != toolName {
		return false
	}

	if rulePattern == "" {
		return true
	}

	permStr := BuildPermissionString(toolName, toolInput)
	_, invocationArg := parseRule(permStr)

	return matchPattern(rulePattern, invocationArg)
}

// parseRule splits a rule like "Bash(git *)" into tool name "Bash" and
// pattern "git *". For plain rules like "Read", returns ("Read", "").
func parseRule(rule string) (string, string) {
	parenIdx := strings.Index(rule, "(")
	if parenIdx < 0 {
		return rule, ""
	}

	toolName := rule[:parenIdx]
	inner := rule[parenIdx+1:]
	inner = strings.TrimSuffix(inner, ")")

	return toolName, inner
}

// matchPattern matches a command against a rule pattern. Supports three forms:
//   - "git status" -- exact match
//   - "git *" -- trailing space-star: command must start with "git "
//   - "go test:*" -- colon-star: command must start with "go test" (the prefix
//     before ":*"), matching "go test", "go test ./...", etc.
func matchPattern(pattern string, command string) bool {
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(command, prefix)
	}

	if strings.HasSuffix(pattern, ":*") {
		prefix := strings.TrimSuffix(pattern, ":*")
		return command == prefix || strings.HasPrefix(command, prefix+" ")
	}

	if strings.HasSuffix(pattern, " *") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(command, prefix)
	}

	return pattern == command
}

func stringField(m map[string]any, key string) (string, bool) {
	if m == nil {
		return "", false
	}

	v, ok := m[key]
	if !ok {
		return "", false
	}

	s, ok := v.(string)
	return s, ok
}
