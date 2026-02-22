package git

import (
	"strconv"
	"strings"
)

const logRecordSep = "\x1e"
const logFieldSep = "\x1f"

const LogFormat = "%H" + logFieldSep + "%an" + logFieldSep + "%ae" + logFieldSep + "%aI" + logFieldSep + "%s" + logFieldSep + "%b" + logRecordSep

func ParseLog(output string) []LogEntry {
	var entries []LogEntry

	records := strings.Split(output, logRecordSep)
	for _, record := range records {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}

		fields := strings.SplitN(record, logFieldSep, 6)
		if len(fields) < 5 {
			continue
		}

		entry := LogEntry{
			Hash:        strings.TrimSpace(fields[0]),
			AuthorName:  strings.TrimSpace(fields[1]),
			AuthorEmail: strings.TrimSpace(fields[2]),
			AuthorDate:  strings.TrimSpace(fields[3]),
			Subject:     strings.TrimSpace(fields[4]),
		}

		if len(fields) > 5 {
			entry.Body = strings.TrimSpace(fields[5])
		}

		entries = append(entries, entry)
	}

	if entries == nil {
		entries = []LogEntry{}
	}

	return entries
}

const ShowFormat = "%H" + logFieldSep + "%an" + logFieldSep + "%ae" + logFieldSep + "%aI" + logFieldSep + "%s" + logFieldSep + "%b" + logRecordSep

func ParseShow(metadataOutput, numstatOutput, patchOutput string) ShowResult {
	records := strings.SplitN(metadataOutput, logRecordSep, 2)
	record := strings.TrimSpace(records[0])

	fields := strings.SplitN(record, logFieldSep, 6)

	var result ShowResult

	if len(fields) >= 5 {
		result.Hash = strings.TrimSpace(fields[0])
		result.AuthorName = strings.TrimSpace(fields[1])
		result.AuthorEmail = strings.TrimSpace(fields[2])
		result.AuthorDate = strings.TrimSpace(fields[3])
		result.Subject = strings.TrimSpace(fields[4])

		if len(fields) > 5 {
			result.Body = strings.TrimSpace(fields[5])
		}
	}

	result.Stats = ParseDiffNumstat(numstatOutput)
	result.Patch = patchOutput

	return result
}

func ParseBlame(output string) []BlameLine {
	var lines []BlameLine

	rawLines := strings.Split(output, "\n")
	i := 0

	commitCache := make(map[string]*BlameLine)

	for i < len(rawLines) {
		line := rawLines[i]
		if line == "" {
			i++
			continue
		}

		headerParts := strings.Fields(line)
		if len(headerParts) < 3 {
			i++
			continue
		}

		hash := headerParts[0]
		origLine, _ := strconv.Atoi(headerParts[1])
		finalLine, _ := strconv.Atoi(headerParts[2])

		entry := BlameLine{
			Hash:      hash,
			OrigLine:  origLine,
			FinalLine: finalLine,
		}

		i++

		if cached, ok := commitCache[hash]; ok {
			entry.AuthorName = cached.AuthorName
			entry.AuthorEmail = cached.AuthorEmail
			entry.AuthorDate = cached.AuthorDate
			entry.Summary = cached.Summary
		}

		for i < len(rawLines) {
			kvLine := rawLines[i]

			if strings.HasPrefix(kvLine, "\t") {
				entry.Content = kvLine[1:]
				i++
				break
			}

			parts := strings.SplitN(kvLine, " ", 2)
			key := parts[0]
			value := ""
			if len(parts) > 1 {
				value = parts[1]
			}

			switch key {
			case "author":
				entry.AuthorName = value
			case "author-mail":
				entry.AuthorEmail = strings.Trim(value, "<>")
			case "author-time":
				entry.AuthorDate = value
			case "summary":
				entry.Summary = value
			}

			i++
		}

		if _, ok := commitCache[hash]; !ok {
			cached := entry
			commitCache[hash] = &cached
		}

		lines = append(lines, entry)
	}

	if lines == nil {
		lines = []BlameLine{}
	}

	return lines
}
