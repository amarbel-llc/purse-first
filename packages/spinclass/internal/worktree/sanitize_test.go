package worktree

import "testing"

func TestSanitizeBranchName(t *testing.T) {
	tests := []struct {
		name     string
		parts    []string
		expected string
	}{
		{
			name:     "multiple words joined with hyphens",
			parts:    []string{"this", "is", "a", "branch"},
			expected: "this-is-a-branch",
		},
		{
			name:     "hyphens in parts become underscores",
			parts:    []string{"branch-name"},
			expected: "branch_name",
		},
		{
			name:     "mixed plain and hyphenated parts",
			parts:    []string{"this", "is", "the", "branch-name"},
			expected: "this-is-the-branch_name",
		},
		{
			name:     "uppercase lowercased",
			parts:    []string{"Fix", "AUTH", "Bug"},
			expected: "fix-auth-bug",
		},
		{
			name:     "single part passthrough",
			parts:    []string{"single"},
			expected: "single",
		},
		{
			name:     "tilde stripped",
			parts:    []string{"branch~name"},
			expected: "branchname",
		},
		{
			name:     "caret stripped",
			parts:    []string{"branch^name"},
			expected: "branchname",
		},
		{
			name:     "colon stripped",
			parts:    []string{"branch:name"},
			expected: "branchname",
		},
		{
			name:     "backslash stripped",
			parts:    []string{"branch\\name"},
			expected: "branchname",
		},
		{
			name:     "question mark stripped",
			parts:    []string{"branch?name"},
			expected: "branchname",
		},
		{
			name:     "asterisk stripped",
			parts:    []string{"branch*name"},
			expected: "branchname",
		},
		{
			name:     "open bracket stripped",
			parts:    []string{"branch[name"},
			expected: "branchname",
		},
		{
			name:     "close bracket stripped",
			parts:    []string{"branch]name"},
			expected: "branchname",
		},
		{
			name:     "spaces in parts become hyphens",
			parts:    []string{"hello world"},
			expected: "hello-world",
		},
		{
			name:     "spaces and hyphens in same part",
			parts:    []string{"hello world", "branch-name"},
			expected: "hello-world-branch_name",
		},
		{
			name:     "control characters stripped",
			parts:    []string{"branch\x00name", "test\x1f"},
			expected: "branchname-test",
		},
		{
			name:     "leading dots stripped",
			parts:    []string{".branch"},
			expected: "branch",
		},
		{
			name:     "trailing dots stripped",
			parts:    []string{"branch."},
			expected: "branch",
		},
		{
			name:     "trailing .lock stripped",
			parts:    []string{"branch.lock"},
			expected: "branch",
		},
		{
			name:     "double dots collapsed to single",
			parts:    []string{"branch..name"},
			expected: "branch.name",
		},
		{
			name:     "at-brace sequence stripped",
			parts:    []string{"branch@{name"},
			expected: "branchname",
		},
		{
			name:     "consecutive hyphens collapsed from empty parts",
			parts:    []string{"a", "", "b"},
			expected: "a-b",
		},
		{
			name:     "consecutive hyphens collapsed from stripped chars",
			parts:    []string{"a~ ~b"},
			expected: "a-b",
		},
		{
			name:     "double hyphens in part become double underscores then collapse",
			parts:    []string{"branch--name"},
			expected: "branch_name",
		},
		{
			name:     "consecutive underscores collapsed",
			parts:    []string{"branch__name"},
			expected: "branch_name",
		},
		{
			name:     "leading hyphens stripped",
			parts:    []string{"-branch"},
			expected: "branch",
		},
		{
			name:     "trailing hyphens stripped",
			parts:    []string{"branch-"},
			expected: "branch",
		},
		{
			name:     "leading underscores stripped",
			parts:    []string{"_branch"},
			expected: "branch",
		},
		{
			name:     "trailing underscores stripped",
			parts:    []string{"branch_"},
			expected: "branch",
		},
		{
			name:     "slashes preserved",
			parts:    []string{"feature/login"},
			expected: "feature/login",
		},
		{
			name:     "dots mid-word preserved",
			parts:    []string{"v1.0"},
			expected: "v1.0",
		},
		{
			name:     "underscores preserved",
			parts:    []string{"my_branch"},
			expected: "my_branch",
		},
		{
			name:     "empty parts produce empty string",
			parts:    []string{},
			expected: "",
		},
		{
			name:     "multiple invalid chars at once",
			parts:    []string{"a~b^c:d"},
			expected: "abcd",
		},
		{
			name:     "complex real-world example",
			parts:    []string{"Fix", "user-auth", "login page"},
			expected: "fix-user_auth-login-page",
		},
		{
			name:     "trailing .lock with suffix after join",
			parts:    []string{"my", "branch.lock"},
			expected: "my-branch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeBranchName(tt.parts)
			if got != tt.expected {
				t.Errorf("SanitizeBranchName(%q) = %q, want %q", tt.parts, got, tt.expected)
			}
		})
	}
}
