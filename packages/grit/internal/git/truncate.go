package git

import "strings"

// TruncatePatch truncates a patch string to maxLines lines.
// Returns the truncated string, whether truncation occurred, and the line
// number at which truncation happened. If maxLines is 0, no truncation is
// performed.
func TruncatePatch(patch string, maxLines int) (string, bool, int) {
	if maxLines <= 0 || patch == "" {
		return patch, false, 0
	}

	lines := strings.SplitN(patch, "\n", maxLines+1)
	if len(lines) <= maxLines {
		return patch, false, 0
	}

	return strings.Join(lines[:maxLines], "\n"), true, maxLines
}
