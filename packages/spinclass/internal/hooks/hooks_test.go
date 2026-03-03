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

func TestViolationWritesJSONApproval(t *testing.T) {
	boundary := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.go")

	input := makeInput("Read", map[string]any{"file_path": target}, boundary)

	var stdout bytes.Buffer
	err := Run(bytes.NewReader(input), &stdout, boundary, nil, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.Len() == 0 {
		t.Fatal("expected JSON output for boundary violation")
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON, got %q: %v", stdout.String(), err)
	}

	hso, ok := result["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("expected hookSpecificOutput in output")
	}

	if hso["hookEventName"] != "PreToolUse" {
		t.Errorf("expected hookEventName PreToolUse, got %v", hso["hookEventName"])
	}

	if hso["permissionDecision"] != "allow" {
		t.Errorf("expected permissionDecision allow, got %v", hso["permissionDecision"])
	}

	ctx, ok := hso["additionalContext"].(string)
	if !ok || ctx == "" {
		t.Fatal("expected additionalContext in output")
	}

	if !strings.Contains(ctx, "worktree boundary violation") {
		t.Errorf("expected violation in additionalContext, got %q", ctx)
	}

	if !strings.Contains(ctx, "work exclusively within the worktree") {
		t.Errorf("expected guidance in additionalContext, got %q", ctx)
	}
}

func TestNoViolationProducesNoOutput(t *testing.T) {
	boundary := t.TempDir()
	target := filepath.Join(boundary, "inside.go")

	input := makeInput("Read", map[string]any{"file_path": target}, boundary)

	var stdout bytes.Buffer
	err := Run(bytes.NewReader(input), &stdout, boundary, nil, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.Len() != 0 {
		t.Errorf("expected no output for path inside boundary, got %q", stdout.String())
	}
}

func TestBoundaryNotifyDisabledProducesNoOutput(t *testing.T) {
	boundary := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.go")

	input := makeInput("Read", map[string]any{"file_path": target}, boundary)

	var stdout bytes.Buffer
	err := Run(bytes.NewReader(input), &stdout, boundary, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.Len() != 0 {
		t.Errorf("expected no output when boundary notify disabled, got %q", stdout.String())
	}
}

func TestNoBoundaryProducesNoOutput(t *testing.T) {
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.go")

	input := makeInput("Read", map[string]any{"file_path": target}, outside)

	var stdout bytes.Buffer
	err := Run(bytes.NewReader(input), &stdout, "", nil, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.Len() != 0 {
		t.Errorf("expected no output with empty boundary, got %q", stdout.String())
	}
}

func TestStopHookEventRouteApproves(t *testing.T) {
	input, _ := json.Marshal(map[string]any{
		"hook_event_name": "Stop",
		"session_id":      "test-session-123",
		"cwd":             t.TempDir(),
	})

	var out bytes.Buffer
	err := Run(bytes.NewReader(input), &out, "", nil, false)
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
	err := Run(bytes.NewReader(input), &out, "", nil, false)
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
	err := Run(bytes.NewReader(input), &out, "", nil, false)
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
	err := Run(bytes.NewReader(input), &out, "", nil, false)
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
