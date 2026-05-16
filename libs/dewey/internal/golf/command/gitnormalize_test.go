package command

import "testing"

func TestNormalizeGitCommand(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain git command unchanged",
			in:   "git diff --stat",
			want: "git diff --stat",
		},
		{
			name: "strips -C path",
			in:   "git -C /some/path diff --stat",
			want: "git diff --stat",
		},
		{
			name: "strips --no-pager",
			in:   "git --no-pager log --oneline",
			want: "git log --oneline",
		},
		{
			name: "strips -c key=val",
			in:   "git -c color.ui=always status",
			want: "git status",
		},
		{
			name: "strips --git-dir with separate arg",
			in:   "git --git-dir /foo/.git status",
			want: "git status",
		},
		{
			name: "strips --git-dir=value",
			in:   "git --git-dir=/foo/.git status",
			want: "git status",
		},
		{
			name: "strips --work-tree with separate arg",
			in:   "git --work-tree /foo rev-parse HEAD",
			want: "git rev-parse HEAD",
		},
		{
			name: "strips --work-tree=value",
			in:   "git --work-tree=/foo rev-parse HEAD",
			want: "git rev-parse HEAD",
		},
		{
			name: "strips --bare",
			in:   "git --bare log",
			want: "git log",
		},
		{
			name: "strips multiple global options",
			in:   "git -C /repo --no-pager -c core.pager=cat diff --stat",
			want: "git diff --stat",
		},
		{
			name: "non-git command unchanged",
			in:   "docker ps",
			want: "docker ps",
		},
		{
			name: "empty string unchanged",
			in:   "",
			want: "",
		},
		{
			name: "bare git unchanged",
			in:   "git",
			want: "git",
		},
		{
			name: "strips -C=path equals form",
			in:   "git -C=/some/path diff",
			want: "git diff",
		},
		{
			name: "strips -c=key=val equals form",
			in:   "git -c=color.ui=always status",
			want: "git status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeGitCommand(tt.in)
			if got != tt.want {
				t.Errorf("normalizeGitCommand(%q)\n  got:  %q\n  want: %q", tt.in, got, tt.want)
			}
		})
	}
}
