package command

import (
	"testing"
)

func TestExtractSimpleCommands(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple command",
			input: "git status --short",
			want:  []string{"git status --short"},
		},
		{
			name:  "two commands joined by &&",
			input: "cd /home/user/repo && git log --oneline master..HEAD",
			want:  []string{"cd /home/user/repo", "git log --oneline master..HEAD"},
		},
		{
			name:  "&&, ||, and redirect",
			input: `cd /foo && git log --oneline master..69dde07 2>/dev/null || echo "(no commits ahead)"`,
			want:  []string{"cd /foo", "git log --oneline master..69dde07", `echo "(no commits ahead)"`},
		},
		{
			name:  "semicolon-separated commands",
			input: "git status; git diff",
			want:  []string{"git status", "git diff"},
		},
		{
			name:  "pipe",
			input: "git log --oneline | head -5",
			want:  []string{"git log --oneline", "head -5"},
		},
		{
			name:  "subshell with &&",
			input: "(cd /foo && git log --oneline)",
			want:  []string{"cd /foo", "git log --oneline"},
		},
		{
			name:  "quoted argument preserved",
			input: `echo "git status"`,
			want:  []string{`echo "git status"`},
		},
		{
			name:  "empty string",
			input: "",
			want:  []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSimpleCommands(tt.input)

			if len(got) != len(tt.want) {
				t.Fatalf("extractSimpleCommands(%q) returned %d commands %v, want %d commands %v",
					tt.input, len(got), got, len(tt.want), tt.want)
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("extractSimpleCommands(%q)[%d] = %q, want %q",
						tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}
