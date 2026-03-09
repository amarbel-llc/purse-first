package worktree

import (
	"strings"
	"unicode"
)

// SanitizeBranchName transforms parts into a valid git branch name using
// "snob-case": hyphens within parts become underscores, spaces become hyphens,
// parts are joined with hyphens, and the result is lowercased and stripped of
// git-invalid characters.
func SanitizeBranchName(parts []string) string {
	if len(parts) == 0 {
		return ""
	}

	transformed := make([]string, len(parts))
	for i, part := range parts {
		part = strings.ReplaceAll(part, "-", "_")
		part = strings.ReplaceAll(part, " ", "-")
		transformed[i] = part
	}

	name := strings.Join(transformed, "-")
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "@{", "")
	name = stripGitInvalidChars(name)
	name = collapseConsecutive(name, '.')
	name = collapseConsecutive(name, '-')
	name = collapseConsecutive(name, '_')
	name = stripTrailingLockSuffix(name)
	name = strings.Trim(name, ".-_")

	return name
}

func stripGitInvalidChars(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for _, r := range s {
		if unicode.IsControl(r) {
			continue
		}

		switch r {
		case '~', '^', ':', '\\', '?', '*', '[', ']':
			continue
		}

		b.WriteRune(r)
	}

	return b.String()
}

func collapseConsecutive(s string, ch byte) string {
	double := string([]byte{ch, ch})
	single := string([]byte{ch})

	for strings.Contains(s, double) {
		s = strings.ReplaceAll(s, double, single)
	}

	return s
}

func stripTrailingLockSuffix(s string) string {
	return strings.TrimSuffix(s, ".lock")
}
