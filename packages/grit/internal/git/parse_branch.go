package git

import (
	"strings"
)

const branchRecordSep = "\x1e"
const fieldSep = "\x1f"

func ParseBranchList(output string) []BranchEntry {
	var branches []BranchEntry

	records := strings.Split(strings.TrimSpace(output), branchRecordSep)
	for _, record := range records {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}

		fields := strings.Split(record, fieldSep)
		if len(fields) < 4 {
			continue
		}

		entry := BranchEntry{
			IsCurrent: strings.TrimSpace(fields[0]) == "*",
			Name:      strings.TrimSpace(fields[1]),
			Hash:      strings.TrimSpace(fields[2]),
			Subject:   strings.TrimSpace(fields[3]),
		}

		if len(fields) > 4 {
			entry.Upstream = strings.TrimSpace(fields[4])
		}

		if len(fields) > 5 {
			entry.Track = strings.TrimSpace(fields[5])
		}

		branches = append(branches, entry)
	}

	if branches == nil {
		branches = []BranchEntry{}
	}

	return branches
}
