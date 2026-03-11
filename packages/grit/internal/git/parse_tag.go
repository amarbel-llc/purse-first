package git

import (
	"strings"
)

const TagListFormat = "%(refname:short)\x1f%(objectname:short)\x1f%(objecttype)\x1f%(subject)\x1f%(creatordate:iso-strict)\x1f%(taggername)\x1f%(taggeremail)\x1f%(*objectname:short)\x1e"

func ParseTagList(output string) []TagEntry {
	var tags []TagEntry

	records := strings.Split(strings.TrimSpace(output), branchRecordSep)
	for _, record := range records {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}

		fields := strings.Split(record, fieldSep)
		if len(fields) < 3 {
			continue
		}

		tagType := "lightweight"
		if strings.TrimSpace(fields[2]) == "tag" {
			tagType = "annotated"
		}

		entry := TagEntry{
			Name: strings.TrimSpace(fields[0]),
			Hash: strings.TrimSpace(fields[1]),
			Type: tagType,
		}

		if len(fields) > 3 {
			entry.Subject = strings.TrimSpace(fields[3])
		}

		if len(fields) > 4 {
			entry.TaggerDate = strings.TrimSpace(fields[4])
		}

		if len(fields) > 5 {
			entry.TaggerName = strings.TrimSpace(fields[5])
		}

		if len(fields) > 6 {
			email := strings.TrimSpace(fields[6])
			email = strings.TrimPrefix(email, "<")
			email = strings.TrimSuffix(email, ">")
			entry.TaggerEmail = email
		}

		if len(fields) > 7 {
			entry.TargetHash = strings.TrimSpace(fields[7])
		}

		tags = append(tags, entry)
	}

	if tags == nil {
		tags = []TagEntry{}
	}

	return tags
}

func ParseTagVerify(stdout, stderr string, exitErr error) TagVerifyResult {
	result := TagVerifyResult{
		Valid: exitErr == nil,
	}

	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Good signature from") {
			result.Signer = extractQuoted(line)
		}
	}

	if exitErr != nil {
		for _, line := range strings.Split(stderr, "\n") {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "no signature found") {
				result.Message = "no signature found"
				break
			}
			if strings.Contains(line, "BAD signature") {
				result.Message = line
				break
			}
		}

		if result.Message == "" {
			result.Message = strings.TrimSpace(stderr)
		}
	}

	return result
}

func extractQuoted(s string) string {
	start := strings.IndexByte(s, '"')
	if start < 0 {
		return ""
	}
	end := strings.IndexByte(s[start+1:], '"')
	if end < 0 {
		return ""
	}
	return s[start+1 : start+1+end]
}
