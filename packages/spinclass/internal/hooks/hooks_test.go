package hooks

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestNotifyWriterReceivesViolation(t *testing.T) {
	boundary := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.go")

	input := makeInput("Read", map[string]any{"file_path": target}, boundary)

	var stdout, notify bytes.Buffer
	err := Run(bytes.NewReader(input), &stdout, boundary, nil, &notify)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.Len() != 0 {
		t.Errorf("expected no stdout output, got %q", stdout.String())
	}

	if notify.Len() == 0 {
		t.Fatal("expected violation written to notify writer")
	}

	if !strings.Contains(notify.String(), "worktree boundary violation") {
		t.Errorf("expected violation message, got %q", notify.String())
	}
}

func TestNilNotifySkipsBoundaryCheck(t *testing.T) {
	boundary := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.go")

	input := makeInput("Read", map[string]any{"file_path": target}, boundary)

	var stdout bytes.Buffer
	err := Run(bytes.NewReader(input), &stdout, boundary, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.Len() != 0 {
		t.Errorf("expected no output anywhere, got stdout %q", stdout.String())
	}
}

func TestStopHookEventRouteApproves(t *testing.T) {
	input, _ := json.Marshal(map[string]any{
		"hook_event_name": "Stop",
		"session_id":      "test-session-123",
		"cwd":             t.TempDir(),
	})

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, "", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No stop_hook configured -> approve (no output)
	if out.Len() != 0 {
		t.Errorf("expected no output for Stop with no stop_hook, got %q", out.String())
	}
}

func TestStopHookBlocksOnFailure(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TMPDIR", tmpDir)

	// Create a sweatfile with a failing stop_hook
	cwd := t.TempDir()
	os.WriteFile(filepath.Join(cwd, "sweatfile"), []byte(`stop_hook = "false"`), 0o644)

	input, _ := json.Marshal(map[string]any{
		"hook_event_name": "Stop",
		"session_id":      "block-test-session",
		"cwd":             cwd,
	})

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, "", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() == 0 {
		t.Fatal("expected block output for failing stop_hook")
	}

	var result map[string]any
	json.Unmarshal(out.Bytes(), &result)
	if result["decision"] != "block" {
		t.Errorf("expected block decision, got %v", result["decision"])
	}

	// Sentinel file should exist
	sentinel := filepath.Join(tmpDir, "stop-hook-block-test-session")
	if _, err := os.Stat(sentinel); os.IsNotExist(err) {
		t.Error("expected sentinel file to be created")
	}
}

func TestStopHookApprovesOnSecondInvocation(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TMPDIR", tmpDir)

	cwd := t.TempDir()
	os.WriteFile(filepath.Join(cwd, "sweatfile"), []byte(`stop_hook = "false"`), 0o644)

	// Create sentinel file (simulating first invocation already happened)
	sentinel := filepath.Join(tmpDir, "stop-hook-approve-test-session")
	os.WriteFile(sentinel, []byte("previous failure output"), 0o644)

	input, _ := json.Marshal(map[string]any{
		"hook_event_name": "Stop",
		"session_id":      "approve-test-session",
		"cwd":             cwd,
	})

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, "", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Sentinel exists -> approve (no output)
	if out.Len() != 0 {
		t.Errorf("expected no output on second invocation, got %q", out.String())
	}
}

func TestStopHookApprovesOnSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TMPDIR", tmpDir)

	cwd := t.TempDir()
	os.WriteFile(filepath.Join(cwd, "sweatfile"), []byte(`stop_hook = "true"`), 0o644)

	input, _ := json.Marshal(map[string]any{
		"hook_event_name": "Stop",
		"session_id":      "success-test-session",
		"cwd":             cwd,
	})

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, "", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() != 0 {
		t.Errorf("expected no output for passing stop_hook, got %q", out.String())
	}

	// No sentinel should exist on success
	sentinel := filepath.Join(tmpDir, "stop-hook-success-test-session")
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Error("expected no sentinel file for successful stop_hook")
	}
}
