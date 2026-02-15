package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amarbel-llc/purse-first/internal/decision"
	"github.com/amarbel-llc/purse-first/internal/mapping"
)

func setupMappings(t *testing.T, dir string) {
	t.Helper()

	mf := mapping.MappingFile{
		Server: "lux",
		Mappings: []mapping.Mapping{
			{
				Replaces:   "Read",
				Extensions: []string{".go", ".py"},
				Tools: []mapping.ToolSuggestion{
					{Name: "lsp_document_symbols", UseWhen: "understanding file structure"},
					{Name: "lsp_hover", UseWhen: "getting type info"},
				},
				Reason: "Use lux MCP tools instead",
			},
			{
				Replaces:   "Grep",
				Extensions: []string{".go"},
				Tools: []mapping.ToolSuggestion{
					{Name: "lsp_workspace_symbols", UseWhen: "finding symbols by name"},
				},
				Reason: "Use lux MCP tools for semantic search",
			},
		},
	}

	mappingDir := filepath.Join(dir, ".purse-first")
	if err := os.MkdirAll(mappingDir, 0o755); err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(mf)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(mappingDir, "lux.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHandlerDeniesMatchingTool(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	setupMappings(t, projectDir)

	input := decision.HookInput{
		SessionID:     "test",
		ToolName:      "Read",
		ToolInput:     map[string]any{"file_path": "/path/to/foo.go"},
		HookEventName: "PreToolUse",
	}

	inputJSON, _ := json.Marshal(input)

	var stdout bytes.Buffer
	err := HandlePreToolUse(bytes.NewReader(inputJSON), &stdout, projectDir)
	if err != nil {
		t.Fatal(err)
	}

	var output decision.HookOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse output: %v\nraw: %s", err, stdout.String())
	}

	if output.HookSpecificOutput.PermissionDecision != "deny" {
		t.Errorf("expected deny, got %s", output.HookSpecificOutput.PermissionDecision)
	}

	reason := output.HookSpecificOutput.PermissionDecisionReason
	if !strings.Contains(reason, "mcp__lux__lsp_document_symbols") {
		t.Errorf("expected mcp__lux__ prefix in reason, got: %s", reason)
	}

	if !strings.Contains(reason, "mcp__lux__lsp_hover") {
		t.Errorf("expected lsp_hover suggestion in reason, got: %s", reason)
	}
}

func TestHandlerPassthroughNoMappings(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	input := decision.HookInput{
		SessionID:     "test",
		ToolName:      "Read",
		ToolInput:     map[string]any{"file_path": "/path/to/foo.go"},
		HookEventName: "PreToolUse",
	}

	inputJSON, _ := json.Marshal(input)

	var stdout bytes.Buffer
	err := HandlePreToolUse(bytes.NewReader(inputJSON), &stdout, projectDir)
	if err != nil {
		t.Fatal(err)
	}

	if stdout.Len() != 0 {
		t.Errorf("expected empty output for passthrough, got: %s", stdout.String())
	}
}

func TestHandlerPassthroughNonMatchingExtension(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	setupMappings(t, projectDir)

	input := decision.HookInput{
		SessionID:     "test",
		ToolName:      "Read",
		ToolInput:     map[string]any{"file_path": "/path/to/readme.md"},
		HookEventName: "PreToolUse",
	}

	inputJSON, _ := json.Marshal(input)

	var stdout bytes.Buffer
	err := HandlePreToolUse(bytes.NewReader(inputJSON), &stdout, projectDir)
	if err != nil {
		t.Fatal(err)
	}

	if stdout.Len() != 0 {
		t.Errorf("expected passthrough for .md file, got: %s", stdout.String())
	}
}

func TestHandlerPassthroughNoFilePath(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	setupMappings(t, projectDir)

	input := decision.HookInput{
		SessionID:     "test",
		ToolName:      "Read",
		ToolInput:     map[string]any{},
		HookEventName: "PreToolUse",
	}

	inputJSON, _ := json.Marshal(input)

	var stdout bytes.Buffer
	err := HandlePreToolUse(bytes.NewReader(inputJSON), &stdout, projectDir)
	if err != nil {
		t.Fatal(err)
	}

	if stdout.Len() != 0 {
		t.Errorf("expected passthrough with no file_path, got: %s", stdout.String())
	}
}

func TestHandlerFailOpenOnBadJSON(t *testing.T) {
	var stdout bytes.Buffer
	err := HandlePreToolUse(strings.NewReader("not json"), &stdout, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if stdout.Len() != 0 {
		t.Errorf("expected passthrough on bad JSON, got: %s", stdout.String())
	}
}

func TestHandlerGrepWithPath(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	setupMappings(t, projectDir)

	input := decision.HookInput{
		SessionID: "test",
		ToolName:  "Grep",
		ToolInput: map[string]any{
			"pattern": "func main",
			"path":    "/path/to/foo.go",
		},
		HookEventName: "PreToolUse",
	}

	inputJSON, _ := json.Marshal(input)

	var stdout bytes.Buffer
	err := HandlePreToolUse(bytes.NewReader(inputJSON), &stdout, projectDir)
	if err != nil {
		t.Fatal(err)
	}

	var output decision.HookOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}

	if output.HookSpecificOutput.PermissionDecision != "deny" {
		t.Errorf("expected deny for Grep on .go path, got %s", output.HookSpecificOutput.PermissionDecision)
	}
}
