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
	planRegexp      = regexp.MustCompile(`^1\.\.([\d,.\x{00a0}\x{202f} ]+)(\s+#\s+(.*))?$`)
	testPointRegexp = regexp.MustCompile(`^(not )?ok\b`)
	pragmaRegexp    = regexp.MustCompile(`^pragma\s+[+-]\w`)
	// csiRegexp matches all CSI escape sequences (ESC [ ... <final byte>),
	// not just SGR, per the ANSI Display Hints amendment security guidance.
	csiRegexp = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]")
)

// stripANSI removes all CSI escape sequences from a string.
func stripANSI(s string) string {
	return csiRegexp.ReplaceAllString(s, "")
}

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
