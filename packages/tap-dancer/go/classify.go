package tap

import (
	"regexp"
	"strings"
)

type lineKind int

const (
	lineUnknown lineKind = iota
	lineVersion
	linePlan
	lineTestPoint
	lineYAMLStart
	lineYAMLEnd
	lineBailOut
	linePragma
	lineComment
	lineSubtestComment
	lineEmpty
)

var (
	planRegexp      = regexp.MustCompile(`^1\.\.(\d+)(\s+#\s+(.*))?$`)
	testPointRegexp = regexp.MustCompile(`^(not )?ok\b`)
	pragmaRegexp    = regexp.MustCompile(`^pragma\s+[+-]\w`)
)

func classifyLine(line string) lineKind {
	if line == "TAP version 14" {
		return lineVersion
	}

	if planRegexp.MatchString(line) {
		return linePlan
	}

	if testPointRegexp.MatchString(line) {
		return lineTestPoint
	}

	if line == "---" {
		return lineYAMLStart
	}

	if line == "..." {
		return lineYAMLEnd
	}

	if strings.HasPrefix(line, "Bail out!") {
		return lineBailOut
	}

	if pragmaRegexp.MatchString(line) {
		return linePragma
	}

	if strings.HasPrefix(line, "# Subtest") {
		return lineSubtestComment
	}

	if strings.HasPrefix(line, "#") {
		return lineComment
	}

	if strings.TrimSpace(line) == "" {
		return lineEmpty
	}

	return lineUnknown
}
