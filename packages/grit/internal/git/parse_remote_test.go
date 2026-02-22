package git

import (
	"testing"
)

func TestParseRemoteList(t *testing.T) {
	input := "origin\tgit@github.com:user/repo.git (fetch)\norigin\tgit@github.com:user/repo.git (push)\nupstream\thttps://github.com/org/repo.git (fetch)\nupstream\thttps://github.com/org/repo-push.git (push)\n"

	remotes := ParseRemoteList(input)

	if len(remotes) != 2 {
		t.Fatalf("remotes count = %d, want 2", len(remotes))
	}

	if remotes[0].Name != "origin" {
		t.Errorf("remote 0 name = %q, want %q", remotes[0].Name, "origin")
	}

	if remotes[0].FetchURL != "git@github.com:user/repo.git" {
		t.Errorf("remote 0 fetch = %q, want %q", remotes[0].FetchURL, "git@github.com:user/repo.git")
	}

	if remotes[0].PushURL != "git@github.com:user/repo.git" {
		t.Errorf("remote 0 push = %q, want %q", remotes[0].PushURL, "git@github.com:user/repo.git")
	}

	if remotes[1].Name != "upstream" {
		t.Errorf("remote 1 name = %q, want %q", remotes[1].Name, "upstream")
	}

	if remotes[1].PushURL != "https://github.com/org/repo-push.git" {
		t.Errorf("remote 1 push = %q, want %q", remotes[1].PushURL, "https://github.com/org/repo-push.git")
	}
}

func TestParseRemoteListEmpty(t *testing.T) {
	remotes := ParseRemoteList("")

	if len(remotes) != 0 {
		t.Errorf("remotes count = %d, want 0", len(remotes))
	}
}
