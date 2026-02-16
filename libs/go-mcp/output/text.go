package output

import (
	"strings"
	"unicode/utf8"
)

// TextLimits controls how text output is truncated.
// Head and Tail are mutually exclusive; Head takes priority when both are set.
// Zero values mean unlimited.
type TextLimits struct {
	Head     int `json:"head,omitempty"`
	Tail     int `json:"tail,omitempty"`
	MaxLines int `json:"max_lines,omitempty"`
	MaxBytes int `json:"max_bytes,omitempty"`
}

// TruncationInfo describes what was removed during truncation.
type TruncationInfo struct {
	OriginalBytes int    `json:"original_bytes"`
	OriginalLines int    `json:"original_lines"`
	KeptBytes     int    `json:"kept_bytes"`
	KeptLines     int    `json:"kept_lines"`
	Position      string `json:"position"`
}

// LimitedText is the result of applying TextLimits to a string.
type LimitedText struct {
	Content        string          `json:"content"`
	Truncated      bool            `json:"truncated"`
	TruncationInfo *TruncationInfo `json:"truncation_info,omitempty"`
}

// LimitText applies the given limits to the input string.
// Processing order: Head/Tail, then MaxLines, then MaxBytes.
func LimitText(input string, limits TextLimits) LimitedText {
	if input == "" {
		return LimitedText{Content: input}
	}

	originalBytes := len(input)
	// Split carefully: a trailing newline should not produce a phantom empty line.
	lines := splitLines(input)
	originalLines := len(lines)
	trailingNewline := len(input) > 0 && input[len(input)-1] == '\n'

	position := ""
	result := lines

	// Step 1: Head / Tail
	if limits.Head > 0 && limits.Head < len(result) {
		result = result[:limits.Head]
		position = "head"
	} else if limits.Tail > 0 && limits.Tail < len(result) {
		result = result[len(result)-limits.Tail:]
		position = "tail"
	}

	// Step 2: MaxLines
	if limits.MaxLines > 0 && limits.MaxLines < len(result) {
		if position == "tail" {
			result = result[len(result)-limits.MaxLines:]
		} else {
			result = result[:limits.MaxLines]
			if position == "" {
				position = "head"
			}
		}
	}

	// Rejoin before byte limiting
	content := joinLines(result, trailingNewline && position == "")

	// Step 3: MaxBytes
	if limits.MaxBytes > 0 && len(content) > limits.MaxBytes {
		content = truncateAtBoundary(content, limits.MaxBytes)
		if position == "" {
			position = "head"
		}
		// Recount lines after byte truncation
		result = splitLines(content)
	}

	truncated := len(content) != originalBytes
	if !truncated {
		return LimitedText{Content: content}
	}

	keptLines := len(result)

	return LimitedText{
		Content:   content,
		Truncated: true,
		TruncationInfo: &TruncationInfo{
			OriginalBytes: originalBytes,
			OriginalLines: originalLines,
			KeptBytes:     len(content),
			KeptLines:     keptLines,
			Position:      position,
		},
	}
}

// splitLines splits input into lines without producing phantom empty entries
// from trailing newlines.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}

	s = strings.TrimRight(s, "\n")
	if s == "" {
		// Input was entirely newlines — treat as one empty line.
		return []string{""}
	}

	return strings.Split(s, "\n")
}

func joinLines(lines []string, trailingNewline bool) string {
	s := strings.Join(lines, "\n")
	if trailingNewline {
		s += "\n"
	}

	return s
}

// truncateAtBoundary truncates content to at most maxBytes. It first tries to
// cut at the last newline boundary, then falls back to a UTF-8 rune boundary.
func truncateAtBoundary(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}

	if len(s) <= maxBytes {
		return s
	}

	truncated := s[:maxBytes]

	// Try to cut at a line boundary.
	if idx := strings.LastIndex(truncated, "\n"); idx > 0 {
		return truncated[:idx+1]
	}

	return truncateUTF8(truncated, maxBytes)
}

// truncateUTF8 ensures we don't cut in the middle of a multi-byte rune.
func truncateUTF8(s string, maxBytes int) string {
	if maxBytes < len(s) {
		s = s[:maxBytes]
	}

	// Walk backwards past any incomplete trailing multi-byte sequence.
	// A UTF-8 leading byte tells us how many bytes the rune needs.
	// We only need to check the last few bytes (max 3 continuation bytes).
	for i := 0; i < utf8.UTFMax && len(s) > 0; i++ {
		if utf8.ValidString(s) {
			return s
		}
		s = s[:len(s)-1]
	}

	return s
}
