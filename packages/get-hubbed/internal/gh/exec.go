package gh

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"unicode/utf8"
)

func Run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gh %v: %w: %s", args, err, truncateStderr(stderr.String()))
	}

	return stdout.String(), nil
}

const maxStderrBytes = 100_000

func truncateStderr(s string) string {
	if len(s) <= maxStderrBytes {
		return s
	}

	s = s[:maxStderrBytes]

	for !utf8.ValidString(s) && len(s) > 0 {
		s = s[:len(s)-1]
	}

	return s
}
