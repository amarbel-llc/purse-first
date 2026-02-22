package git

import (
	"regexp"
	"strings"
)

var commitRegexp = regexp.MustCompile(`^\[(.+?)\s+([a-f0-9]+)\]\s+(.*)$`)

func ParseCommit(output string) CommitResult {
	firstLine := strings.SplitN(strings.TrimSpace(output), "\n", 2)[0]
	firstLine = strings.TrimSpace(firstLine)

	matches := commitRegexp.FindStringSubmatch(firstLine)
	if matches == nil {
		return CommitResult{
			Status:  "committed",
			Subject: firstLine,
		}
	}

	return CommitResult{
		Status:  "committed",
		Branch:  matches[1],
		Hash:    matches[2],
		Subject: matches[3],
	}
}
