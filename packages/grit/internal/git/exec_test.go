package git

import (
	"context"
	"strings"
	"testing"
)

func TestRunWithEnvPassesExtraVars(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	if _, err := Run(ctx, dir, "init"); err != nil {
		t.Fatal(err)
	}

	out, err := RunWithEnv(ctx, dir, []string{"GIT_AUTHOR_NAME=TestBot"}, "var", "GIT_AUTHOR_IDENT")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "TestBot") {
		t.Errorf("expected output to contain TestBot, got: %s", out)
	}
}

func TestRunWithEnvPreservesBaseEnv(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	if _, err := Run(ctx, dir, "init"); err != nil {
		t.Fatal(err)
	}

	out, err := RunWithEnv(ctx, dir, nil, "status")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "On branch") {
		t.Errorf("expected status output, got: %s", out)
	}
}
