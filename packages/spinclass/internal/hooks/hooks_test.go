package hooks

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func makeInput(toolName string, toolInput map[string]any, cwd string) []byte {
	input := map[string]any{
		"tool_name":  toolName,
		"tool_input": toolInput,
		"cwd":        cwd,
	}
	data, _ := json.Marshal(input)
	return data
}

func TestReadInsideBoundaryAllowed(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "src", "main.go")

	input := makeInput("Read", map[string]any{"file_path": target}, dir)

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() != 0 {
		t.Errorf("expected no output for allowed path, got %q", out.String())
	}
}

func TestReadOutsideBoundaryDenied(t *testing.T) {
	boundary := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.go")

	input := makeInput("Read", map[string]any{"file_path": target}, boundary)

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, boundary, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() == 0 {
		t.Fatal("expected deny output for path outside boundary")
	}

	var result map[string]any
	json.Unmarshal(out.Bytes(), &result)
	hookOutput := result["hookSpecificOutput"].(map[string]any)

	if hookOutput["permissionDecision"] != "deny" {
		t.Errorf("expected deny, got %v", hookOutput["permissionDecision"])
	}
}

func TestWriteOutsideBoundaryDenied(t *testing.T) {
	boundary := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "file.go")

	input := makeInput("Write", map[string]any{"file_path": target}, boundary)

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, boundary, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() == 0 {
		t.Fatal("expected deny output")
	}
}

func TestEditOutsideBoundaryDenied(t *testing.T) {
	boundary := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "file.go")

	input := makeInput("Edit", map[string]any{"file_path": target}, boundary)

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, boundary, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() == 0 {
		t.Fatal("expected deny output")
	}
}

func TestGlobOutsideBoundaryDenied(t *testing.T) {
	boundary := t.TempDir()
	outside := t.TempDir()

	input := makeInput("Glob", map[string]any{
		"pattern": "*.go",
		"path":    outside,
	}, boundary)

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, boundary, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() == 0 {
		t.Fatal("expected deny output")
	}
}

func TestGlobNoPathAllowed(t *testing.T) {
	boundary := t.TempDir()

	input := makeInput("Glob", map[string]any{
		"pattern": "*.go",
	}, boundary)

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, boundary, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() != 0 {
		t.Errorf("expected no output when path is absent, got %q", out.String())
	}
}

func TestGrepOutsideBoundaryDenied(t *testing.T) {
	boundary := t.TempDir()
	outside := t.TempDir()

	input := makeInput("Grep", map[string]any{
		"pattern": "TODO",
		"path":    outside,
	}, boundary)

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, boundary, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() == 0 {
		t.Fatal("expected deny output")
	}
}

func TestBashAbsolutePathOutsideDenied(t *testing.T) {
	boundary := t.TempDir()
	outside := t.TempDir()

	cmd := "cat " + filepath.Join(outside, "secret.txt")
	input := makeInput("Bash", map[string]any{"command": cmd}, boundary)

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, boundary, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() == 0 {
		t.Fatal("expected deny output for bash with outside absolute path")
	}
}

func TestBashAbsolutePathInsideAllowed(t *testing.T) {
	boundary := t.TempDir()

	cmd := "cat " + filepath.Join(boundary, "file.txt")
	input := makeInput("Bash", map[string]any{"command": cmd}, boundary)

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, boundary, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() != 0 {
		t.Errorf("expected no output for bash with inside path, got %q", out.String())
	}
}

func TestBashRelativePathAllowed(t *testing.T) {
	boundary := t.TempDir()

	input := makeInput("Bash", map[string]any{"command": "cat ../../etc/passwd"}, boundary)

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, boundary, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() != 0 {
		t.Errorf("expected no output for relative path, got %q", out.String())
	}
}

func TestBashNoPathAllowed(t *testing.T) {
	boundary := t.TempDir()

	input := makeInput("Bash", map[string]any{"command": "go test ./..."}, boundary)

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, boundary, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() != 0 {
		t.Errorf("expected no output for command without abs paths, got %q", out.String())
	}
}

func TestTaskAllowedBecauseSubagentsInheritHooks(t *testing.T) {
	boundary := t.TempDir()

	input := makeInput("Task", map[string]any{
		"prompt":        "explore the codebase",
		"subagent_type": "Explore",
	}, boundary)

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, boundary, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() != 0 {
		t.Errorf("expected no output for Task (subagents inherit hooks), got %q", out.String())
	}
}

func TestSymlinkOutsideBoundaryDenied(t *testing.T) {
	boundary := t.TempDir()
	outside := t.TempDir()

	outsideFile := filepath.Join(outside, "real.go")
	os.WriteFile(outsideFile, []byte("package main"), 0o644)

	link := filepath.Join(boundary, "link.go")
	os.Symlink(outsideFile, link)

	input := makeInput("Read", map[string]any{"file_path": link}, boundary)

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, boundary, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() == 0 {
		t.Fatal("expected deny output for symlink pointing outside")
	}
}

func TestAllowedPathBypassesBoundary(t *testing.T) {
	boundary := t.TempDir()
	allowedDir := t.TempDir()
	target := filepath.Join(allowedDir, "settings.json")

	input := makeInput("Read", map[string]any{"file_path": target}, boundary)

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, boundary, []string{allowedDir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() != 0 {
		t.Errorf("expected no output for allowed path outside boundary, got %q", out.String())
	}
}

func TestAllowedPathExactMatch(t *testing.T) {
	boundary := t.TempDir()
	allowedDir := t.TempDir()

	input := makeInput("Read", map[string]any{"file_path": allowedDir}, boundary)

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, boundary, []string{allowedDir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() != 0 {
		t.Errorf("expected no output for exact allowed path, got %q", out.String())
	}
}

func TestNonAllowedPathStillDenied(t *testing.T) {
	boundary := t.TempDir()
	allowedDir := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.go")

	input := makeInput("Read", map[string]any{"file_path": target}, boundary)

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, boundary, []string{allowedDir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() == 0 {
		t.Fatal("expected deny for path outside both boundary and allowed list")
	}
}

func TestUnmatchedToolPassesThrough(t *testing.T) {
	boundary := t.TempDir()

	input := makeInput("WebSearch", map[string]any{"query": "test"}, boundary)

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, boundary, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() != 0 {
		t.Errorf("expected passthrough for unmatched tool, got %q", out.String())
	}
}
