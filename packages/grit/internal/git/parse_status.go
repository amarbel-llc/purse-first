package git

import (
	"strconv"
	"strings"
)

func ParseStatus(output string) StatusResult {
	result := StatusResult{
		Entries: []StatusEntry{},
	}

	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "# ") {
			parseStatusHeader(line, &result.Branch)
			continue
		}

		if entry, ok := parseStatusEntry(line); ok {
			result.Entries = append(result.Entries, entry)
		}
	}

	return result
}

func parseStatusHeader(line string, branch *BranchStatus) {
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 3 {
		return
	}

	key := parts[1]
	value := parts[2]

	switch key {
	case "branch.oid":
		branch.OID = value
	case "branch.head":
		branch.Head = value
	case "branch.upstream":
		branch.Upstream = value
	case "branch.ab":
		abParts := strings.Fields(value)
		for _, p := range abParts {
			if strings.HasPrefix(p, "+") {
				branch.Ahead, _ = strconv.Atoi(p[1:])
			} else if strings.HasPrefix(p, "-") {
				branch.Behind, _ = strconv.Atoi(p[1:])
			}
		}
	}
}

func parseStatusEntry(line string) (StatusEntry, bool) {
	if len(line) < 2 {
		return StatusEntry{}, false
	}

	prefix := string(line[0])

	switch prefix {
	case "1":
		return parseOrdinaryEntry(line)
	case "2":
		return parseRenameEntry(line)
	case "?":
		return parseUntrackedEntry(line)
	case "!":
		return parseIgnoredEntry(line)
	}

	return StatusEntry{}, false
}

func parseOrdinaryEntry(line string) (StatusEntry, bool) {
	fields := strings.Fields(line)
	if len(fields) < 9 {
		return StatusEntry{}, false
	}

	return StatusEntry{
		State: fields[1],
		Path:  fields[8],
	}, true
}

func parseRenameEntry(line string) (StatusEntry, bool) {
	// Porcelain v2 rename format:
	// 2 XY sub mH mI mW hH hI X### path\torigPath
	// We need to skip 9 space-separated fields to reach the tab-separated paths.
	fields := strings.Fields(line)
	if len(fields) < 9 {
		return StatusEntry{}, false
	}

	state := fields[1]

	spaceCount := 0
	pathStart := 0
	for i, ch := range line {
		if ch == ' ' {
			spaceCount++
			if spaceCount == 9 {
				pathStart = i + 1
				break
			}
		}
	}

	if pathStart == 0 {
		return StatusEntry{}, false
	}

	paths := strings.SplitN(line[pathStart:], "\t", 2)
	entry := StatusEntry{
		State: state,
		Path:  paths[0],
	}

	if len(paths) > 1 {
		entry.OrigPath = paths[1]
	}

	return entry, true
}

func parseUntrackedEntry(line string) (StatusEntry, bool) {
	if len(line) < 3 {
		return StatusEntry{}, false
	}

	return StatusEntry{
		State: "?",
		Path:  line[2:],
	}, true
}

func parseIgnoredEntry(line string) (StatusEntry, bool) {
	if len(line) < 3 {
		return StatusEntry{}, false
	}

	return StatusEntry{
		State: "!",
		Path:  line[2:],
	}, true
}

func ParseDiffNumstat(output string) []DiffStat {
	var stats []DiffStat

	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}

		fields := strings.SplitN(line, "\t", 3)
		if len(fields) < 3 {
			continue
		}

		stat := DiffStat{
			Path: fields[2],
		}

		if fields[0] == "-" && fields[1] == "-" {
			stat.Binary = true
		} else {
			stat.Additions, _ = strconv.Atoi(fields[0])
			stat.Deletions, _ = strconv.Atoi(fields[1])
		}

		stats = append(stats, stat)
	}

	if stats == nil {
		stats = []DiffStat{}
	}

	return stats
}
