package tap

import (
	"fmt"
	"strconv"
	"strings"
)

func parsePlan(line string) (PlanResult, error) {
	m := planRegexp.FindStringSubmatch(line)
	if m == nil {
		return PlanResult{}, fmt.Errorf("invalid plan line: %q", line)
	}

	count, err := strconv.Atoi(m[1])
	if err != nil {
		return PlanResult{}, fmt.Errorf("invalid plan count: %v", err)
	}

	return PlanResult{
		Count:  count,
		Reason: strings.TrimSpace(m[3]),
	}, nil
}

func parseTestPoint(line string) (TestPointResult, []Diagnostic) {
	var tp TestPointResult
	var diags []Diagnostic

	rest := line
	if strings.HasPrefix(rest, "not ok") {
		tp.OK = false
		rest = rest[6:]
	} else if strings.HasPrefix(rest, "ok") {
		tp.OK = true
		rest = rest[2:]
	}

	rest = strings.TrimLeft(rest, " ")

	// Parse optional test number
	if len(rest) > 0 && rest[0] >= '0' && rest[0] <= '9' {
		numEnd := 0
		for numEnd < len(rest) && rest[numEnd] >= '0' && rest[numEnd] <= '9' {
			numEnd++
		}
		tp.Number, _ = strconv.Atoi(rest[:numEnd])
		rest = rest[numEnd:]
	}

	// Parse optional description separator " - " or "- "
	if strings.HasPrefix(rest, " - ") {
		rest = rest[3:]
	} else if strings.HasPrefix(rest, "- ") {
		rest = rest[2:]
	} else if strings.HasPrefix(rest, " ") {
		rest = rest[1:]
	}

	// Find unescaped # for directive
	desc, directive, reason := splitDirective(rest)
	tp.Description = unescapeDescription(strings.TrimSpace(desc))
	tp.Directive = directive
	tp.Reason = reason

	return tp, diags
}

func splitDirective(s string) (desc string, directive Directive, reason string) {
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++ // skip escaped char
			continue
		}
		if s[i] == '#' {
			// Check for directive pattern: " # TODO" or " # SKIP"
			if i > 0 && s[i-1] == ' ' {
				after := strings.TrimSpace(s[i+1:])
				upper := strings.ToUpper(after)
				if strings.HasPrefix(upper, "TODO") {
					reason := strings.TrimSpace(after[4:])
					return s[:i-1], DirectiveTodo, reason
				}
				if strings.HasPrefix(upper, "SKIP") {
					reason := strings.TrimSpace(after[4:])
					return s[:i-1], DirectiveSkip, reason
				}
			}
		}
	}
	return s, DirectiveNone, ""
}

func unescapeDescription(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			next := s[i+1]
			if next == '#' || next == '\\' {
				b.WriteByte(next)
				i++
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func parseBailOut(line string) BailOutResult {
	reason := strings.TrimPrefix(line, "Bail out!")
	return BailOutResult{Reason: strings.TrimSpace(reason)}
}

func parsePragma(line string) PragmaResult {
	rest := strings.TrimPrefix(line, "pragma ")
	enabled := rest[0] == '+'
	key := rest[1:]
	return PragmaResult{Key: key, Enabled: enabled}
}
