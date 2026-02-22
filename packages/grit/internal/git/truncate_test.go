package git

import "testing"

func TestTruncatePatch(t *testing.T) {
	tests := []struct {
		name          string
		patch         string
		maxLines      int
		wantTruncated bool
		wantLine      int
		wantLines     int
	}{
		{
			name:          "no truncation when maxLines is 0",
			patch:         "line1\nline2\nline3",
			maxLines:      0,
			wantTruncated: false,
			wantLine:      0,
			wantLines:     3,
		},
		{
			name:          "no truncation when under limit",
			patch:         "line1\nline2",
			maxLines:      5,
			wantTruncated: false,
			wantLine:      0,
			wantLines:     2,
		},
		{
			name:          "no truncation when exactly at limit",
			patch:         "line1\nline2\nline3",
			maxLines:      3,
			wantTruncated: false,
			wantLine:      0,
			wantLines:     3,
		},
		{
			name:          "truncates when over limit",
			patch:         "line1\nline2\nline3\nline4\nline5",
			maxLines:      3,
			wantTruncated: true,
			wantLine:      3,
			wantLines:     3,
		},
		{
			name:          "empty patch",
			patch:         "",
			maxLines:      5,
			wantTruncated: false,
			wantLine:      0,
			wantLines:     0,
		},
		{
			name:          "single line no truncation",
			patch:         "only-line",
			maxLines:      1,
			wantTruncated: false,
			wantLine:      0,
			wantLines:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, truncated, line := TruncatePatch(tt.patch, tt.maxLines)
			if truncated != tt.wantTruncated {
				t.Errorf("truncated = %v, want %v", truncated, tt.wantTruncated)
			}
			if line != tt.wantLine {
				t.Errorf("line = %d, want %d", line, tt.wantLine)
			}

			if tt.wantLines > 0 {
				gotLines := len(splitLines(got))
				if gotLines != tt.wantLines {
					t.Errorf("result has %d lines, want %d", gotLines, tt.wantLines)
				}
			}
		})
	}
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := []string{}
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start <= len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
