package mesa

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Severity is the fixed styling vocabulary carried on the wire. A consumer
// maps its domain states onto it (active -> OK, stale -> Error, remote rows
// -> Special); the renderer owns the color for each, so producers stay
// colorless. See RFC 0003 §5.
type Severity uint8

const (
	Neutral Severity = iota // default foreground
	Muted                   // dim / secondary
	OK                      // success / healthy (green family)
	Accent                  // informational / secondary-active (cyan family)
	Warn                    // degraded (yellow family)
	Error                   // failed / unhealthy (red family)
	Special                 // distinguished (magenta family) — e.g. remote rows
)

var severityNames = map[Severity]string{
	Neutral: "neutral",
	Muted:   "muted",
	OK:      "ok",
	Accent:  "accent",
	Warn:    "warn",
	Error:   "error",
	Special: "special",
}

var severityByName = func() map[string]Severity {
	m := make(map[string]Severity, len(severityNames))
	for sev, name := range severityNames {
		m[name] = sev
	}
	return m
}()

// String returns the wire name of the severity. An out-of-range value
// renders as "neutral".
func (s Severity) String() string {
	if name, ok := severityNames[s]; ok {
		return name
	}
	return "neutral"
}

// ParseSeverity maps a wire name to a Severity. ok is false for an unknown
// name; per RFC 0003 §5 callers render an unknown severity as Neutral.
func ParseSeverity(name string) (Severity, bool) {
	s, ok := severityByName[name]
	return s, ok
}

var mutedColor = lipgloss.AdaptiveColor{Light: "240", Dark: "245"}

// defaultColor returns the built-in color for a severity, or nil for
// Neutral (which leaves the terminal's default foreground).
func (s Severity) defaultColor() lipgloss.TerminalColor {
	switch s {
	case Muted:
		return mutedColor
	case OK:
		return lipgloss.Color("2")
	case Accent:
		return lipgloss.Color("6")
	case Warn:
		return lipgloss.Color("3")
	case Error:
		return lipgloss.Color("1")
	case Special:
		return lipgloss.Color("5")
	default:
		return nil
	}
}

// parseColor accepts a "#RRGGBB" hex string or a base-10 ANSI 256 index
// ("0".."255"), matching the palette-override grammar of RFC 0003 §5.
func parseColor(s string) (lipgloss.Color, bool) {
	if strings.HasPrefix(s, "#") {
		if len(s) == 7 {
			return lipgloss.Color(s), true
		}
		return "", false
	}
	if n, err := strconv.Atoi(s); err == nil && n >= 0 && n <= 255 {
		return lipgloss.Color(s), true
	}
	return "", false
}
